package embedded

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDatabaseExecAutocommitsEmbeddedUpdates(t *testing.T) {
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

	createResult, err := database.Exec("create table course (id int)")
	if err != nil {
		t.Fatalf("database Exec(CREATE TABLE) error = %v", err)
	}
	if rows, err := createResult.RowsAffected(); err != nil || rows != 0 {
		t.Errorf("CREATE TABLE RowsAffected() = (%d, %v), want (0, nil)", rows, err)
	}

	insertResult, err := database.Exec("insert into course (id) values (1)")
	if err != nil {
		t.Fatalf("database Exec(INSERT) error = %v", err)
	}
	if rows, err := insertResult.RowsAffected(); err != nil || rows != 1 {
		t.Errorf("INSERT RowsAffected() = (%d, %v), want (1, nil)", rows, err)
	}

	verification, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("verification NewTransaction() error = %v", err)
	}
	layout, err := managers.Metadata.Layout("course", verification)
	if err != nil {
		t.Fatalf("Metadata.Layout() after autocommit error = %v", err)
	}
	if layout == nil {
		t.Fatal("Metadata.Layout() after autocommit = nil, want course layout")
	}
	if _, err := verification.Commit(); err != nil {
		t.Fatalf("verification Commit() error = %v", err)
	}
}

func TestConnectionExecRejectsInvalidStateAndUpdateFailures(t *testing.T) {
	var nilConnection *Connection
	if result, err := nilConnection.Exec("create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Exec() on nil connection = (%v, %v), want connection error", result, err)
	}

	closed, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() for closed state error = %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if result, err := closed.Exec("create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Exec() on closed connection = (%v, %v), want closed error", result, err)
	}

	if result, err := (&Connection{}).Exec("create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Exec() without managers = (%v, %v), want manager error", result, err)
	}
	incomplete, err := NewConnection(&server.Managers{})
	if err != nil {
		t.Fatalf("NewConnection() for incomplete managers error = %v", err)
	}
	if result, err := incomplete.Exec("create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Errorf("Exec() with incomplete managers = (%v, %v), want incomplete-manager error", result, err)
	}

	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("server.NewManagers() for update tests error = %v", err)
	}
	initializeEmbeddedCatalogs(t, managers)
	connection, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() for input tests error = %v", err)
	}
	if result, err := connection.Exec("create table course (id int)", []driver.Value{int64(1)}); result != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("Exec() with arguments = (%v, %v), want argument error", result, err)
	}
	if result, err := connection.Exec("not valid SQL", nil); result != nil || err == nil || !strings.Contains(err.Error(), "autocommit") {
		t.Errorf("Exec() with invalid SQL = (%v, %v), want contextual autocommit error", result, err)
	}
	if result, err := connection.Exec("create table course (id int)", nil); err != nil || result == nil {
		t.Errorf("Exec() after a parse failure = (%v, %v), want a successful update", result, err)
	}

	managerFailure, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() for planner error test = %v", err)
	}
	managerFailure.managers.UpdatePlanner = nil
	if result, err := managerFailure.Exec("create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "autocommit") {
		t.Errorf("Exec() with missing planner = (%v, %v), want contextual autocommit error", result, err)
	}
}

func TestConnectionExecReportsCommitAndRollbackFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		plannerErr error
		wantError  string
	}{
		{name: "commit", wantError: "commit failed"},
		{name: "rollback", plannerErr: errors.New("planner failed after update"), wantError: "rollback embedded transaction"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "database")
			managers, err := server.NewManagers(directory, 400, 8)
			if err != nil {
				t.Fatalf("server.NewManagers() error = %v", err)
			}
			initializeEmbeddedCatalogs(t, managers)
			managers.UpdatePlanner = &logMutatingUpdatePlanner{
				UpdatePlanner: managers.UpdatePlanner,
				logPath:       filepath.Join(directory, "simpledb.log"),
				returnError:   test.plannerErr,
			}
			connection, err := NewConnection(managers)
			if err != nil {
				t.Fatalf("NewConnection() error = %v", err)
			}
			result, err := connection.Exec("create table course (id int)", nil)
			if result != nil || err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("Exec() = (%v, %v), want %q error", result, err, test.wantError)
			}
		})
	}
}

func TestStatementExecLeavesCallerTransactionOpen(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("server.NewManagers() error = %v", err)
	}
	initializeEmbeddedCatalogs(t, managers)
	connection, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	transaction, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	statement, err := connection.Prepare("create table temporary (id int)")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	result, err := statement.Exec(nil)
	if err != nil {
		t.Fatalf("statement Exec() error = %v", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 0 {
		t.Errorf("RowsAffected() = (%d, %v), want (0, nil)", rows, err)
	}
	if connection.activeTransaction == nil {
		t.Fatal("statement Exec() ended the caller's transaction")
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	verification, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("verification NewTransaction() error = %v", err)
	}
	if _, err := managers.Metadata.Layout("temporary", verification); err == nil {
		t.Error("table created by statement Exec() is visible after the caller rolled back")
	}
	if _, err := verification.Commit(); err != nil {
		t.Fatalf("verification Commit() error = %v", err)
	}
}

type logMutatingUpdatePlanner struct {
	plan.UpdatePlanner
	logPath     string
	returnError error
}

func (planner *logMutatingUpdatePlanner) ExecuteCreateTable(data parse.CreateTableData, transaction record.TableScanTransaction) (int, error) {
	rows, err := planner.UpdatePlanner.ExecuteCreateTable(data, transaction)
	if err != nil {
		return rows, err
	}
	if err := os.Remove(planner.logPath); err != nil {
		return rows, err
	}
	if err := os.Mkdir(planner.logPath, 0o700); err != nil {
		return rows, err
	}
	return rows, planner.returnError
}
