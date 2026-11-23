package metadata

import "github.com/Rwatana/simledb_in_go/internal/record"

// IndexManager stores and retrieves index registrations through the metadata catalogs.
type IndexManager struct {
	tables *TableManager
	stats  *StatManager
}

// NewIndexManager initializes an index manager with its metadata dependencies.
func NewIndexManager(tables *TableManager, stats *StatManager) *IndexManager {
	return &IndexManager{
		tables: tables,
		stats:  stats,
	}
}

// NewIndexCatalogSchema constructs the logical schema of the index catalog.
func NewIndexCatalogSchema() *record.Schema {
	var schema record.Schema
	schema.AddVarcharField("indexname", MaxNameLength)
	schema.AddVarcharField("tablename", MaxNameLength)
	schema.AddVarcharField("fieldname", MaxNameLength)
	return &schema
}
