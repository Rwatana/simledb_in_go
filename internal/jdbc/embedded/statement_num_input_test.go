package embedded

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestPreparedStatementCountsParametersOutsideQuotedStrings(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "no parameters", query: "select id from student", want: 0},
		{name: "one parameter", query: "select id from student where id = ?", want: 1},
		{name: "multiple parameters", query: "insert into student (id, name) values (?, ?)", want: 2},
		{name: "question mark inside literal", query: "select id from student where name = '?' and id = ?", want: 1},
		{name: "question mark inside escaped literal", query: "select id from student where name = 'it''s ?' and id = ?", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (&statement{query: test.query}).NumInput(); got != test.want {
				t.Errorf("NumInput() = %d, want %d", got, test.want)
			}
		})
	}
	if got := (*statement)(nil).NumInput(); got != -1 {
		t.Errorf("NumInput() on nil statement = %d, want -1", got)
	}
	if got := (&statement{query: "select id from student where name = 'unterminated ?"}).NumInput(); got != -1 {
		t.Errorf("NumInput() with unterminated string = %d, want -1", got)
	}
}

func TestPreparedStatementParameterBindingValidatesValuesAndSQL(t *testing.T) {
	query := "select name from student where id = ? and name = ?"
	bound, err := bindParameters(query, []driver.Value{int64(7), "O'Neil"})
	if err != nil {
		t.Fatalf("bindParameters() error = %v", err)
	}
	if want := "select name from student where id = 7 and name = 'O''Neil'"; bound != want {
		t.Errorf("bindParameters() = %q, want %q", bound, want)
	}
	if unchanged, err := bindParameters("select name from student", nil); err != nil || unchanged != "select name from student" {
		t.Errorf("bindParameters() without placeholders = (%q, %v), want original SQL and nil error", unchanged, err)
	}
	if _, err := bindParameters("select name from student where id = ?", nil); err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("bindParameters() with missing argument error = %v, want argument-count error", err)
	}
	if _, err := bindParameters("select name from student where id = ?", []driver.Value{int64(1), int64(2)}); err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("bindParameters() with extra argument error = %v, want argument-count error", err)
	}
	if _, err := bindParameters("select name from student where note = 'unterminated ?", nil); err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("bindParameters() with unterminated string error = %v, want SQL string error", err)
	}
	if _, err := bindParameters("select name from student where id = ?", []driver.Value{[]byte("unsupported")}); err == nil || !strings.Contains(err.Error(), "unsupported parameter type") {
		t.Errorf("bindParameters() with unsupported value error = %v, want unsupported parameter type", err)
	}
}

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
