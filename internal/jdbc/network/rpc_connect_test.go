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

	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestServeConnectionCreatesOneTransactionOnRemoteConnect(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)

	var response ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &response); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if response.TransactionID != 1 {
		t.Fatalf("Connect() transaction ID = %d, want 1", response.TransactionID)
	}

	var secondResponse ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &secondResponse); err == nil {
		t.Fatal("second Connect() succeeded, want an already-connected error")
	}
	if secondResponse.TransactionID != 0 {
		t.Errorf("failed Connect() transaction ID = %d, want 0", secondResponse.TransactionID)
	}

	iterator, err := managers.Logs.Iterator()
	if err != nil {
		t.Fatalf("Logs.Iterator() error = %v", err)
	}
	encoded, ok, err := iterator.Next()
	if err != nil || !ok {
		t.Fatalf("Iterator.Next() = (_, %t, %v), want one START record", ok, err)
	}
	decoded, err := recovery.DecodeRecord(encoded)
	if err != nil {
		t.Fatalf("DecodeRecord() error = %v", err)
	}
	start, ok := decoded.(interface {
		Operation() recovery.Operation
		TransactionID() recovery.TransactionID
	})
	if !ok || start.Operation() != recovery.OperationStart || start.TransactionID() != 1 {
		t.Fatalf("record = %#v, want START for transaction 1", decoded)
	}
	if encoded, ok, err := iterator.Next(); err != nil || ok || encoded != nil {
		t.Errorf("Iterator.Next() after one Connect() = (%v, %t, %v), want (nil, false, nil)", encoded, ok, err)
	}
}

func TestServeConnectionReportsManagerTransactionFailureOverRPC(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 11, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	var response ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &response); err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("Connect() error = %v, want START creation failure", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("failed Connect() transaction ID = %d, want 0", response.TransactionID)
	}
}

func TestServeConnectionRejectsNilAndIncompleteInputs(t *testing.T) {
	if err := ServeConnection(nil, &server.Managers{}); err == nil || !strings.Contains(err.Error(), "connection") {
		t.Errorf("ServeConnection(nil, managers) error = %v, want connection error", err)
	}
	if err := ServeConnection(&testNetConn{}, nil); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("ServeConnection(conn, nil) error = %v, want manager error", err)
	}

	client := newTestRPCClient(t, &server.Managers{})
	var response ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Fatalf("Connect() with incomplete managers error = %v, want manager error", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("failed Connect() transaction ID = %d, want 0", response.TransactionID)
	}
}

func TestDriverServiceConnectRejectsInvalidReceiverAndReply(t *testing.T) {
	var nilService *DriverService
	if err := nilService.Connect(ConnectRequest{}, &ConnectResponse{}); err == nil {
		t.Error("nil service Connect() succeeded, want error")
	}
	if err := (&DriverService{}).Connect(ConnectRequest{}, &ConnectResponse{}); err == nil {
		t.Error("unbound service Connect() succeeded, want error")
	}
	if err := (&DriverService{}).Connect(ConnectRequest{}, nil); err == nil {
		t.Error("Connect() with nil reply succeeded, want error")
	}
	reply := ConnectResponse{TransactionID: 73}
	if err := (&DriverService{managers: &server.Managers{}}).Connect(ConnectRequest{}, &reply); err == nil {
		t.Fatal("Connect() with incomplete managers succeeded, want error")
	}
	if reply.TransactionID != 0 {
		t.Errorf("failed Connect() transaction ID = %d, want 0", reply.TransactionID)
	}
}

func TestRegisterDriverServiceRejectsInvalidInputsAndDuplicateRegistration(t *testing.T) {
	if err := registerDriverService(nil, &DriverService{}); err == nil {
		t.Error("registerDriverService(nil, service) succeeded, want error")
	}
	registrar := rpc.NewServer()
	if err := registerDriverService(registrar, nil); err == nil {
		t.Error("registerDriverService(registrar, nil) succeeded, want error")
	}
	service := &DriverService{}
	if err := registerDriverService(registrar, service); err != nil {
		t.Fatalf("registerDriverService() error = %v", err)
	}
	if err := registerDriverService(registrar, service); err == nil || !strings.Contains(err.Error(), "already defined") {
		t.Errorf("duplicate registerDriverService() error = %v, want duplicate-name error", err)
	}
}

func TestServeConnectionWrapsRegistrationFailures(t *testing.T) {
	managerSet := &server.Managers{}
	successfulServer := &fakeRPCConnectionServer{}
	if err := serveConnection(&testNetConn{}, managerSet, successfulServer); err != nil {
		t.Fatalf("serveConnection() error = %v", err)
	}
	if successfulServer.registerName != "SimpleDB" {
		t.Errorf("RegisterName() name = %q, want SimpleDB", successfulServer.registerName)
	}
	if service, ok := successfulServer.receiver.(*DriverService); !ok || service.managers != managerSet {
		t.Errorf("RegisterName() receiver = %#v, want DriverService bound to manager set", successfulServer.receiver)
	}
	if successfulServer.serveCalls != 1 {
		t.Errorf("ServeConn() calls = %d, want 1", successfulServer.serveCalls)
	}

	rpcServer := &fakeRPCConnectionServer{registerErr: errors.New("registration failed")}
	if err := serveConnection(&testNetConn{}, managerSet, rpcServer); err == nil || !strings.Contains(err.Error(), "registration failed") {
		t.Fatalf("serveConnection() error = %v, want wrapped registration failure", err)
	}
	if rpcServer.serveCalls != 0 {
		t.Errorf("ServeConn() calls = %d, want 0 after registration failure", rpcServer.serveCalls)
	}
	if err := serveConnection(&testNetConn{}, &server.Managers{}, nil); err == nil || !strings.Contains(err.Error(), "RPC server") {
		t.Errorf("serveConnection() with nil server error = %v, want RPC server error", err)
	}
}

func newTestRPCClient(t *testing.T, managers *server.Managers) *rpc.Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- ServeConnection(serverConn, managers)
	}()
	client := rpc.NewClient(clientConn)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("rpc.Client.Close() error = %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("ServeConnection() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Error("ServeConnection() did not stop after the client closed")
		}
	})
	return client
}

// testNetConn is an inert connection value for testing ServeConnection input validation.
type testNetConn struct{}

func (*testNetConn) Read([]byte) (int, error)         { return 0, nil }
func (*testNetConn) Write([]byte) (int, error)        { return 0, nil }
func (*testNetConn) Close() error                     { return nil }
func (*testNetConn) LocalAddr() net.Addr              { return nil }
func (*testNetConn) RemoteAddr() net.Addr             { return nil }
func (*testNetConn) SetDeadline(time.Time) error      { return nil }
func (*testNetConn) SetReadDeadline(time.Time) error  { return nil }
func (*testNetConn) SetWriteDeadline(time.Time) error { return nil }

type fakeRPCConnectionServer struct {
	registerErr  error
	registerName string
	receiver     any
	serveCalls   int
}

func (s *fakeRPCConnectionServer) RegisterName(name string, receiver any) error {
	s.registerName = name
	s.receiver = receiver
	return s.registerErr
}
func (s *fakeRPCConnectionServer) ServeConn(io.ReadWriteCloser) { s.serveCalls++ }
