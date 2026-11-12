// Package concurrency coordinates transaction locks on database blocks.
package concurrency

import (
	"sync"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

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
	t.locks[block]++
	return nil
}
