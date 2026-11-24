package network

import (
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
