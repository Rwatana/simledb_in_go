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

func TestManagerUpgradesTransactionLockToExclusiveOnce(t *testing.T) {
	var table LockTable
	manager := NewManager(&table)
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := manager.LockShared(other); err != nil {
		t.Fatal(err)
	}
	if err := manager.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := manager.LockExclusive(block); err != nil {
		t.Fatal(err)
	}

	if got := table.locks[block]; got != -1 {
		t.Fatalf("exclusive block state = %d, want -1", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared block state = %d, want 1", got)
	}
	if got := manager.locks[block]; got != exclusiveLock {
		t.Fatalf("local block lock = %v, want exclusive", got)
	}
	if got := manager.locks[other]; got != sharedLock {
		t.Fatalf("local other lock = %v, want shared", got)
	}
}

func TestManagerPreservesSharedLockAfterExclusiveUpgradeFailure(t *testing.T) {
	wantErr := errors.New("exclusive upgrade unavailable")
	table := &failingExclusiveWaitTable{err: wantErr}
	manager := &Manager{table: table, locks: make(map[storage.BlockID]lockMode)}
	block := storage.NewBlockID("tbl", 8)

	if err := manager.LockExclusive(block); !errors.Is(err, wantErr) {
		t.Fatalf("LockExclusive() error = %v, want %v", err, wantErr)
	}
	if got := manager.locks[block]; got != sharedLock {
		t.Fatalf("transaction-local lock = %v, want shared after failed upgrade", got)
	}
	if table.calls != 1 || table.block != block {
		t.Fatalf("WaitExclusive() calls/block = %d/%v, want 1/%v", table.calls, table.block, block)
	}
}

func TestManagerDoesNotRecordLockWhenExclusivePrerequisiteFails(t *testing.T) {
	wantErr := errors.New("shared prerequisite unavailable")
	table := &failingSharedWaitTable{err: wantErr}
	manager := &Manager{table: table, locks: make(map[storage.BlockID]lockMode)}
	block := storage.NewBlockID("tbl", 9)

	if err := manager.LockExclusive(block); !errors.Is(err, wantErr) {
		t.Fatalf("LockExclusive() error = %v, want %v", err, wantErr)
	}
	if _, ok := manager.locks[block]; ok {
		t.Fatalf("transaction-local locks = %v, want failed block absent", manager.locks)
	}
	if table.calls != 1 || table.block != block {
		t.Fatalf("WaitShared() calls/block = %d/%v, want 1/%v", table.calls, table.block, block)
	}
}
