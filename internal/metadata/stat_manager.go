package metadata

import "sync"

// StatManager calculates and caches table statistics.
type StatManager struct {
	mu     sync.Mutex
	tables *TableManager
	stats  map[string]*StatInfo
	calls  int
}

// NewStatManager initializes an empty statistics cache for a table metadata manager.
func NewStatManager(tables *TableManager) *StatManager {
	return &StatManager{
		tables: tables,
		stats:  make(map[string]*StatInfo),
	}
}
