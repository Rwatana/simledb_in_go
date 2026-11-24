package embedded

import (
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDatabaseSQLExecutesUpdatesInsideExplicitTransaction(t *testing.T) {
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
	if _, err := transaction.Exec("not valid SQL"); err == nil {
		t.Fatal("transaction Exec(invalid SQL) error = nil, want parse failure")
	}
	createResult, err := transaction.Exec("create table course (id int)")
	if err != nil {
		t.Fatalf("transaction Exec(CREATE TABLE) error = %v", err)
	}
	if rows, err := createResult.RowsAffected(); err != nil || rows != 0 {
		t.Errorf("CREATE TABLE RowsAffected() = (%d, %v), want (0, nil)", rows, err)
	}
	insertResult, err := transaction.Exec("insert into course (id) values (1)")
	if err != nil {
		t.Fatalf("transaction Exec(INSERT) error = %v", err)
	}
	if rows, err := insertResult.RowsAffected(); err != nil || rows != 1 {
		t.Errorf("INSERT RowsAffected() = (%d, %v), want (1, nil)", rows, err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("transaction Commit() error = %v", err)
	}

	verification, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("verification NewTransaction() error = %v", err)
	}
	layout, err := managers.Metadata.Layout("course", verification)
	if err != nil {
		t.Fatalf("Metadata.Layout() after commit error = %v", err)
	}
	if layout == nil {
		t.Fatal("Metadata.Layout() after commit = nil, want course layout")
	}
	if fields, want := layout.Schema().Fields(), []string{"id"}; len(fields) != len(want) || fields[0] != want[0] {
		t.Errorf("committed table fields = %v, want %v", fields, want)
	}
	if _, err := verification.Commit(); err != nil {
		t.Fatalf("verification Commit() error = %v", err)
	}

	rolledBack, err := database.Begin()
	if err != nil {
		t.Fatalf("database Begin() after commit error = %v", err)
	}
	if _, err := rolledBack.Exec("create table temporary (id int)"); err != nil {
		t.Fatalf("transaction Exec(rollback target) error = %v", err)
	}
	if err := rolledBack.Rollback(); err != nil {
		t.Fatalf("transaction Rollback() error = %v", err)
	}
	verification, err = managers.NewTransaction()
	if err != nil {
		t.Fatalf("verification NewTransaction() after rollback error = %v", err)
	}
	if _, err := managers.Metadata.Layout("temporary", verification); err == nil {
		t.Fatal("temporary table is visible after rollback, want its catalog entry to be undone")
	}
	if _, err := verification.Commit(); err != nil {
		t.Fatalf("verification Commit() after rollback error = %v", err)
	}

}

func TestConnectionTracksOneActiveTransactionForStatementExecution(t *testing.T) {
	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	first, err := connection.Begin()
	if err != nil {
		t.Fatalf("first Begin() error = %v", err)
	}
	if second, err := connection.Begin(); second != nil || err == nil || !strings.Contains(err.Error(), "active") {
		t.Errorf("nested Begin() = (%v, %v), want active-transaction error", second, err)
	}
	if connection.activeTransaction == nil {
		t.Fatal("active transaction was not associated with the connection")
	}
	if err := first.Rollback(); err != nil {
		t.Fatalf("first Rollback() error = %v", err)
	}
	if connection.activeTransaction != nil {
		t.Fatal("active transaction remains after rollback")
	}
	second, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() after rollback error = %v", err)
	}
	if err := second.Commit(); err != nil {
		t.Fatalf("second Commit() error = %v", err)
	}
}

func TestStatementExecRejectsInvalidStateAndWrapsUpdateErrors(t *testing.T) {
	var nilStatement *statement
	if result, err := nilStatement.Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "statement") {
		t.Errorf("Exec() on nil statement = (%v, %v), want statement error", result, err)
	}
	if result, err := (&statement{closed: true}).Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Exec() on closed statement = (%v, %v), want closed-statement error", result, err)
	}
	if result, err := (&statement{}).Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Exec() without connection = (%v, %v), want connection error", result, err)
	}
	closedConnection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() for closed-state test error = %v", err)
	}
	if err := closedConnection.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if result, err := (&statement{connection: closedConnection}).Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Exec() on closed connection = (%v, %v), want closed-connection error", result, err)
	}
	if result, err := (&statement{connection: &Connection{}}).Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Exec() without managers = (%v, %v), want manager error", result, err)
	}

	managers := newEmbeddedManagers(t)
	connection, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() for active-state tests error = %v", err)
	}
	statementValue := &statement{connection: connection, query: "insert into student (id) values (1)"}
	if result, err := statementValue.Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "active transaction") {
		t.Errorf("Exec() without active transaction = (%v, %v), want active-transaction error", result, err)
	}
	active, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() for active-state tests error = %v", err)
	}
	if result, err := statementValue.Exec([]driver.Value{int64(1)}); result != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("Exec() with arguments = (%v, %v), want argument error", result, err)
	}
	statementValue.query = "not valid SQL"
	if result, err := statementValue.Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "execute embedded statement") {
		t.Errorf("Exec() with invalid SQL = (%v, %v), want contextual update error", result, err)
	}
	if err := active.Rollback(); err != nil {
		t.Fatalf("Rollback() after invalid SQL error = %v", err)
	}

	managerFailure, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() for planner failure error = %v", err)
	}
	active, err = managerFailure.Begin()
	if err != nil {
		t.Fatalf("Begin() for planner failure error = %v", err)
	}
	managers.UpdatePlanner = nil
	statementValue = &statement{connection: managerFailure, query: "create table course (id int)"}
	if result, err := statementValue.Exec(nil); result != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Exec() with missing update planner = (%v, %v), want wrapped planner error", result, err)
	}
	if err := active.Rollback(); err != nil {
		t.Fatalf("Rollback() after planner error = %v", err)
	}
}

func initializeEmbeddedCatalogs(t *testing.T, managers *server.Managers) {
	t.Helper()
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("catalog NewTransaction() error = %v", err)
	}
	if err := managers.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("catalog transaction Commit() error = %v", err)
	}
}
