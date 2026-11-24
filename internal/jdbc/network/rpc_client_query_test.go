package network

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestRemoteSessionExecuteQueryReturnsTypedRowsWithoutCompletingTransaction(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	initializer, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("catalog NewTransaction() error = %v", err)
	}
	if err := managers.InitializeCatalogs(initializer); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	if _, err := initializer.Commit(); err != nil {
		t.Fatalf("catalog Commit() error = %v", err)
	}

	client := newTestRPCClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	initialID := session.TransactionID()
	for _, statement := range []string{
		"create table student (id int, name varchar(20))",
		"insert into student (id, name) values (1, 'Ada')",
		"insert into student (id, name) values (2, 'Grace')",
	} {
		if _, err := session.ExecuteUpdate(statement); err != nil {
			t.Fatalf("RemoteSession.ExecuteUpdate(%q) error = %v", statement, err)
		}
	}

	empty, err := session.ExecuteQuery("select name from student where id = 99")
	if err != nil {
		t.Fatalf("ExecuteQuery(empty) error = %v", err)
	}
	if len(empty.Columns) != 1 || empty.Columns[0] != (QueryColumn{Name: "name", Type: "VARCHAR", DisplaySize: 21}) || len(empty.Rows) != 0 {
		t.Errorf("ExecuteQuery(empty) = %#v, want one VARCHAR column and no rows", empty)
	}

	response, err := session.ExecuteQuery("select name, id from student")
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	wantColumns := []QueryColumn{{Name: "name", Type: "VARCHAR", DisplaySize: 21}, {Name: "id", Type: "INTEGER", DisplaySize: 7}}
	if !reflect.DeepEqual(response.Columns, wantColumns) {
		t.Errorf("ExecuteQuery() columns = %#v, want %#v", response.Columns, wantColumns)
	}
	wantRows := [][]QueryValue{
		{{Kind: "VARCHAR", String: "Ada"}, {Kind: "INTEGER", Integer: 1}},
		{{Kind: "VARCHAR", String: "Grace"}, {Kind: "INTEGER", Integer: 2}},
	}
	if len(response.Rows) != len(wantRows) {
		t.Fatalf("ExecuteQuery() row count = %d, want %d", len(response.Rows), len(wantRows))
	}
	for index, want := range wantRows {
		if !reflect.DeepEqual(response.Rows[index], want) {
			t.Errorf("row %d = %#v, want %#v", index, response.Rows[index], want)
		}
	}
	if got := session.TransactionID(); got != initialID {
		t.Errorf("TransactionID() after query = %d, want unchanged %d", got, initialID)
	}
	if err := session.Close(); err != nil {
		t.Errorf("RemoteSession.Close() after query error = %v", err)
	}
}

func TestRemoteSessionExecuteQueryRejectsInvalidStateAndHidesRPCFailureResults(t *testing.T) {
	var nilSession *RemoteSession
	if response, err := nilSession.ExecuteQuery("select id from student"); err == nil || !strings.Contains(err.Error(), "session is nil") || len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("nil session ExecuteQuery() = %#v, %v; want empty response and nil-session error", response, err)
	}

	session := &RemoteSession{transactionID: 31}
	if response, err := session.ExecuteQuery("select id from student"); err == nil || !strings.Contains(err.Error(), "client is nil") || len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("client-less ExecuteQuery() = %#v, %v; want empty response and nil-client error", response, err)
	}
	if got := session.TransactionID(); got != 31 {
		t.Errorf("TransactionID() after client-less ExecuteQuery() = %d, want 31", got)
	}

	closed := &RemoteSession{closed: true, transactionID: 37}
	if response, err := closed.ExecuteQuery("select id from student"); err == nil || !strings.Contains(err.Error(), "session is closed") || len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("closed session ExecuteQuery() = %#v, %v; want empty response and closed-session error", response, err)
	}

	service := &failingRemoteQueryService{err: errors.New("query unavailable")}
	session.client = newTestRPCServiceClient(t, service)
	response, err := session.ExecuteQuery("select name from student")
	if err == nil || !strings.Contains(err.Error(), "execute network RPC client query") || !strings.Contains(err.Error(), "query unavailable") {
		t.Fatalf("RPC-failed ExecuteQuery() error = %v, want contextual remote error", err)
	}
	if len(response.Columns) != 0 || len(response.Rows) != 0 {
		t.Errorf("RPC-failed ExecuteQuery() response = %#v, want no partial results", response)
	}
	if service.statement != "select name from student" {
		t.Errorf("RPC statement = %q, want caller statement", service.statement)
	}
	if got := session.TransactionID(); got != 31 {
		t.Errorf("TransactionID() after RPC failure = %d, want unchanged 31", got)
	}
}

type failingRemoteQueryService struct {
	err       error
	statement string
}

func (service *failingRemoteQueryService) ExecuteQuery(request ExecuteQueryRequest, reply *ExecuteQueryResponse) error {
	service.statement = request.Statement
	reply.Columns = []QueryColumn{{Name: "partial", Type: "INTEGER"}}
	reply.Rows = [][]QueryValue{{{Kind: "INTEGER", Integer: 1}}}
	return service.err
}
