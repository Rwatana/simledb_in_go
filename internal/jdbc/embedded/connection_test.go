package embedded

import (
	"database/sql/driver"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

func TestNewConnectionValidatesManagersAndImplementsDriverConnection(t *testing.T) {
	if connection, err := NewConnection(nil); err == nil || connection != nil || !strings.Contains(err.Error(), "manager") {
		t.Fatalf("NewConnection(nil) = (%v, %v), want manager validation error", connection, err)
	}

	managers := newEmbeddedManagers(t)
	connection, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	if connection == nil {
		t.Fatal("NewConnection() = nil, want a connection")
	}
	if _, ok := any(connection).(driver.Conn); !ok {
		t.Fatalf("connection type %T does not implement driver.Conn", connection)
	}
	if statement, err := connection.Prepare("select id from student"); statement != nil || !errors.Is(err, driver.ErrSkip) {
		t.Errorf("Prepare() = (%v, %v), want unsupported statement and driver.ErrSkip", statement, err)
	}
}

func TestConnectionBeginsAndCompletesCallerControlledTransactions(t *testing.T) {
	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}

	first, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() first transaction error = %v", err)
	}
	firstAdapter, ok := first.(*transactionAdapter)
	if !ok || firstAdapter.transaction == nil {
		t.Fatalf("Begin() returned %T, want a transaction adapter with a transaction", first)
	}
	if err := first.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if err := first.Commit(); !errors.Is(err, transaction.ErrTransactionCompleted) {
		t.Errorf("Commit() after commit error = %v, want completed-transaction error", err)
	}
	if err := first.Rollback(); !errors.Is(err, transaction.ErrTransactionCompleted) {
		t.Errorf("Rollback() after commit error = %v, want completed-transaction error", err)
	}

	second, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() after committed transaction error = %v", err)
	}
	secondAdapter, ok := second.(*transactionAdapter)
	if !ok || secondAdapter.transaction == firstAdapter.transaction {
		t.Fatalf("second Begin() transaction = %T, want a fresh transaction adapter", second)
	}
	if err := second.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := second.Rollback(); !errors.Is(err, transaction.ErrTransactionCompleted) {
		t.Errorf("Rollback() after rollback error = %v, want completed-transaction error", err)
	}
}

func TestConnectionCloseIsIdempotentAndLeavesReturnedTransactionCallerOwned(t *testing.T) {
	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	transaction, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Errorf("second Close() error = %v, want nil", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Errorf("Commit() after connection close error = %v, want the transaction to remain active", err)
	}
	if transaction, err := connection.Begin(); transaction != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Begin() after Close() = (%v, %v), want closed-connection error", transaction, err)
	}
	if statement, err := connection.Prepare("select id from student"); statement != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Prepare() after Close() = (%v, %v), want closed-connection error", statement, err)
	}
}

func TestConnectionRejectsInvalidReceiversAndManagerState(t *testing.T) {
	var nilConnection *Connection
	if statement, err := nilConnection.Prepare("select id from student"); statement != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Prepare() on nil connection = (%v, %v), want connection error", statement, err)
	}
	if transaction, err := nilConnection.Begin(); transaction != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Begin() on nil connection = (%v, %v), want connection error", transaction, err)
	}
	if err := nilConnection.Close(); err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Close() on nil connection error = %v, want connection error", err)
	}
	if transaction, err := (&Connection{}).Begin(); transaction != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Begin() without managers = (%v, %v), want manager error", transaction, err)
	}
	incomplete, err := NewConnection(&server.Managers{})
	if err != nil {
		t.Fatalf("NewConnection() with a non-nil manager set error = %v", err)
	}
	if transaction, err := incomplete.Begin(); transaction != nil || err == nil || !strings.Contains(err.Error(), "begin embedded transaction") {
		t.Errorf("Begin() with incomplete manager state = (%v, %v), want contextual manager error", transaction, err)
	}
}

func TestTransactionAdapterRejectsNilAndMissingTransactions(t *testing.T) {
	var nilAdapter *transactionAdapter
	if err := nilAdapter.Commit(); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Commit() on nil adapter error = %v, want transaction error", err)
	}
	if err := nilAdapter.Rollback(); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Rollback() on nil adapter error = %v, want transaction error", err)
	}
	emptyAdapter := &transactionAdapter{}
	if err := emptyAdapter.Commit(); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Commit() without an underlying transaction error = %v, want transaction error", err)
	}
	if err := emptyAdapter.Rollback(); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Rollback() without an underlying transaction error = %v, want transaction error", err)
	}
}

func newEmbeddedManagers(t *testing.T) *server.Managers {
	t.Helper()
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 128, 2)
	if err != nil {
		t.Fatalf("server.NewManagers() error = %v", err)
	}
	return managers
}
