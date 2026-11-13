// Package concurrency coordinates transaction locks on database blocks.
package concurrency

import (
	"errors"
	"sync"
	"time"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// ErrLockConflict reports that another lock holder prevents the requested transition.
var ErrLockConflict = errors.New("lock conflict")

// ErrLockTimeout reports that a lock request remained blocked until its deadline.
var ErrLockTimeout = errors.New("lock wait timed out")

// LockTable stores the shared lock state for database blocks.
// Its zero value is ready for use.
type LockTable struct {
	mu               sync.Mutex
	locks            map[storage.BlockID]int
	changed          *sync.Cond
	exclusiveWaiters int
}

// LockShared acquires a compatible shared lock on block.
func (t *LockTable) LockShared(block storage.BlockID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.locks == nil {
		t.locks = make(map[storage.BlockID]int)
	}
	if t.locks[block] < 0 {
		return ErrLockConflict
	}
	t.locks[block]++
	return nil
}

// WaitShared waits for an exclusive holder to release block, then acquires one shared hold.
func (t *LockTable) WaitShared(block storage.BlockID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.locks[block] < 0 {
		t.waitForChangeLocked()
	}
	if t.locks == nil {
		t.locks = make(map[storage.BlockID]int)
	}
	t.locks[block]++
	return nil
}

// WaitSharedFor waits up to timeout for an exclusive holder to release block, then acquires one shared hold.
func (t *LockTable) WaitSharedFor(block storage.BlockID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.locks[block] < 0 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrLockTimeout
		}
		expired := false
		finished := false
		timer := time.AfterFunc(remaining, func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if finished {
				return
			}
			expired = true
			if t.changed != nil {
				t.changed.Broadcast()
			}
		})
		t.waitForChangeLocked()
		finished = true
		timer.Stop()
		if expired && t.locks[block] < 0 {
			return ErrLockTimeout
		}
	}
	if t.locks == nil {
		t.locks = make(map[storage.BlockID]int)
	}
	t.locks[block]++
	return nil
}

// WaitExclusive waits for competing shared holders to release block, then upgrades the caller's shared hold.
func (t *LockTable) WaitExclusive(block storage.BlockID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.locks[block] > 1 {
		t.exclusiveWaiters++
		t.waitForChangeLocked()
		t.exclusiveWaiters--
	}
	t.locks[block] = -1
	return nil
}

// WaitExclusiveFor waits up to timeout for competing shared holders to release block, then upgrades the caller's shared hold.
func (t *LockTable) WaitExclusiveFor(block storage.BlockID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.locks[block] > 1 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrLockTimeout
		}
		expired := false
		finished := false
		t.exclusiveWaiters++
		timer := time.AfterFunc(remaining, func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if finished {
				return
			}
			expired = true
			if t.changed != nil {
				t.changed.Broadcast()
			}
		})
		t.waitForChangeLocked()
		t.exclusiveWaiters--
		finished = true
		timer.Stop()
		if expired && t.locks[block] > 1 {
			return ErrLockTimeout
		}
	}
	t.locks[block] = -1
	return nil
}

// LockExclusive upgrades the sole shared holder on block to exclusive.
func (t *LockTable) LockExclusive(block storage.BlockID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.locks[block] > 1 {
		return ErrLockConflict
	}
	t.locks[block] = -1
	return nil
}

// Unlock releases one of multiple shared holders on block.
func (t *LockTable) Unlock(block storage.BlockID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.locks[block] > 1 {
		t.locks[block]--
		if t.locks[block] == 1 && t.exclusiveWaiters > 0 && t.changed != nil {
			t.changed.Broadcast()
		}
	} else if t.locks[block] == 1 || t.locks[block] == -1 {
		delete(t.locks, block)
		if t.changed != nil {
			t.changed.Broadcast()
		}
	}
}

func (t *LockTable) waitForChangeLocked() {
	if t.changed == nil {
		t.changed = sync.NewCond(&t.mu)
	}
	t.changed.Wait()
}
