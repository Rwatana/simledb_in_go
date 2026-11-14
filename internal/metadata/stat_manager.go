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
