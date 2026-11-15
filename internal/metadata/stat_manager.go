package metadata

import (
	"fmt"
	"sync"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

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

// Statistics returns the cached statistics for a table, calculating them on the first lookup.
func (m *StatManager) Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*StatInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls > 100 {
		if err := m.refreshLocked(transaction); err != nil {
			return nil, fmt.Errorf("look up statistics for table %q: refresh cache: %w", tableName, err)
		}
	}
	if stats := m.stats[tableName]; stats != nil {
		return stats, nil
	}
	stats, err := m.calculateTableStats(tableName, layout, transaction)
	if err != nil {
		return nil, fmt.Errorf("look up statistics for table %q: %w", tableName, err)
	}
	m.stats[tableName] = stats
	return stats, nil
}

// Refresh rebuilds statistics for every registered table and resets the lookup counter.
func (m *StatManager) Refresh(transaction record.TableScanTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked(transaction)
}

func (m *StatManager) refreshLocked(transaction record.TableScanTransaction) error {
	if m.tables == nil {
		return fmt.Errorf("refresh table statistics: table manager is nil")
	}
	tableCatalogLayout, err := m.tables.Layout("tblcat", transaction)
	if err != nil {
		return fmt.Errorf("refresh table statistics: load table catalog layout: %w", err)
	}
	refreshed := make(map[string]*StatInfo)
	if err := withCatalogScan(transaction, "tblcat", tableCatalogLayout, func(scan *record.TableScan) error {
		for {
			found, err := scan.Next()
			if err != nil {
				return fmt.Errorf("scan registered tables: %w", err)
			}
			if !found {
				return nil
			}
			tableName, err := scan.GetString("tblname")
			if err != nil {
				return fmt.Errorf("read registered table name: %w", err)
			}
			layout, err := m.tables.Layout(tableName, transaction)
			if err != nil {
				return fmt.Errorf("load layout for table %q: %w", tableName, err)
			}
			stats, err := m.calculateTableStats(tableName, layout, transaction)
			if err != nil {
				return fmt.Errorf("calculate table %q: %w", tableName, err)
			}
			refreshed[tableName] = stats
		}
	}); err != nil {
		return fmt.Errorf("refresh table statistics: %w", err)
	}
	m.stats = refreshed
	m.calls = 0
	return nil
}

func (m *StatManager) calculateTableStats(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*StatInfo, error) {
	blocks := 0
	records := 0
	if err := withCatalogScan(transaction, tableName, layout, func(scan *record.TableScan) error {
		for {
			found, err := scan.Next()
			if err != nil {
				return fmt.Errorf("scan records: %w", err)
			}
			if !found {
				return nil
			}
			records++
			rid, err := scan.GetRID()
			if err != nil {
				return fmt.Errorf("read RID for record %d: %w", records, err)
			}
			blocks = rid.BlockNumber() + 1
		}
	}); err != nil {
		return nil, fmt.Errorf("calculate statistics for table %q: %w", tableName, err)
	}
	return NewStatInfo(blocks, records), nil
}
