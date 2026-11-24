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

func TestDatabaseExecContextUsesAndHonorsContext(t *testing.T) {
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

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := database.ExecContext(canceled, "create table canceled (id int)"); !errors.Is(err, context.Canceled) {
		t.Errorf("database ExecContext() with canceled context error = %v, want context.Canceled", err)
	}

	result, err := database.ExecContext(context.Background(), "create table course (id int)")
	if err != nil {
		t.Fatalf("database ExecContext(CREATE TABLE) error = %v", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 0 {
		t.Errorf("CREATE TABLE RowsAffected() = (%d, %v), want (0, nil)", rows, err)
	}

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("database Begin() error = %v", err)
	}
	if _, err := transaction.ExecContext(canceled, "insert into course (id) values (1)"); !errors.Is(err, context.Canceled) {
		t.Errorf("transaction ExecContext() with canceled context error = %v, want context.Canceled", err)
	}
	result, err = transaction.ExecContext(context.Background(), "insert into course (id) values (1)")
	if err != nil {
		t.Fatalf("transaction ExecContext(INSERT) after cancellation error = %v", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		t.Errorf("INSERT RowsAffected() = (%d, %v), want (1, nil)", rows, err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("transaction Commit() error = %v", err)
	}
}

func TestConnectionExecContextRejectsNilCanceledAndArgumentInputs(t *testing.T) {
	var nilConnection *Connection
	if result, err := nilConnection.ExecContext(nil, "create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("ExecContext() with nil receiver/context = (%v, %v), want context error", result, err)
	}
	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	if result, err := connection.ExecContext(nil, "create table course (id int)", nil); result != nil || err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("ExecContext() with nil context = (%v, %v), want context error", result, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := connection.ExecContext(canceled, "create table course (id int)", nil); result != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("ExecContext() with canceled context = (%v, %v), want context.Canceled", result, err)
	}
	if result, err := connection.ExecContext(context.Background(), "create table course (id int)", []driver.NamedValue{{Name: "id", Value: int64(1)}}); result != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("ExecContext() with named argument = (%v, %v), want argument error", result, err)
	}
}
