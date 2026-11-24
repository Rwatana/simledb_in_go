package embedded

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestPreparedStatementExecContextUsesCallerTransaction(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("server.NewManagers() error = %v", err)
	}
	initializeEmbeddedCatalogs(t, managers)
	connector, err := NewConnector(managers)
	if err != nil {
		t.Fatalf("NewConnector() error = %v", err)
	}
	database := sql.OpenDB(connector)
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("database Close() error = %v", err)
		}
	})

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("database Begin() error = %v", err)
	}
	statement, err := transaction.PrepareContext(context.Background(), "create table temporary (id int)")
	if err != nil {
		t.Fatalf("transaction PrepareContext() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := statement.ExecContext(canceled); !errors.Is(err, context.Canceled) {
		t.Errorf("statement ExecContext() with canceled context error = %v, want context.Canceled", err)
	}
	result, err := statement.ExecContext(context.Background())
	if err != nil {
		t.Fatalf("statement ExecContext() after cancellation error = %v", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 0 {
		t.Errorf("RowsAffected() = (%d, %v), want (0, nil)", rows, err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("transaction Rollback() error = %v", err)
	}
	verification, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("verification NewTransaction() error = %v", err)
	}
	if _, err := managers.Metadata.Layout("temporary", verification); err == nil {
		t.Error("statement update became visible after its caller rolled back")
	}
	if _, err := verification.Commit(); err != nil {
		t.Fatalf("verification Commit() error = %v", err)
	}
}

func TestStatementExecContextRejectsInvalidInputs(t *testing.T) {
	var nilStatement *statement
	if result, err := nilStatement.ExecContext(nil, nil); result != nil || err == nil || !strings.Contains(err.Error(), "statement") {
		t.Errorf("ExecContext() on nil statement/context = (%v, %v), want statement error", result, err)
	}
	closed := &statement{closed: true}
	if result, err := closed.ExecContext(context.Background(), nil); result != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("ExecContext() on closed statement = (%v, %v), want closed error", result, err)
	}
	missingConnection := &statement{}
	if result, err := missingConnection.ExecContext(context.Background(), nil); result != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("ExecContext() without connection = (%v, %v), want connection error", result, err)
	}
	if result, err := (&statement{}).ExecContext(nil, nil); result != nil || err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("ExecContext() with nil context = (%v, %v), want context error", result, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := (&statement{}).ExecContext(canceled, nil); result != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("ExecContext() with canceled context = (%v, %v), want context.Canceled", result, err)
	}

	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	closedConnection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() for closed state error = %v", err)
	}
	if err := closedConnection.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	statementValue := &statement{connection: closedConnection}
	if result, err := statementValue.ExecContext(context.Background(), nil); result != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("ExecContext() on closed connection = (%v, %v), want closed error", result, err)
	}

	active, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	statementValue = &statement{connection: connection, query: "create table course (id int)"}
	if result, err := statementValue.ExecContext(context.Background(), []driver.NamedValue{{Name: "id", Value: int64(1)}}); result != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("ExecContext() with named argument = (%v, %v), want argument error", result, err)
	}
	if err := active.Rollback(); err != nil {
		t.Fatalf("Rollback() before inactive statement check error = %v", err)
	}
	if result, err := statementValue.ExecContext(context.Background(), nil); result != nil || err == nil || !strings.Contains(err.Error(), "active transaction") {
		t.Errorf("ExecContext() without active transaction = (%v, %v), want active-transaction error", result, err)
	}
}
