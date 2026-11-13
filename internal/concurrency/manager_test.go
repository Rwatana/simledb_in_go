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
