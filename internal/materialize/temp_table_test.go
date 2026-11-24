package materialize

import (
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type temporaryTableTransactionStub struct {
	calls atomic.Int64
}

func (tx *temporaryTableTransactionStub) Pin(storage.BlockID) error {
	tx.calls.Add(1)
	return nil
}

func (tx *temporaryTableTransactionStub) BlockSize() (int, error) {
	tx.calls.Add(1)
	return 32, nil
}

func (tx *temporaryTableTransactionStub) GetInt(storage.BlockID, int) (int32, error) {
	tx.calls.Add(1)
	return 0, nil
}

func (tx *temporaryTableTransactionStub) GetString(storage.BlockID, int) (string, error) {
	tx.calls.Add(1)
	return "", nil
}

func (tx *temporaryTableTransactionStub) SetInt(storage.BlockID, int32, int32, bool) error {
	tx.calls.Add(1)
	return nil
}

func (tx *temporaryTableTransactionStub) SetString(storage.BlockID, int32, string, bool) error {
	tx.calls.Add(1)
	return nil
}

func (tx *temporaryTableTransactionStub) Size(string) (int, error) {
	tx.calls.Add(1)
	return 0, nil
}

func (tx *temporaryTableTransactionStub) Append(string) (storage.BlockID, error) {
	tx.calls.Add(1)
	return storage.BlockID{}, nil
}

func (tx *temporaryTableTransactionStub) Unpin(storage.BlockID) error {
	tx.calls.Add(1)
	return nil
}

func TestNewTempTableRetainsSchemaLayoutWithoutTransactionIO(t *testing.T) {
	t.Parallel()

	transaction := &temporaryTableTransactionStub{}
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 12)

	table, err := NewTempTable(transaction, schema)
	if err != nil {
		t.Fatalf("NewTempTable() error = %v", err)
	}
	if !regexp.MustCompile(`^temp[1-9][0-9]*$`).MatchString(table.TableName()) {
		t.Errorf("TableName() = %q, want a tempN name", table.TableName())
	}
	layout := table.Layout()
	if layout == nil || layout.Schema() != schema {
		t.Fatalf("Layout() = %#v, want a layout retaining schema %p", layout, schema)
	}
	if got, want := layout.SlotSize(), 60; got != want {
		t.Errorf("Layout().SlotSize() = %d, want %d", got, want)
	}
	if table.transaction != transaction {
		t.Error("temporary table did not retain the transaction")
	}
	if got, ok := layout.Offset("name"); !ok || got != 8 {
		t.Errorf("Layout().Offset(\"name\") = (%d, %t), want (8, true)", got, ok)
	}
	if got := transaction.calls.Load(); got != 0 {
		t.Errorf("temporary table construction made %d transaction calls, want 0", got)
	}
}

func TestNewTempTableNamesAreUniqueUnderConcurrentConstruction(t *testing.T) {
	t.Parallel()

	const count = 32
	transaction := &temporaryTableTransactionStub{}
	schema := &record.Schema{}
	schema.AddIntegerField("id")

	names := make(chan string, count)
	errors := make(chan error, count)
	var wait sync.WaitGroup
	for range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			table, err := NewTempTable(transaction, schema)
			if err != nil {
				errors <- err
				return
			}
			names <- table.TableName()
		}()
	}
	wait.Wait()
	close(names)
	close(errors)
	for err := range errors {
		t.Errorf("NewTempTable() error = %v", err)
	}

	seen := make(map[string]struct{}, count)
	for name := range names {
		if _, exists := seen[name]; exists {
			t.Errorf("duplicate temporary table name %q", name)
		}
		seen[name] = struct{}{}
	}
	if got := len(seen); got != count {
		t.Errorf("unique temporary table names = %d, want %d", got, count)
	}
	if got := transaction.calls.Load(); got != 0 {
		t.Errorf("temporary table construction made %d transaction calls, want 0", got)
	}
}

func TestNewTempTableRejectsNilTransactionAndSchema(t *testing.T) {
	t.Parallel()

	schema := &record.Schema{}
	tests := []struct {
		name        string
		transaction record.TableScanTransaction
		schema      *record.Schema
		wantContext string
	}{
		{name: "nil transaction", schema: schema, wantContext: "create temporary table: transaction must not be nil"},
		{name: "nil schema", transaction: &temporaryTableTransactionStub{}, wantContext: "create temporary table: schema must not be nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewTempTable(tt.transaction, tt.schema); err == nil || err.Error() != tt.wantContext {
				t.Errorf("NewTempTable() error = %v, want %q", err, tt.wantContext)
			}
		})
	}
}

var _ record.TableScanTransaction = (*temporaryTableTransactionStub)(nil)
