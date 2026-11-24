package network

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestRemoteSessionExecuteUpdateReturnsAffectedRowsWithoutCompletingTransaction(t *testing.T) {
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
	for _, test := range []struct {
		statement string
		wantRows  int64
	}{
		{statement: "create table student (id int, name varchar(20))", wantRows: 0},
		{statement: "insert into student (id, name) values (1, 'Ada')", wantRows: 1},
	} {
		gotRows, err := session.ExecuteUpdate(test.statement)
		if err != nil {
			t.Fatalf("RemoteSession.ExecuteUpdate(%q) error = %v", test.statement, err)
		}
		if gotRows != test.wantRows {
			t.Errorf("RemoteSession.ExecuteUpdate(%q) rows = %d, want %d", test.statement, gotRows, test.wantRows)
		}
	}
	if got := session.TransactionID(); got != initialID {
		t.Errorf("TransactionID() after update RPCs = %d, want unchanged %d", got, initialID)
	}
	if err := session.Close(); err != nil {
		t.Errorf("RemoteSession.Close() after updates error = %v", err)
	}
}

func TestRemoteSessionExecuteUpdateRejectsInvalidStateAndWrapsRPCFailure(t *testing.T) {
	var nilSession *RemoteSession
	if _, err := nilSession.ExecuteUpdate("insert"); err == nil || !strings.Contains(err.Error(), "session is nil") {
		t.Errorf("nil session ExecuteUpdate() error = %v, want nil-session error", err)
	}

	session := &RemoteSession{transactionID: 17}
	if _, err := session.ExecuteUpdate("insert"); err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("client-less ExecuteUpdate() error = %v, want nil-client error", err)
	}
	if got := session.TransactionID(); got != 17 {
		t.Errorf("TransactionID() after client-less ExecuteUpdate() = %d, want 17", got)
	}

	closed := &RemoteSession{closed: true, transactionID: 19}
	if _, err := closed.ExecuteUpdate("insert"); err == nil || !strings.Contains(err.Error(), "session is closed") {
		t.Errorf("closed session ExecuteUpdate() error = %v, want closed-session error", err)
	}

	service := &failingRemoteUpdateService{err: errors.New("update unavailable")}
	session.client = newTestRPCServiceClient(t, service)
	if _, err := session.ExecuteUpdate("insert into student values (1)"); err == nil || !strings.Contains(err.Error(), "execute network RPC client update") || !strings.Contains(err.Error(), "update unavailable") {
		t.Fatalf("RPC-failed ExecuteUpdate() error = %v, want contextual remote error", err)
	}
	if service.statement != "insert into student values (1)" {
		t.Errorf("RPC statement = %q, want caller statement", service.statement)
	}
	if got := session.TransactionID(); got != 17 {
		t.Errorf("TransactionID() after RPC failure = %d, want unchanged 17", got)
	}
}

type failingRemoteUpdateService struct {
	err       error
	statement string
}

func (service *failingRemoteUpdateService) ExecuteUpdate(request ExecuteUpdateRequest, _ *ExecuteUpdateResponse) error {
	service.statement = request.Statement
	return service.err
}
