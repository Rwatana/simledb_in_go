package network

import (
	"errors"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestConnectRPCSessionRetainsTransactionIDAndLeavesClientOpen(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("ConnectRPCSession() returned a nil session")
	}
	if got := session.TransactionID(); got != 1 {
		t.Errorf("TransactionID() = %d, want 1", got)
	}
	var response CloseResponse
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &response); err != nil {
		t.Fatalf("caller-owned rpc.Client.Call(Close) error = %v, want client to remain open", err)
	}
}

func TestRemoteSessionCommitRetainsSuccessorTransactionID(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	if err := session.Commit(); err != nil {
		t.Fatalf("RemoteSession.Commit() error = %v", err)
	}
	if got := session.TransactionID(); got != 2 {
		t.Errorf("TransactionID() after Commit = %d, want 2", got)
	}
	var response CloseResponse
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &response); err != nil {
		t.Fatalf("caller-owned rpc.Client.Call(Close) error = %v", err)
	}
}

func TestRemoteSessionCommitRejectsInvalidStateAndPreservesIDOnRPCFailure(t *testing.T) {
	var nilSession *RemoteSession
	if err := nilSession.Commit(); err == nil || !strings.Contains(err.Error(), "session is nil") {
		t.Errorf("nil session Commit() error = %v, want nil-session error", err)
	}
	session := &RemoteSession{transactionID: 7}
	if err := session.Commit(); err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("client-less Commit() error = %v, want nil-client error", err)
	}
	if got := session.TransactionID(); got != 7 {
		t.Errorf("TransactionID() after client-less failure = %d, want 7", got)
	}

	client := newTestRPCServiceClient(t, &failingRemoteCommitService{err: errors.New("commit unavailable")})
	session.client = client
	if err := session.Commit(); err == nil || !strings.Contains(err.Error(), "commit unavailable") || !strings.Contains(err.Error(), "commit network RPC client session") {
		t.Fatalf("RPC-failed Commit() error = %v, want contextual remote error", err)
	}
	if got := session.TransactionID(); got != 7 {
		t.Errorf("TransactionID() after RPC failure = %d, want unchanged 7", got)
	}
}

func newTestRPCServiceClient(t *testing.T, service any) *rpc.Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("SimpleDB", service); err != nil {
		t.Fatalf("RegisterName() error = %v", err)
	}
	done := make(chan struct{})
	go func() {
		rpcServer.ServeConn(serverConn)
		close(done)
	}()
	client := rpc.NewClient(clientConn)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("rpc.Client.Close() error = %v", err)
		}
		<-done
	})
	return client
}

type failingRemoteCommitService struct {
	err error
}

func (service *failingRemoteCommitService) Commit(_ CommitRequest, _ *CommitResponse) error {
	return service.err
}

func TestConnectRPCSessionRejectsNilClient(t *testing.T) {
	session, err := ConnectRPCSession(nil)
	if err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Fatalf("ConnectRPCSession(nil) error = %v, want nil-client error", err)
	}
	if session != nil {
		t.Errorf("ConnectRPCSession(nil) session = %#v, want nil", session)
	}
	var nilSession *RemoteSession
	if got := nilSession.TransactionID(); got != 0 {
		t.Errorf("nil session TransactionID() = %d, want 0", got)
	}
}

func TestConnectRPCSessionWrapsRemoteConnectErrorWithoutReturningSession(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 11, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	var client *rpc.Client = newTestRPCClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err == nil || !strings.Contains(err.Error(), "connect network RPC client session") || !strings.Contains(err.Error(), "start") {
		t.Fatalf("ConnectRPCSession() error = %v, want contextual remote START failure", err)
	}
	if session != nil {
		t.Errorf("failed ConnectRPCSession() returned %#v, want nil", session)
	}
}
