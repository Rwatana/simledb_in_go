package network

import (
	"errors"
	"io"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestServeConnectionRollsBackActiveSessionOnTransportDisconnect(t *testing.T) {
	managers := newDisconnectTestManagers(t)
	client, done, serveErr := newDisconnectTestClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	if _, err := session.ExecuteUpdate("create table abandoned (id int)"); err != nil {
		t.Fatalf("ExecuteUpdate() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("rpc.Client.Close() error = %v", err)
	}
	if err := waitForDisconnectServe(t, done, serveErr); err != nil {
		t.Fatalf("ServeConnection() error = %v", err)
	}
	assertDisconnectTable(t, managers, "abandoned", false)
}

func TestServeConnectionKeepsExplicitSessionCloseCommitted(t *testing.T) {
	managers := newDisconnectTestManagers(t)
	client, done, serveErr := newDisconnectTestClient(t, managers)
	session, err := ConnectRPCSession(client)
	if err != nil {
		t.Fatalf("ConnectRPCSession() error = %v", err)
	}
	if _, err := session.ExecuteUpdate("create table committed (id int)"); err != nil {
		t.Fatalf("ExecuteUpdate() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("RemoteSession.Close() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("rpc.Client.Close() error = %v", err)
	}
	if err := waitForDisconnectServe(t, done, serveErr); err != nil {
		t.Fatalf("ServeConnection() error = %v", err)
	}
	assertDisconnectTable(t, managers, "committed", true)
}

func TestDriverServiceDisconnectHandlesInvalidStateAndRollbackFailure(t *testing.T) {
	var nilService *DriverService
	if err := nilService.disconnect(); err == nil || !strings.Contains(err.Error(), "service is nil") {
		t.Errorf("nil disconnect() error = %v, want nil-service error", err)
	}
	if err := (&DriverService{closed: true}).disconnect(); err != nil {
		t.Errorf("closed disconnect() error = %v, want nil", err)
	}
	serviceWithoutTransaction := &DriverService{}
	if err := serviceWithoutTransaction.disconnect(); err != nil || !serviceWithoutTransaction.closed {
		t.Errorf("disconnect() without transaction = (%v, closed=%t), want nil and closed", err, serviceWithoutTransaction.closed)
	}

	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 4)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	completed, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	if _, err := completed.Rollback(); err != nil {
		t.Fatalf("first Rollback() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: completed}
	if err := service.disconnect(); err == nil || !strings.Contains(err.Error(), "transaction is completed") {
		t.Errorf("disconnect() rollback failure = %v, want completed-transaction error", err)
	}
	if !service.closed || service.transaction != nil {
		t.Errorf("disconnect() after rollback failure left service open or attached: closed=%t transaction=%v", service.closed, service.transaction)
	}
}

func TestServeConnectionReturnsDisconnectRollbackFailure(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 4)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	server := &completedTransactionRPCServer{}
	err = serveConnection(&testNetConn{}, managers, server)
	if err == nil || !strings.Contains(err.Error(), "disconnect network RPC session") || !strings.Contains(err.Error(), "transaction is completed") {
		t.Fatalf("serveConnection() error = %v, want contextual disconnect rollback error", err)
	}
}

func newDisconnectTestManagers(t *testing.T) *server.Managers {
	t.Helper()
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
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
	return managers
}

func newDisconnectTestClient(t *testing.T, managers *server.Managers) (*rpc.Client, <-chan struct{}, *error) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = ServeConnection(serverConn, managers)
		close(done)
	}()
	client := rpc.NewClient(clientConn)
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("ServeConnection() did not stop after the RPC client closed")
		}
	})
	return client, done, &serveErr
}

func waitForDisconnectServe(t *testing.T, done <-chan struct{}, serveErr *error) error {
	t.Helper()
	select {
	case <-done:
		return *serveErr
	case <-time.After(time.Second):
		t.Fatal("ServeConnection() did not return after transport disconnect")
		return nil
	}
}

func assertDisconnectTable(t *testing.T, managers *server.Managers, name string, wantPresent bool) {
	t.Helper()
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("probe NewTransaction() error = %v", err)
	}
	_, lookupErr := managers.Metadata.Layout(name, transaction)
	if _, err := transaction.Rollback(); err != nil {
		t.Errorf("probe transaction Rollback() error = %v", err)
	}
	if wantPresent && lookupErr != nil {
		t.Errorf("Layout(%q) after explicit Close error = %v, want committed table", name, lookupErr)
	}
	if !wantPresent && (lookupErr == nil || !strings.Contains(lookupErr.Error(), "not registered")) {
		t.Errorf("Layout(%q) after transport disconnect error = %v, want uncommitted table not registered", name, lookupErr)
	}
}

type completedTransactionRPCServer struct {
	service *DriverService
}

func (server *completedTransactionRPCServer) RegisterName(name string, receiver any) error {
	if name != "SimpleDB" {
		return errors.New("unexpected RPC service name")
	}
	service, ok := receiver.(*DriverService)
	if !ok {
		return errors.New("unexpected RPC service receiver")
	}
	server.service = service
	return nil
}

func (server *completedTransactionRPCServer) ServeConn(io.ReadWriteCloser) {
	if err := server.service.Connect(ConnectRequest{}, &ConnectResponse{}); err != nil {
		return
	}
	_, _ = server.service.transaction.Rollback()
}
