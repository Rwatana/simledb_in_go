package concurrency

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func startLockTableWaiter(table *LockTable) (<-chan struct{}, <-chan struct{}) {
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		table.mu.Lock()
		close(ready)
		table.waitForChangeLocked()
		table.mu.Unlock()
		close(done)
	}()
	return ready, done
}

func requireLockWaiterWake(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lock waiter was not notified")
	}
}

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

func TestLockTableAddsCompatibleSharedHolder(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if got := table.locks[block]; got != 2 {
		t.Fatalf("shared holders = %d, want 2", got)
	}
}

func TestLockTableSerializesConcurrentSharedHolders(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	const holders = 32
	var wg sync.WaitGroup
	wg.Add(holders)
	for range holders {
		go func() {
			defer wg.Done()
			if err := table.LockShared(block); err != nil {
				t.Errorf("LockShared() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := table.locks[block]; got != holders {
		t.Fatalf("shared holders = %d, want %d", got, holders)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
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

func TestLockTableUpgradesSoleSharedHolderToExclusive(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}
	if err := table.LockExclusive(block); err != nil {
		t.Fatalf("LockExclusive() error = %v", err)
	}

	if got := table.locks[block]; got != -1 {
		t.Fatalf("exclusive state = %d, want -1", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableRejectsExclusiveUpgradeWithCompetingSharedHolder(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	err := table.LockExclusive(block)
	if !errors.Is(err, ErrLockConflict) {
		t.Fatalf("LockExclusive() error = %v, want ErrLockConflict", err)
	}
	if got := table.locks[block]; got != 2 {
		t.Fatalf("shared holders after conflict = %d, want 2", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableReleasesOneOfMultipleSharedHolders(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	for range 3 {
		if err := table.LockShared(block); err != nil {
			t.Fatal(err)
		}
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	table.Unlock(block)

	if got := table.locks[block]; got != 2 {
		t.Fatalf("shared holders after unlock = %d, want 2", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableRemovesFinalSharedLock(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	table.Unlock(block)

	if _, exists := table.locks[block]; exists {
		t.Fatalf("final shared lock remains in table: %v", table.locks)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableRemovesExclusiveLock(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	table.Unlock(block)

	if _, exists := table.locks[block]; exists {
		t.Fatalf("exclusive lock remains in table: %v", table.locks)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableRejectsSharedAcquisitionWhileExclusiveLockExists(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(other); err != nil {
		t.Fatal(err)
	}

	err := table.LockShared(block)
	if !errors.Is(err, ErrLockConflict) {
		t.Fatalf("LockShared() error = %v, want ErrLockConflict", err)
	}
	if got := table.locks[block]; got != -1 {
		t.Fatalf("exclusive state after conflict = %d, want -1", got)
	}
	if got := table.locks[other]; got != 1 {
		t.Fatalf("other shared holders = %d, want 1", got)
	}
}

func TestLockTableNotifiesWaitersAfterFinalRelease(t *testing.T) {
	t.Run("shared", func(t *testing.T) {
		var table LockTable
		block := storage.NewBlockID("tbl", 1)
		if err := table.LockShared(block); err != nil {
			t.Fatal(err)
		}

		ready, done := startLockTableWaiter(&table)
		<-ready
		table.Unlock(block)
		requireLockWaiterWake(t, done)
	})

	t.Run("exclusive", func(t *testing.T) {
		var table LockTable
		block := storage.NewBlockID("tbl", 1)
		if err := table.LockShared(block); err != nil {
			t.Fatal(err)
		}
		if err := table.LockExclusive(block); err != nil {
			t.Fatal(err)
		}

		ready, done := startLockTableWaiter(&table)
		<-ready
		table.Unlock(block)
		requireLockWaiterWake(t, done)
	})
}

func TestLockTableDoesNotNotifyWaitersAfterPartialSharedRelease(t *testing.T) {
	var table LockTable
	block := storage.NewBlockID("tbl", 1)
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}

	ready, done := startLockTableWaiter(&table)
	<-ready
	table.Unlock(block)
	select {
	case <-done:
		t.Fatal("partial shared release notified lock waiter")
	case <-time.After(20 * time.Millisecond):
	}

	table.Unlock(block)
	requireLockWaiterWake(t, done)
}
