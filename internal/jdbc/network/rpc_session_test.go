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

func TestRemoteSessionRollbackRetainsSuccessorTransactionID(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	if err := session.Rollback(); err != nil {
		t.Fatalf("RemoteSession.Rollback() error = %v", err)
	}
	if got := session.TransactionID(); got != 2 {
		t.Errorf("TransactionID() after Rollback = %d, want 2", got)
	}
	var response CloseResponse
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &response); err != nil {
		t.Fatalf("caller-owned rpc.Client.Call(Close) error = %v", err)
	}
}

func TestRemoteSessionRollbackRejectsInvalidStateAndPreservesIDOnRPCFailure(t *testing.T) {
	var nilSession *RemoteSession
	if err := nilSession.Rollback(); err == nil || !strings.Contains(err.Error(), "session is nil") {
		t.Errorf("nil session Rollback() error = %v, want nil-session error", err)
	}
	session := &RemoteSession{transactionID: 11}
	if err := session.Rollback(); err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("client-less Rollback() error = %v, want nil-client error", err)
	}
	if got := session.TransactionID(); got != 11 {
		t.Errorf("TransactionID() after client-less failure = %d, want 11", got)
	}

	client := newTestRPCServiceClient(t, &failingRemoteRollbackService{err: errors.New("rollback unavailable")})
	session.client = client
	if err := session.Rollback(); err == nil || !strings.Contains(err.Error(), "rollback unavailable") || !strings.Contains(err.Error(), "roll back network RPC client session") {
		t.Fatalf("RPC-failed Rollback() error = %v, want contextual remote error", err)
	}
	if got := session.TransactionID(); got != 11 {
		t.Errorf("TransactionID() after RPC failure = %d, want unchanged 11", got)
	}
}

func TestRemoteSessionCloseClosesRemoteSessionAndRetainsCallerClient(t *testing.T) {
	service := &remoteSessionLifecycleService{closeErr: errors.New("close unavailable")}
	client := newTestRPCServiceClient(t, service)
	session := &RemoteSession{client: client, transactionID: 19}

	if err := session.Close(); err == nil || !strings.Contains(err.Error(), "close network RPC client session") || !strings.Contains(err.Error(), "close unavailable") {
		t.Fatalf("first RemoteSession.Close() error = %v, want wrapped remote error", err)
	}
	if got := session.TransactionID(); got != 19 {
		t.Errorf("TransactionID() after failed Close = %d, want 19", got)
	}
	if service.closeCalls != 1 {
		t.Errorf("remote Close calls after failure = %d, want 1", service.closeCalls)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("retry RemoteSession.Close() error = %v", err)
	}
	if got := session.TransactionID(); got != 0 {
		t.Errorf("TransactionID() after successful Close = %d, want 0", got)
	}
	if err := session.Close(); err == nil || !strings.Contains(err.Error(), "session is closed") {
		t.Errorf("repeated RemoteSession.Close() error = %v, want local closed-session error", err)
	}
	if err := session.Commit(); err == nil || !strings.Contains(err.Error(), "session is closed") {
		t.Errorf("Commit() after Close error = %v, want local closed-session error", err)
	}
	if err := session.Rollback(); err == nil || !strings.Contains(err.Error(), "session is closed") {
		t.Errorf("Rollback() after Close error = %v, want local closed-session error", err)
	}
	if service.closeCalls != 2 || service.commitCalls != 0 || service.rollbackCalls != 0 {
		t.Errorf("remote calls after local close = close:%d commit:%d rollback:%d, want 2:0:0", service.closeCalls, service.commitCalls, service.rollbackCalls)
	}

	var response CloseResponse
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &response); err == nil || !strings.Contains(err.Error(), "session already closed") {
		t.Errorf("caller-owned rpc.Client call after Close error = %v, want server's already-closed reply", err)
	}
}

func TestRemoteSessionCloseRejectsNilState(t *testing.T) {
	var nilSession *RemoteSession
	if err := nilSession.Close(); err == nil || !strings.Contains(err.Error(), "session is nil") {
		t.Errorf("nil session Close() error = %v, want nil-session error", err)
	}

	session := &RemoteSession{transactionID: 23}
	if err := session.Close(); err == nil || !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("client-less Close() error = %v, want nil-client error", err)
	}
	if got := session.TransactionID(); got != 23 {
		t.Errorf("TransactionID() after client-less Close() = %d, want 23", got)
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

type failingRemoteRollbackService struct {
	err error
}

func (service *failingRemoteRollbackService) Rollback(_ RollbackRequest, _ *RollbackResponse) error {
	return service.err
}

type remoteSessionLifecycleService struct {
	closeErr      error
	closeCalls    int
	commitCalls   int
	rollbackCalls int
	closed        bool
}

func (service *remoteSessionLifecycleService) Close(_ CloseRequest, _ *CloseResponse) error {
	service.closeCalls++
	if service.closed {
		return errors.New("session already closed")
	}
	if service.closeErr != nil {
		err := service.closeErr
		service.closeErr = nil
		return err
	}
	service.closed = true
	return nil
}

func (service *remoteSessionLifecycleService) Commit(_ CommitRequest, _ *CommitResponse) error {
	service.commitCalls++
	return nil
}

func (service *remoteSessionLifecycleService) Rollback(_ RollbackRequest, _ *RollbackResponse) error {
	service.rollbackCalls++
	return nil
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
