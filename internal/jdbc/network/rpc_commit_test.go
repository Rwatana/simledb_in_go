package network

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/server"
)

func TestDriverServiceCommitCommitsAndStartsNextTransactionOverRPC(t *testing.T) {
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

	var committed CommitResponse
	if err := client.Call("SimpleDB.Commit", CommitRequest{}, &committed); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if committed.TransactionID != 2 {
		t.Fatalf("Commit() next transaction ID = %d, want 2", committed.TransactionID)
	}

	want := []struct {
		operation recovery.Operation
		id        recovery.TransactionID
	}{
		{recovery.OperationStart, 2},
		{recovery.OperationCommit, 1},
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
		t.Errorf("Iterator.Next() after commit cycle = (%v, %t, %v), want (nil, false, nil)", encoded, ok, err)
	}
}

func TestDriverServiceCommitRejectsInvalidStateAndReply(t *testing.T) {
	var nilService *DriverService
	response := CommitResponse{TransactionID: 77}
	if err := nilService.Commit(CommitRequest{}, &response); err == nil {
		t.Error("nil service Commit() succeeded, want error")
	}
	if response.TransactionID != 0 {
		t.Errorf("nil-service Commit() transaction ID = %d, want 0", response.TransactionID)
	}
	if err := (&DriverService{}).Commit(CommitRequest{}, nil); err == nil {
		t.Error("Commit() with nil reply succeeded, want error")
	}
	response.TransactionID = 77
	if err := (&DriverService{}).Commit(CommitRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Errorf("Commit() without managers error = %v, want manager error", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("manager-less Commit() transaction ID = %d, want 0", response.TransactionID)
	}

	managers, err := server.NewManagers(filepath.Join(t.TempDir(), "database"), 64, 1)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	service := &DriverService{managers: managers}
	if err := service.Commit(CommitRequest{}, &response); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("Commit() without active transaction error = %v, want transaction error", err)
	}
}

func TestDriverServiceCommitKeepsTransactionWhenCommitFails(t *testing.T) {
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
	response := CommitResponse{TransactionID: 77}
	if err := service.Commit(CommitRequest{}, &response); err == nil || !strings.Contains(err.Error(), "completed") {
		t.Fatalf("Commit() error = %v, want completed-transaction failure", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("failed Commit() transaction ID = %d, want 0", response.TransactionID)
	}
	if service.transaction != active {
		t.Error("failed Commit() replaced the connection transaction")
	}
}

func TestDriverServiceCommitLeavesNoStaleTransactionWhenNextStartFails(t *testing.T) {
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
	response := CommitResponse{TransactionID: 77}
	if err := service.Commit(CommitRequest{}, &response); err == nil || !strings.Contains(err.Error(), "manager") {
		t.Fatalf("Commit() error = %v, want replacement START failure", err)
	}
	if response.TransactionID != 0 {
		t.Errorf("Commit() transaction ID after replacement failure = %d, want 0", response.TransactionID)
	}
	if service.transaction != nil {
		t.Error("Commit() retained the already-completed transaction after replacement failure")
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
