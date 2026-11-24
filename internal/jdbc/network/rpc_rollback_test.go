package network

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDriverServiceRollbackRollsBackAndStartsNextTransactionOverRPC(t *testing.T) {
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

	var rolledBack RollbackResponse
	if err := client.Call("SimpleDB.Rollback", RollbackRequest{}, &rolledBack); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rolledBack.TransactionID != 2 {
		t.Fatalf("Rollback() next transaction ID = %d, want 2", rolledBack.TransactionID)
	}

	want := []struct {
		operation recovery.Operation
		id        recovery.TransactionID
	}{
		{recovery.OperationStart, 2},
		{recovery.OperationRollback, 1},
		{recovery.OperationStart, 1},
	}
	iterator, err := managers.Logs.Iterator()
	if err != nil {
		t.Fatalf("Logs.Iterator() error = %v", err)
	}
	for _, expected := range want {
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
		t.Errorf("Iterator.Next() after rollback cycle = (%v, %t, %v), want (nil, false, nil)", encoded, ok, err)
	}
}

func TestDriverServiceRollbackRejectsInvalidStateAndReply(t *testing.T) {
	var nilService *DriverService
	response := RollbackResponse{TransactionID: 77}
	if err := nilService.Rollback(RollbackRequest{}, &response); err == nil {
		t.Error("nil service Rollback() succeeded, want error")
	}
	if response.TransactionID != 0 {
		t.Errorf("nil-service Rollback() transaction ID = %d, want 0", response.TransactionID)
	}
	if err := (&DriverService{}).Rollback(RollbackRequest{}, nil); err == nil {
		t.Error("Rollback() with nil reply succeeded, want error")
	}
	response.TransactionID = 77
	if err := (&DriverService{}).Rollback(RollbackRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Rollback() without managers error = %v, want manager error", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("manager-less Rollback() transaction ID = %d, want 0", response.TransactionID)
	}

	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	service := &DriverService{managers: managers}
	if err := service.Rollback(RollbackRequest{}, &response); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Rollback() without active transaction error = %v, want transaction error", err)
	}
}

func TestDriverServiceRollbackKeepsTransactionWhenRollbackFails(t *testing.T) {
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
	response := RollbackResponse{TransactionID: 77}
	if err := service.Rollback(RollbackRequest{}, &response); err == nil || !strings.Contains(err.Error(), "completed") {
		t.Fatalf("Rollback() error = %v, want completed-transaction failure", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("failed Rollback() transaction ID = %d, want 0", response.TransactionID)
	}
	if service.transaction != active {
		t.Error("failed Rollback() replaced the connection transaction")
	}
}

func TestDriverServiceRollbackLeavesNoStaleTransactionWhenNextStartFails(t *testing.T) {
	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	active, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	service := &DriverService{managers: managers, transaction: active}
	buffers := managers.Buffers
	managers.Buffers = nil
	response := RollbackResponse{TransactionID: 77}
	if err := service.Rollback(RollbackRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Fatalf("Rollback() error = %v, want replacement START failure", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("Rollback() transaction ID after replacement failure = %d, want 0", response.TransactionID)
	}
	if service.transaction != nil {
		t.Error("Rollback() retained the already-completed transaction after replacement failure")
	}

	managers.Buffers = buffers
	var connected ConnectResponse
	if err := service.Connect(ConnectRequest{}, &connected); err != nil {
		t.Fatalf("Connect() retry error = %v", err)
	}
	if connected.TransactionID != 2 {
		t.Errorf("Connect() retry transaction ID = %d, want 2", connected.TransactionID)
	}
}
