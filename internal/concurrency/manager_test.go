package concurrency

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type failingSharedWaitTable struct {
	err   error
	block storage.BlockID
	calls int
}

func (t *failingSharedWaitTable) WaitShared(block storage.BlockID) error {
	t.calls++
	t.block = block
	return t.err
}

func (t *failingSharedWaitTable) WaitExclusive(storage.BlockID) error { return nil }
func (t *failingSharedWaitTable) Unlock(storage.BlockID)              {}

type failingExclusiveWaitTable struct {
	err   error
	block storage.BlockID
	calls int
}

func (t *failingExclusiveWaitTable) WaitShared(storage.BlockID) error { return nil }
func (t *failingExclusiveWaitTable) WaitExclusive(block storage.BlockID) error {
	t.calls++
	t.block = block
	return t.err
}
func (t *failingExclusiveWaitTable) Unlock(storage.BlockID) {}

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

func TestManagerReleasesAllTransactionLocks(t *testing.T) {
	var table LockTable
	manager := NewManager(&table)
	otherManager := NewManager(&table)
	sharedBlock := storage.NewBlockID("tbl", 1)
	exclusiveBlock := storage.NewBlockID("tbl", 2)
	otherBlock := storage.NewBlockID("tbl", 3)

	if err := manager.LockShared(sharedBlock); err != nil {
		t.Fatal(err)
	}
	if err := manager.LockExclusive(exclusiveBlock); err != nil {
		t.Fatal(err)
	}
	if err := otherManager.LockShared(otherBlock); err != nil {
		t.Fatal(err)
	}

	manager.Release()
	manager.Release()

	if _, ok := table.locks[sharedBlock]; ok {
		t.Fatal("shared block remains globally locked")
	}
	if _, ok := table.locks[exclusiveBlock]; ok {
		t.Fatal("exclusive block remains globally locked")
	}
	if got := table.locks[otherBlock]; got != 1 {
		t.Fatalf("other manager block state = %d, want 1", got)
	}
	if len(manager.locks) != 0 {
		t.Fatalf("local locks after release = %d, want 0", len(manager.locks))
	}
	if got := otherManager.locks[otherBlock]; got != sharedLock {
		t.Fatalf("other manager local lock = %v, want shared", got)
	}
}
