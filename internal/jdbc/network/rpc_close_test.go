package network

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDriverServiceCloseCommitsWithoutSuccessorOverRPC(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	client := newTestRPCClient(t, managers)
	var connected ConnectResponse
	if err := client.Call("SimpleDB.Connect", ConnectRequest{}, &connected); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if connected.TransactionID != 1 {
		t.Fatalf("Connect() transaction ID = %d, want 1", connected.TransactionID)
	}
	if err := client.Call("SimpleDB.Close", CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	iterator, err := managers.Logs.Iterator()
	if err != nil {
		t.Fatalf("Logs.Iterator() error = %v", err)
	}
	for _, expected := range []struct {
		operation recovery.Operation
		id        recovery.TransactionID
	}{{recovery.OperationCommit, 1}, {recovery.OperationStart, 1}} {
		encoded, ok, err := iterator.Next()
		if err != nil || !ok {
			t.Fatalf("Iterator.Next() = (_, %t, %v), want transaction record", ok, err)
		}
		decoded, err := recovery.DecodeRecord(encoded)
		if err != nil {
			t.Fatalf("DecodeRecord() error = %v", err)
		}
		record, ok := decoded.(interface {
			Operation() recovery.Operation
			TransactionID() recovery.TransactionID
		})
		if !ok || record.Operation() != expected.operation || record.TransactionID() != expected.id {
			t.Fatalf("record = %#v, want operation %d for transaction %d", decoded, expected.operation, expected.id)
		}
	}
	if encoded, ok, err := iterator.Next(); err != nil || ok || encoded != nil {
		t.Errorf("Iterator.Next() after Close() = (%v, %t, %v), want (nil, false, nil)", encoded, ok, err)
	}
}

func TestDriverServiceCloseRejectsInvalidStateAndReply(t *testing.T) {
	var nilService *DriverService
	if err := nilService.Close(CloseRequest{}, &CloseResponse{}); err == nil {
		t.Error("nil service Close() succeeded, want error")
	}
	if err := (&DriverService{}).Close(CloseRequest{}, nil); err == nil {
		t.Error("Close() with nil reply succeeded, want error")
	}
	if err := (&DriverService{}).Close(CloseRequest{}, &CloseResponse{}); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Close() without transaction error = %v, want transaction error", err)
	}
}

func TestDriverServiceCloseKeepsSessionOpenWhenCommitFails(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	if _, err := active.Commit(); err != nil {
		t.Fatalf("setup Commit() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err == nil || !strings.Contains(err.Error(), "completed") {
		t.Fatalf("Close() error = %v, want completed-transaction failure", err)
	}
	if service.closed || service.transaction != active {
		t.Errorf("failed Close() state = (closed %t, transaction %p), want (false, %p)", service.closed, service.transaction, active)
	}
}

func TestDriverServiceRejectsNewOperationsAfterClose(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !service.closed || service.transaction != nil {
		t.Fatalf("Close() state = (closed %t, transaction %p), want (true, nil)", service.closed, service.transaction)
	}
	var connect ConnectResponse
	if err := service.Connect(ConnectRequest{}, &connect); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Connect() after Close() error = %v, want closed-session error", err)
	}
	var committed CommitResponse
	if err := service.Commit(CommitRequest{}, &committed); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Commit() after Close() error = %v, want closed-session error", err)
	}
	var rolledBack RollbackResponse
	if err := service.Rollback(RollbackRequest{}, &rolledBack); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Rollback() after Close() error = %v, want closed-session error", err)
	}
	if err := service.Close(CloseRequest{}, &CloseResponse{}); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("second Close() error = %v, want closed-session error", err)
	}
}
