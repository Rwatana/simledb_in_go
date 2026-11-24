package embedded

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestPreparedStatementDeclaresZeroInputsToDatabaseSQL(t *testing.T) {
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
	if _, err := statement.ExecContext(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "expected 0 arguments") {
		t.Errorf("statement ExecContext() with one argument error = %v, want database/sql zero-input arity error", err)
	}
	if _, err := statement.ExecContext(context.Background()); err != nil {
		t.Fatalf("statement ExecContext() without arguments error = %v", err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("transaction Rollback() error = %v", err)
	}
}
