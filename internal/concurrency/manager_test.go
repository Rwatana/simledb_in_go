package concurrency

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagerReusesTransactionSharedLock(t *testing.T) {
	var table LockTable
	manager := NewManager(&table)
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := manager.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := manager.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := manager.LockShared(other); err != nil {
		t.Fatal(err)
	}

	if got := table.locks[block]; got != 1 {
		t.Fatalf("shared holds after repeated request = %d, want 1", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holds = %d, want 1", got)
	}
	if got := manager.locks[block]; got != sharedLock {
		t.Fatalf("local block lock = %v, want shared", got)
	}
	if got := manager.locks[other]; got != sharedLock {
		t.Fatalf("local other lock = %v, want shared", got)
	}
}

func TestManagerDoesNotRecordSharedLockAfterTableFailure(t *testing.T) {
	wantErr := errors.New("table unavailable")
	table := &failingSharedWaitTable{err: wantErr}
	manager := &Manager{table: table, locks: make(map[storage.BlockID]lockMode)}
	block := storage.NewBlockID("tbl", 7)

	if err := manager.LockShared(block); !errors.Is(err, wantErr) {
		t.Fatalf("LockShared() error = %v, want %v", err, wantErr)
	}
	if _, ok := manager.locks[block]; ok {
		t.Fatalf("transaction-local locks = %v, want failed block absent", manager.locks)
	}
	if table.calls != 1 || table.block != block {
		t.Fatalf("WaitShared() calls/block = %d/%v, want 1/%v", table.calls, table.block, block)
	}
}
