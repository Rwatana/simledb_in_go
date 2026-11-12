package concurrency

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestZeroValueLockTableAcquiresFirstSharedLock(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)

	if err := table.LockShared(block); err != nil {
		t.Fatalf("LockShared() error = %v", err)
	}
	if got := table.locks[block]; got != 1 {
		t.Fatalf("shared holders = %d, want 1", got)
	}
}

func TestLockTableKeepsUncontendedBlocksIndependent(t *testing.T) {
	var table LockTable
	first := storage.NewBlockID("tbl", 1)
	second := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(first); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(second); err != nil {
		t.Fatal(err)
	}
	if table.locks[first] != 1 || table.locks[second] != 1 || len(table.locks) != 2 {
		t.Fatalf("locks = %v, want one shared holder on each block", table.locks)
	}
}
