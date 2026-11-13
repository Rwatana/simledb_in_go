package concurrency

import "github.com/Rwatana/simledb_in_go/internal/storage"

type lockMode uint8

const (
	sharedLock lockMode = iota + 1
)

// Manager tracks the block locks held by one transaction.
type Manager struct {
	table *LockTable
	locks map[storage.BlockID]lockMode
}

// NewManager creates transaction-local lock tracking backed by table.
func NewManager(table *LockTable) *Manager {
	return &Manager{
		table: table,
		locks: make(map[storage.BlockID]lockMode),
	}
}

// LockShared acquires a shared block lock unless this manager already tracks one.
func (m *Manager) LockShared(block storage.BlockID) error {
	if _, ok := m.locks[block]; ok {
		return nil
	}
	if err := m.table.WaitShared(block); err != nil {
		return err
	}
	m.locks[block] = sharedLock
	return nil
}
