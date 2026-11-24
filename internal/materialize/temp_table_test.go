package materialize

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type temporaryTableTransactionStub struct {
	calls          atomic.Int64
	sizeFile       string
	size           int
	sizeErr        error
	appendFiles    []string
	appendErr      error
	pins           []storage.BlockID
	pinErr         error
	unpins         []storage.BlockID
	unpinErr       error
	blockSize      int
	blockSizeErr   error
	setIntErr      error
	setStringErr   error
	setIntCalls    int
	setStringCalls int
	intValue       int32
}

func (tx *temporaryTableTransactionStub) Pin(block storage.BlockID) error {
	tx.calls.Add(1)
	tx.pins = append(tx.pins, block)
	return tx.pinErr
}

func (tx *temporaryTableTransactionStub) BlockSize() (int, error) {
	tx.calls.Add(1)
	if tx.blockSizeErr != nil {
		return 0, tx.blockSizeErr
	}
	if tx.blockSize == 0 {
		return 32, nil
	}
	return tx.blockSize, nil
}

func (tx *temporaryTableTransactionStub) GetInt(storage.BlockID, int) (int32, error) {
	tx.calls.Add(1)
	return tx.intValue, nil
}

func (tx *temporaryTableTransactionStub) GetString(storage.BlockID, int) (string, error) {
	tx.calls.Add(1)
	return "", nil
}

func (tx *temporaryTableTransactionStub) SetInt(_ storage.BlockID, _ int32, value int32, _ bool) error {
	tx.calls.Add(1)
	tx.setIntCalls++
	tx.intValue = value
	return tx.setIntErr
}

func (tx *temporaryTableTransactionStub) SetString(storage.BlockID, int32, string, bool) error {
	tx.calls.Add(1)
	tx.setStringCalls++
	return tx.setStringErr
}

func (tx *temporaryTableTransactionStub) Size(fileName string) (int, error) {
	tx.calls.Add(1)
	tx.sizeFile = fileName
	return tx.size, tx.sizeErr
}

func (tx *temporaryTableTransactionStub) Append(fileName string) (storage.BlockID, error) {
	tx.calls.Add(1)
	tx.appendFiles = append(tx.appendFiles, fileName)
	if tx.appendErr != nil {
		return storage.BlockID{}, tx.appendErr
	}
	return storage.NewBlockID(fileName, 0), nil
}

func (tx *temporaryTableTransactionStub) Unpin(block storage.BlockID) error {
	tx.calls.Add(1)
	tx.unpins = append(tx.unpins, block)
	return tx.unpinErr
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

func TestTempTableOpenCreatesAndReturnsCallerOwnedScan(t *testing.T) {
	transaction := &temporaryTableTransactionStub{blockSize: 16}
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	table, err := NewTempTable(transaction, schema)
	if err != nil {
		t.Fatalf("NewTempTable() error = %v", err)
	}
	if transaction.calls.Load() != 0 {
		t.Fatalf("descriptor construction made %d transaction calls, want 0", transaction.calls.Load())
	}

	scan, err := table.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	fileName := table.TableName() + ".tbl"
	block := storage.NewBlockID(fileName, 0)
	if transaction.sizeFile != fileName || len(transaction.appendFiles) != 1 || transaction.appendFiles[0] != fileName {
		t.Errorf("table initialization used size file %q and append files %v, want %q", transaction.sizeFile, transaction.appendFiles, fileName)
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != block {
		t.Errorf("pins = %v, want [%v]", transaction.pins, block)
	}
	if !scan.HasField("id") {
		t.Error("opened scan does not expose its retained schema field id")
	}
	if transaction.setIntCalls == 0 {
		t.Error("opening a new temporary table did not format its first record block")
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("scan.Close() error = %v", err)
	}
	if len(transaction.unpins) != 1 || transaction.unpins[0] != block {
		t.Errorf("unpins after caller close = %v, want [%v]", transaction.unpins, block)
	}
}

func TestTempTableOpenProvidesQueryUpdateScan(t *testing.T) {
	transaction := &temporaryTableTransactionStub{blockSize: 16}
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	table, err := NewTempTable(transaction, schema)
	if err != nil {
		t.Fatalf("NewTempTable() error = %v", err)
	}

	var open func() (query.UpdateScan, error) = table.Open
	scan, err := open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := scan.SetVal("id", query.NewIntConstant(7)); err != nil {
		t.Fatalf("SetVal() error = %v", err)
	}
	value, err := scan.GetVal("id")
	if err != nil {
		t.Fatalf("GetVal() error = %v", err)
	}
	if got, ok := value.AsInt(); !ok || got != 7 {
		t.Errorf("GetVal(\"id\") = (%d, %t), want (7, true)", got, ok)
	}
	if transaction.setIntCalls < 2 {
		t.Errorf("SetInt calls = %d, want first-block formatting and query-value write", transaction.setIntCalls)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("scan.Close() error = %v", err)
	}
}

func TestTempTableOpenExistingTableDoesNotAppend(t *testing.T) {
	transaction := &temporaryTableTransactionStub{size: 1}
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	table, err := NewTempTable(transaction, schema)
	if err != nil {
		t.Fatalf("NewTempTable() error = %v", err)
	}

	scan, err := table.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	fileName := table.TableName() + ".tbl"
	if transaction.sizeFile != fileName || len(transaction.appendFiles) != 0 || transaction.setIntCalls != 0 {
		t.Errorf("existing file initialization: size=%q append=%v format writes=%d", transaction.sizeFile, transaction.appendFiles, transaction.setIntCalls)
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != storage.NewBlockID(fileName, 0) {
		t.Errorf("pins = %v, want first block of %q", transaction.pins, fileName)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("scan.Close() error = %v", err)
	}
}

func TestTempTableOpenRejectsNilOrIncompleteDescriptorWithoutIO(t *testing.T) {
	var nilTable *TempTable
	if scan, err := nilTable.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "descriptor") {
		t.Fatalf("nil descriptor Open() = (%v, %v), want contextual error", scan, err)
	}
	if scan, err := (&TempTable{}).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "descriptor") {
		t.Fatalf("incomplete descriptor Open() = (%v, %v), want contextual error", scan, err)
	}
}

func TestTempTableOpenWrapsInitializationFailuresAndCleansPins(t *testing.T) {
	sizeErr := errors.New("size failed")
	appendErr := errors.New("append failed")
	pinErr := errors.New("pin failed")
	formatErr := errors.New("format failed")
	unpinErr := errors.New("unpin failed")
	tests := []struct {
		name        string
		transaction *temporaryTableTransactionStub
		causes      []error
		wantPins    int
		wantUnpins  int
	}{
		{name: "size", transaction: &temporaryTableTransactionStub{sizeErr: sizeErr}, causes: []error{sizeErr}},
		{name: "append", transaction: &temporaryTableTransactionStub{appendErr: appendErr}, causes: []error{appendErr}},
		{name: "pin", transaction: &temporaryTableTransactionStub{pinErr: pinErr}, causes: []error{pinErr}, wantPins: 1},
		{name: "format and cleanup", transaction: &temporaryTableTransactionStub{blockSize: 16, setIntErr: formatErr, unpinErr: unpinErr}, causes: []error{formatErr, unpinErr}, wantPins: 1, wantUnpins: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &record.Schema{}
			schema.AddIntegerField("id")
			table, err := NewTempTable(tt.transaction, schema)
			if err != nil {
				t.Fatalf("NewTempTable() error = %v", err)
			}
			scan, err := table.Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), table.TableName()) {
				t.Fatalf("Open() = (%v, %v), want contextual error", scan, err)
			}
			for _, cause := range tt.causes {
				if !errors.Is(err, cause) {
					t.Errorf("Open() error %v does not preserve cause %v", err, cause)
				}
			}
			if len(tt.transaction.pins) != tt.wantPins || len(tt.transaction.unpins) != tt.wantUnpins {
				t.Errorf("pins=%v unpins=%v, want counts %d and %d", tt.transaction.pins, tt.transaction.unpins, tt.wantPins, tt.wantUnpins)
			}
		})
	}
}

var _ record.TableScanTransaction = (*temporaryTableTransactionStub)(nil)
