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

func TestPreparedStatementQueryContextHonorsCancellationAndTransaction(t *testing.T) {
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
	if _, err := transaction.Exec("create table student (id int, name varchar(20))"); err != nil {
		t.Fatalf("transaction Exec(CREATE TABLE) error = %v", err)
	}
	if _, err := transaction.Exec("insert into student (id, name) values (1, 'Ada')"); err != nil {
		t.Fatalf("transaction Exec(INSERT) error = %v", err)
	}
	statement, err := transaction.PrepareContext(context.Background(), "select id, name from student where id = 1")
	if err != nil {
		t.Fatalf("transaction PrepareContext() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if rows, err := statement.QueryContext(canceled); rows != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("statement QueryContext() canceled = (%v, %v), want context.Canceled", rows, err)
	}
	rows, err := statement.QueryContext(context.Background())
	if err != nil {
		t.Fatalf("statement QueryContext() after cancellation error = %v", err)
	}
	if !rows.Next() {
		t.Fatalf("rows Next() = false, error = %v, want one row", rows.Err())
	}
	var id int64
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("rows Scan() error = %v", err)
	}
	if id != 1 || name != "Ada" {
		t.Errorf("row = (%d, %q), want (1, %q)", id, name, "Ada")
	}
	if err := rows.Close(); err != nil {
		t.Errorf("rows Close() error = %v", err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("transaction Rollback() error = %v", err)
	}
}

func TestStatementQueryContextImplementsDriverContractAndRejectsInvalidInputs(t *testing.T) {
	var nilStatement *statement
	if rows, err := nilStatement.QueryContext(context.Background(), nil); rows != nil || err == nil || !strings.Contains(err.Error(), "statement") {
		t.Errorf("QueryContext() on nil statement = (%v, %v), want statement error", rows, err)
	}
	closed := &statement{closed: true}
	if rows, err := closed.QueryContext(context.Background(), nil); rows != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("QueryContext() on closed statement = (%v, %v), want closed error", rows, err)
	}
	statement := &statement{}
	if rows, err := statement.QueryContext(nil, nil); rows != nil || err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("QueryContext() with nil context = (%v, %v), want context error", rows, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if rows, err := statement.QueryContext(canceled, nil); rows != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("QueryContext() with canceled context = (%v, %v), want context.Canceled", rows, err)
	}
	if rows, err := statement.QueryContext(context.Background(), []driver.NamedValue{{Name: "id", Value: int64(1)}}); rows != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("QueryContext() with an argument = (%v, %v), want argument error", rows, err)
	}
	if _, ok := any(statement).(driver.StmtQueryContext); !ok {
		t.Fatalf("statement type %T does not implement driver.StmtQueryContext", statement)
	}
}
