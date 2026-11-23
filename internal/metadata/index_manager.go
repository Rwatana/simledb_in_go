package metadata

import "github.com/Rwatana/simledb_in_go/internal/record"

// NewIndexCatalogSchema constructs the logical schema of the index catalog.
func NewIndexCatalogSchema() *record.Schema {
	var schema record.Schema
	schema.AddVarcharField("indexname", MaxNameLength)
	schema.AddVarcharField("tablename", MaxNameLength)
	schema.AddVarcharField("fieldname", MaxNameLength)
	return &schema
}
