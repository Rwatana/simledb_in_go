package embedded

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestPreparedStatementQueryReturnsProjectedRowsToDatabaseSQL(t *testing.T) {
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
	for _, values := range []string{"(1, 'Ada')", "(2, 'Grace')"} {
		if _, err := transaction.Exec("insert into student (id, name) values " + values); err != nil {
			t.Fatalf("transaction Exec(INSERT %s) error = %v", values, err)
		}
	}
	statement, err := transaction.PrepareContext(context.Background(), "select name, id from student where id = 2")
	if err != nil {
		t.Fatalf("transaction PrepareContext() error = %v", err)
	}
	rows, err := statement.QueryContext(context.Background())
	if err != nil {
		t.Fatalf("statement QueryContext() error = %v", err)
	}
	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("rows Columns() error = %v", err)
	}
	if want := []string{"name", "id"}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("rows Columns() = %v, want %v", columns, want)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		t.Fatalf("rows ColumnTypes() error = %v", err)
	}
	if len(columnTypes) != 2 {
		t.Fatalf("rows ColumnTypes() returned %d types, want 2", len(columnTypes))
	}
	for index, want := range []string{"VARCHAR", "INTEGER"} {
		if got := columnTypes[index].DatabaseTypeName(); got != want {
			t.Errorf("column %q database type = %q, want %q", columns[index], got, want)
		}
	}
	for index, want := range []reflect.Type{reflect.TypeOf(""), reflect.TypeOf(int64(0))} {
		if got := columnTypes[index].ScanType(); got != want {
			t.Errorf("column %q scan type = %v, want %v", columns[index], got, want)
		}
	}
	for index, want := range []struct {
		length int64
		ok     bool
	}{{20, true}, {0, false}} {
		length, ok := columnTypes[index].Length()
		if length != want.length || ok != want.ok {
			t.Errorf("column %q length = (%d, %t), want (%d, %t)", columns[index], length, ok, want.length, want.ok)
		}
	}
	for index := range columnTypes {
		nullable, ok := columnTypes[index].Nullable()
		if nullable || !ok {
			t.Errorf("column %q nullable = (%t, %t), want (false, true)", columns[index], nullable, ok)
		}
	}
	if !rows.Next() {
		t.Fatalf("rows Next() = false, error = %v, want one matching row", rows.Err())
	}
	var name string
	var id int64
	if err := rows.Scan(&name, &id); err != nil {
		t.Fatalf("rows Scan() error = %v", err)
	}
	if name != "Grace" || id != 2 {
		t.Errorf("scanned row = (%q, %d), want (%q, 2)", name, id, "Grace")
	}
	if rows.Next() || rows.Err() != nil {
		t.Errorf("rows Next() after result = true or error %v, want clean exhaustion", rows.Err())
	}
	if err := rows.Close(); err != nil {
		t.Errorf("rows Close() after exhaustion error = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Errorf("repeated rows Close() error = %v", err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("transaction Rollback() error = %v", err)
	}
}

func TestStatementQueryRejectsInvalidStateAndArguments(t *testing.T) {
	var nilStatement *statement
	if rows, err := nilStatement.Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "statement") {
		t.Errorf("Query() on nil statement = (%v, %v), want statement error", rows, err)
	}
	if rows, err := (&statement{closed: true}).Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Query() on closed statement = (%v, %v), want closed error", rows, err)
	}
	if rows, err := (&statement{}).Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("Query() without connection = (%v, %v), want connection error", rows, err)
	}
	closedConnection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	if err := closedConnection.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if rows, err := (&statement{connection: closedConnection}).Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Query() on closed connection = (%v, %v), want closed error", rows, err)
	}
	if rows, err := (&statement{connection: &Connection{}}).Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Query() without managers = (%v, %v), want manager error", rows, err)
	}

	connection, err := NewConnection(newEmbeddedManagers(t))
	if err != nil {
		t.Fatalf("NewConnection() for inactive transaction error = %v", err)
	}
	if rows, err := (&statement{connection: connection}).Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Query() without transaction = (%v, %v), want transaction error", rows, err)
	}
	transaction, err := connection.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	value := &statement{connection: connection, query: "select id from student"}
	if rows, err := value.Query([]driver.Value{int64(1)}); rows != nil || err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Errorf("Query() with arguments = (%v, %v), want argument error", rows, err)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
}

func TestStatementQueryWrapsPlannerOpenAndRowsCreationFailures(t *testing.T) {
	managers := newEmbeddedManagers(t)
	connection, err := NewConnection(managers)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	if _, err := connection.Begin(); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	statement := &statement{connection: connection, query: "select id from student"}

	plannerErr := errors.New("planner failed")
	managers.QueryPlanner = statementQueryPlannerStub{err: plannerErr}
	if rows, err := statement.Query(nil); rows != nil || !errors.Is(err, plannerErr) {
		t.Errorf("Query() planner error = (%v, %v), want wrapped planner error", rows, err)
	}
	managers.QueryPlanner = statementQueryPlannerStub{}
	if rows, err := statement.Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Errorf("Query() nil plan = (%v, %v), want plan error", rows, err)
	}

	openErr := errors.New("open failed")
	managers.QueryPlanner = statementQueryPlannerStub{plan: statementQueryPlanStub{openErr: openErr}}
	if rows, err := statement.Query(nil); rows != nil || !errors.Is(err, openErr) {
		t.Errorf("Query() open error = (%v, %v), want wrapped open error", rows, err)
	}

	closeErr := errors.New("close failed")
	scan := &statementRowsScanStub{closeErr: closeErr}
	managers.QueryPlanner = statementQueryPlannerStub{plan: statementQueryPlanStub{scan: scan}}
	if rows, err := statement.Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "close rejected query scan") || !errors.Is(err, closeErr) {
		t.Errorf("Query() rows-creation and scan-close errors = (%v, %v), want combined contextual error", rows, err)
	}
	if scan.closeCalls != 1 {
		t.Errorf("rejected query scan Close() calls = %d, want 1", scan.closeCalls)
	}
	managers.QueryPlanner = statementQueryPlannerStub{plan: statementQueryPlanStub{schema: &record.Schema{}}}
	if rows, err := statement.Query(nil); rows != nil || err == nil || !strings.Contains(err.Error(), "scan is nil") {
		t.Errorf("Query() nil scan = (%v, %v), want scan error", rows, err)
	}
}

type statementQueryPlannerStub struct {
	plan plan.Plan
	err  error
}

func (planner statementQueryPlannerStub) CreatePlan(parse.QueryData, record.TableScanTransaction) (plan.Plan, error) {
	return planner.plan, planner.err
}

type statementQueryPlanStub struct {
	scan    query.Scan
	schema  *record.Schema
	openErr error
}

func (queryPlan statementQueryPlanStub) Open() (query.Scan, error) {
	return queryPlan.scan, queryPlan.openErr
}

func (statementQueryPlanStub) BlocksAccessed() int { return 0 }

func (statementQueryPlanStub) RecordsOutput() int { return 0 }

func (statementQueryPlanStub) DistinctValues(string) int { return 0 }

func (queryPlan statementQueryPlanStub) Schema() *record.Schema { return queryPlan.schema }
