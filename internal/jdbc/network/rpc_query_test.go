package network

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDriverServiceExecuteQueryReturnsTypedProjectedRowsOverRPC(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	initializeNetworkQueryCatalogs(t, managers)
	client := newTestRPCClient(t, managers)
	var connected ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &connected); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if connected.TransactionID != 2 {
		t.Fatalf("Connect() transaction ID = %d, want 2", connected.TransactionID)
	}
	for _, statement := range []string{
		"create table student (id int, name varchar(20))",
		"insert into student (id, name) values (1, 'Ada')",
		"insert into student (id, name) values (2, 'Grace')",
	} {
		var result ExecuteUpdateResponse
		if err := client.Call("SimpleDB.ExecuteUpdate", ExecuteUpdateRequest{Statement: statement}, &result); err != nil {
			t.Fatalf("ExecuteUpdate(%q) error = %v", statement, err)
		}
	}

	var response ExecuteQueryResponse
	if err := client.Call("SimpleDB.ExecuteQuery", ExecuteQueryRequest{Statement: "select name, id from student"}, &response); err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	wantColumns := []QueryColumn{{Name: "name", Type: "VARCHAR"}, {Name: "id", Type: "INTEGER"}}
	if len(response.Columns) != len(wantColumns) {
		t.Fatalf("ExecuteQuery() columns = %#v, want %#v", response.Columns, wantColumns)
	}
	for index, want := range wantColumns {
		if response.Columns[index] != want {
			t.Errorf("column %d = %#v, want %#v", index, response.Columns[index], want)
		}
	}
	if len(response.Rows) != 2 {
		t.Fatalf("ExecuteQuery() row count = %d, want 2", len(response.Rows))
	}
	gotRows := make(map[string]bool)
	for _, row := range response.Rows {
		if len(row) != 2 || row[0].Kind != "VARCHAR" || row[1].Kind != "INTEGER" {
			t.Fatalf("row = %#v, want VARCHAR then INTEGER values", row)
		}
		gotRows[row[0].String+":"+strconv.FormatInt(int64(row[1].Integer), 10)] = true
	}
	if !gotRows["Ada:1"] || !gotRows["Grace:2"] {
		t.Errorf("ExecuteQuery() rows = %#v, want Ada:1 and Grace:2", response.Rows)
	}

	var committed CommitResponse
	if err := client.Call("SimpleDB.Commit", CommitRequest{}, &committed); err != nil {
		t.Fatalf("Commit() after query error = %v", err)
	}
	if committed.TransactionID != 3 {
		t.Errorf("Commit() next transaction ID = %d, want 3", committed.TransactionID)
	}
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() after commit error = %v", err)
	}
}

func TestDriverServiceExecuteQueryRejectsInvalidStateAndResetsResponse(t *testing.T) {
	var nilService *DriverService
	response := ExecuteQueryResponse{Columns: []QueryColumn{{Name: "old"}}, Rows: [][]QueryValue{{{Kind: "INTEGER", Integer: 9}}}}
	if err := nilService.ExecuteQuery(ExecuteQueryRequest{}, &response); err == nil {
		t.Error("nil service ExecuteQuery() succeeded, want error")
	}
	if len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("nil-service response = %#v, want empty", response)
	}
	if err := (&DriverService{}).ExecuteQuery(ExecuteQueryRequest{}, nil); err == nil {
		t.Error("ExecuteQuery() with nil reply succeeded, want error")
	}
	response = ExecuteQueryResponse{Columns: []QueryColumn{{Name: "old"}}}
	if err := (&DriverService{}).ExecuteQuery(ExecuteQueryRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("ExecuteQuery() without managers error = %v, want manager error", err)
	}
	if len(response.Columns) != 0 {
		t.Errorf("manager-less response = %#v, want empty", response)
	}
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	response.Columns = []QueryColumn{{Name: "old"}}
	if err := (&DriverService{managers: managers}).ExecuteQuery(ExecuteQueryRequest{}, &response); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("ExecuteQuery() without transaction error = %v, want transaction error", err)
	}
	if len(response.Columns) != 0 {
		t.Errorf("missing-transaction response = %#v, want empty", response)
	}
	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	response.Columns = []QueryColumn{{Name: "old"}}
	if err := service.ExecuteQuery(ExecuteQueryRequest{}, &response); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("ExecuteQuery() after Close() error = %v, want closed-session error", err)
	}
	if len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("closed-session response = %#v, want empty", response)
	}
}

func TestDriverServiceExecuteQueryReturnsPlanningAndOpeningErrors(t *testing.T) {
	plannerErr := errors.New("query planner unavailable")
	for _, test := range []struct {
		name    string
		planner plan.QueryPlanner
		wantErr string
	}{
		{name: "parse", wantErr: "parse"},
		{name: "planner", planner: networkQueryPlanner{err: plannerErr}, wantErr: "query planner unavailable"},
		{name: "nil plan", planner: networkQueryPlanner{}, wantErr: "nil plan"},
		{name: "nil schema", planner: networkQueryPlanner{plan: &networkQueryPlan{}}, wantErr: "schema"},
		{name: "open", planner: networkQueryPlanner{plan: &networkQueryPlan{schema: networkQuerySchema("id"), openErr: errors.New("scan open failed")}}, wantErr: "scan open failed"},
		{name: "nil scan", planner: networkQueryPlanner{plan: &networkQueryPlan{schema: networkQuerySchema("id")}}, wantErr: "scan is nil"},
		{name: "typed nil scan", planner: networkQueryPlanner{plan: &networkQueryPlan{schema: networkQuerySchema("id"), scan: (*networkQueryScan)(nil)}}, wantErr: "scan is nil"},
	} {
		t.Run(test.name, func(t *testing.T) {
			managers, service := newNetworkQueryService(t)
			if test.planner != nil {
				managers.QueryPlanner = test.planner
			}
			statement := "select id from student"
			if test.name == "parse" {
				statement = "select"
			}
			response := ExecuteQueryResponse{Columns: []QueryColumn{{Name: "old"}}}
			err := service.ExecuteQuery(ExecuteQueryRequest{Statement: statement}, &response)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ExecuteQuery() error = %v, want %q", err, test.wantErr)
			}
			if len(response.Columns) != 0 || len(response.Rows) != 0 {
				t.Errorf("failed response = %#v, want empty", response)
			}
		})
	}
}

func TestDriverServiceExecuteQueryClosesScansOnMaterializationFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema *record.Schema
		scan   *networkQueryScan
	}{
		{name: "before first", schema: networkQuerySchema("id"), scan: &networkQueryScan{beforeErr: errors.New("position failed")}},
		{name: "next", schema: networkQuerySchema("id"), scan: &networkQueryScan{nextErr: errors.New("advance failed")}},
		{name: "next after a row", schema: networkQuerySchema("id"), scan: &networkQueryScan{rows: []map[string]QueryValue{{"id": {Kind: "INTEGER", Integer: 1}}}, nextErrAfter: 1, nextErr: errors.New("advance failed after row")}},
		{name: "integer read", schema: networkQuerySchema("id"), scan: &networkQueryScan{rows: []map[string]QueryValue{{"id": {Kind: "INTEGER", Integer: 1}}}, intErr: errors.New("integer read failed")}},
		{name: "string read", schema: networkQuerySchemaWithType("name", record.VarcharField), scan: &networkQueryScan{rows: []map[string]QueryValue{{"name": {Kind: "VARCHAR", String: "Ada"}}}, stringErr: errors.New("string read failed")}},
		{name: "unsupported type", schema: networkQuerySchemaWithType("unknown", record.FieldType(99)), scan: &networkQueryScan{}},
		{name: "close", schema: networkQuerySchema("id"), scan: &networkQueryScan{closeErr: errors.New("scan close failed")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			managers, service := newNetworkQueryService(t)
			plan := &networkQueryPlan{schema: test.schema, scan: test.scan}
			managers.QueryPlanner = networkQueryPlanner{plan: plan}
			response := ExecuteQueryResponse{Rows: [][]QueryValue{{{Kind: "INTEGER", Integer: 7}}}}
			if err := service.ExecuteQuery(ExecuteQueryRequest{Statement: "select id from student"}, &response); err == nil {
				t.Fatal("ExecuteQuery() succeeded, want scan error")
			}
			if test.scan.closeCalls != 1 {
				t.Errorf("scan Close() calls = %d, want 1", test.scan.closeCalls)
			}
			if len(response.Columns) != 0 || len(response.Rows) != 0 {
				t.Errorf("failed response = %#v, want empty", response)
			}
		})
	}
	managers, service := newNetworkQueryService(t)
	managers.QueryPlanner = networkQueryPlanner{plan: &networkQueryPlan{schema: networkQuerySchema("id"), scan: &networkQueryScan{}}}
	var response ExecuteQueryResponse
	if err := service.ExecuteQuery(ExecuteQueryRequest{Statement: "select id from student"}, &response); err != nil {
		t.Fatalf("empty ExecuteQuery() error = %v", err)
	}
	if len(response.Columns) != 1 || len(response.Rows) != 0 {
		t.Errorf("empty query response = %#v, want one column and no rows", response)
	}
}

func TestDriverServiceExecuteQueryClosesPartiallyOpenedScans(t *testing.T) {
	for _, closeErr := range []error{nil, errors.New("close after open failure")} {
		t.Run(strconv.FormatBool(closeErr != nil), func(t *testing.T) {
			managers, service := newNetworkQueryService(t)
			scan := &networkQueryScan{closeErr: closeErr}
			managers.QueryPlanner = networkQueryPlanner{plan: &networkQueryPlan{
				schema:  networkQuerySchema("id"),
				scan:    scan,
				openErr: errors.New("scan open failed"),
			}}
			response := ExecuteQueryResponse{Columns: []QueryColumn{{Name: "old"}}}
			err := service.ExecuteQuery(ExecuteQueryRequest{Statement: "select id from student"}, &response)
			if err == nil || !strings.Contains(err.Error(), "scan open failed") {
				t.Fatalf("ExecuteQuery() error = %v, want scan open failure", err)
			}
			if closeErr != nil && !strings.Contains(err.Error(), closeErr.Error()) {
				t.Errorf("ExecuteQuery() error = %v, want close failure %q", err, closeErr)
			}
			if scan.closeCalls != 1 {
				t.Errorf("partially opened scan Close() calls = %d, want 1", scan.closeCalls)
			}
			if len(response.Columns) != 0 || len(response.Rows) != 0 {
				t.Errorf("failed response = %#v, want empty", response)
			}
		})
	}
}

func TestDriverServiceExecuteQueryDoesNotCloseTypedNilScanAfterOpenError(t *testing.T) {
	managers, service := newNetworkQueryService(t)
	managers.QueryPlanner = networkQueryPlanner{plan: &networkQueryPlan{
		schema:  networkQuerySchema("id"),
		scan:    (*networkQueryScan)(nil),
		openErr: errors.New("scan open failed"),
	}}
	response := ExecuteQueryResponse{Columns: []QueryColumn{{Name: "old"}}}
	err := service.ExecuteQuery(ExecuteQueryRequest{Statement: "select id from student"}, &response)
	if err == nil || !strings.Contains(err.Error(), "scan open failed") {
		t.Fatalf("ExecuteQuery() error = %v, want scan open failure", err)
	}
	if len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("failed response = %#v, want empty", response)
	}
}

func initializeNetworkQueryCatalogs(t *testing.T, managers *server.Managers) {
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

func newNetworkQueryService(t *testing.T) (*server.Managers, *DriverService) {
	t.Helper()
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	return managers, &DriverService{managers: managers, transaction: transaction}
}

func networkQuerySchema(fields ...string) *record.Schema {
	return networkQuerySchemaWithTypes(fields, record.IntegerField)
}

func networkQuerySchemaWithType(field string, fieldType record.FieldType) *record.Schema {
	schema := &record.Schema{}
	schema.AddField(field, fieldType, 0)
	return schema
}

func networkQuerySchemaWithTypes(fields []string, fieldType record.FieldType) *record.Schema {
	schema := &record.Schema{}
	for _, field := range fields {
		schema.AddField(field, fieldType, 0)
	}
	return schema
}

type networkQueryPlanner struct {
	plan plan.Plan
	err  error
}

func (planner networkQueryPlanner) CreatePlan(parse.QueryData, record.TableScanTransaction) (plan.Plan, error) {
	return planner.plan, planner.err
}

type networkQueryPlan struct {
	schema  *record.Schema
	scan    query.Scan
	openErr error
}

func (plan *networkQueryPlan) Open() (query.Scan, error) {
	if plan == nil {
		return nil, errors.New("nil test plan")
	}
	return plan.scan, plan.openErr
}
func (plan *networkQueryPlan) BlocksAccessed() int       { return 0 }
func (plan *networkQueryPlan) RecordsOutput() int        { return 0 }
func (plan *networkQueryPlan) DistinctValues(string) int { return 0 }
func (plan *networkQueryPlan) Schema() *record.Schema    { return plan.schema }

type networkQueryScan struct {
	rows         []map[string]QueryValue
	position     int
	nextCalls    int
	beforeErr    error
	nextErr      error
	nextErrAfter int
	intErr       error
	stringErr    error
	closeErr     error
	closeCalls   int
}

func (scan *networkQueryScan) BeforeFirst() error {
	scan.position = 0
	return scan.beforeErr
}
func (scan *networkQueryScan) Next() (bool, error) {
	scan.nextCalls++
	if scan.nextErr != nil && scan.nextCalls > scan.nextErrAfter {
		return false, scan.nextErr
	}
	if scan.position >= len(scan.rows) {
		return false, nil
	}
	scan.position++
	return true, nil
}
func (scan *networkQueryScan) GetInt(field string) (int32, error) {
	if scan.intErr != nil {
		return 0, scan.intErr
	}
	return scan.rows[scan.position-1][field].Integer, nil
}
func (scan *networkQueryScan) GetString(field string) (string, error) {
	if scan.stringErr != nil {
		return "", scan.stringErr
	}
	return scan.rows[scan.position-1][field].String, nil
}
func (scan *networkQueryScan) GetVal(field string) (query.Constant, error) {
	value := scan.rows[scan.position-1][field]
	if value.Kind == "INTEGER" {
		return query.NewIntConstant(value.Integer), nil
	}
	return query.NewStringConstant(value.String), nil
}
func (scan *networkQueryScan) HasField(field string) bool {
	if scan.position == 0 || scan.position > len(scan.rows) {
		return false
	}
	_, ok := scan.rows[scan.position-1][field]
	return ok
}
func (scan *networkQueryScan) Close() error {
	scan.closeCalls++
	return scan.closeErr
}
