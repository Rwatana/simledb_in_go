// Package concurrency coordinates transaction locks on database blocks.
package concurrency

import (
	"errors"
	"sync"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// ErrLockConflict reports that another lock holder prevents the requested transition.
var ErrLockConflict = errors.New("lock conflict")

// LockTable stores the shared lock state for database blocks.
// Its zero value is ready for use.
type LockTable struct {
	mu    sync.Mutex
	locks map[storage.BlockID]int
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
	} else if t.locks[block] == 1 || t.locks[block] == -1 {
		delete(t.locks, block)
	}
}
