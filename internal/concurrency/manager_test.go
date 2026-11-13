package concurrency

import (
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
