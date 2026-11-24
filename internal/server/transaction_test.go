package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

func TestManagersNewTransactionSharesIdentityAndAppendsStartRecords(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}

	first, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() first error = %v", err)
	}
	second, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() second error = %v", err)
	}
	if first.ID() != 1 || second.ID() != 2 {
		t.Fatalf("transaction IDs = (%d, %d), want (1, 2)", first.ID(), second.ID())
	}
	iterator, err := managers.Logs.Iterator()
	if err != nil {
		t.Fatalf("Logs.Iterator() error = %v", err)
	}
	for _, want := range []recovery.TransactionID{second.ID(), first.ID()} {
		encoded, ok, err := iterator.Next()
		if err != nil || !ok {
			t.Fatalf("Iterator.Next() = (_, %t, %v), want a START record", ok, err)
		}
		record, err := recovery.DecodeRecord(encoded)
		if err != nil {
			t.Fatalf("DecodeRecord() error = %v", err)
		}
		start, ok := record.(interface {
			Operation() recovery.Operation
			TransactionID() recovery.TransactionID
		})
		if !ok || start.Operation() != recovery.OperationStart || start.TransactionID() != want {
			t.Fatalf("record = %#v, want START for transaction %d", record, want)
		}
	}
	if encoded, ok, err := iterator.Next(); err != nil || ok || encoded != nil {
		t.Errorf("Iterator.Next() after START records = (%v, %t, %v), want (nil, false, nil)", encoded, ok, err)
	}
	if _, err := first.Rollback(); err != nil {
		t.Fatalf("first.Rollback() with configured log iterator error = %v", err)
	}
	if _, err := second.Rollback(); err != nil {
		t.Fatalf("second.Rollback() with configured log iterator error = %v", err)
	}
}

func TestManagersNewTransactionRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	managers.Buffers = nil
	if transaction, err := managers.NewTransaction(); err == nil || transaction != nil {
		t.Fatalf("NewTransaction() = (%v, %v), want (nil, dependency error)", transaction, err)
	}
	if length, err := managers.Files.Length("simpledb.log"); err != nil || length != 1 {
		t.Errorf("log blocks after incomplete manager rejection = (%d, %v), want (1, nil)", length, err)
	}
}

func TestManagersNewTransactionReportsMissingInternalState(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	managers.locks = nil
	if transaction, err := managers.NewTransaction(); err == nil || transaction != nil {
		t.Fatalf("NewTransaction() = (%v, %v), want (nil, manager state error)", transaction, err)
	}
}

func TestManagersNewTransactionUsesFileBackedStorage(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	block, err := managers.Files.Append("rows.tbl")
	if err != nil {
		t.Fatalf("Files.Append() error = %v", err)
	}
	if err := transaction.Pin(block); err != nil {
		t.Fatalf("Pin() error = %v", err)
	}
	if err := transaction.SetInt(block, 0, 42, false); err != nil {
		t.Fatalf("SetInt() error = %v", err)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	page, err := managers.Files.Read(block)
	if err != nil {
		t.Fatalf("Files.Read() error = %v", err)
	}
	got, err := page.GetInt(0)
	if err != nil || got != 42 {
		t.Errorf("committed block value = (%d, %v), want (42, nil)", got, err)
	}
}

func TestManagersNewTransactionRejectsNilAndIncompleteManagers(t *testing.T) {
	t.Parallel()

	var nilManagers *Managers
	for name, managers := range map[string]*Managers{
		"nil":        nilManagers,
		"incomplete": {},
	} {
		t.Run(name, func(t *testing.T) {
			transaction, err := managers.NewTransaction()
			if err == nil || !strings.Contains(err.Error(), "manager") {
				t.Fatalf("NewTransaction() error = %v, want manager validation error", err)
			}
			if transaction != nil {
				t.Errorf("NewTransaction() = %#v, want nil on failure", transaction)
			}
		})
	}
}

func TestManagersNewTransactionReturnsNoTransactionWhenStartCannotFit(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 11, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("NewTransaction() error = %v, want START append context", err)
	}
	if transaction != nil {
		t.Errorf("NewTransaction() = %#v, want nil on failure", transaction)
	}
}
