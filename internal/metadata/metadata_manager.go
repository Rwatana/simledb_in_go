package metadata

// MetadataManager coordinates the table, view, statistics, and index metadata managers.
type MetadataManager struct {
	tables  *TableManager
	views   *ViewManager
	stats   *StatManager
	indexes *IndexManager
}

// NewMetadataManager initializes the metadata facade and its shared dependencies.
func NewMetadataManager() *MetadataManager {
	tables := NewTableManager()
	views := NewViewManager(tables)
	stats := NewStatManager(tables)
	indexes := NewIndexManager(tables, stats)
	return &MetadataManager{
		tables:  tables,
		views:   views,
		stats:   stats,
		indexes: indexes,
	}
}
