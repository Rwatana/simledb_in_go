package metadata

import "github.com/Rwatana/simledb_in_go/internal/record"

// MaxViewDefinitionLength is the maximum number of characters stored for a view definition.
const MaxViewDefinitionLength = 100

// NewViewCatalogSchema constructs the logical schema of the view catalog.
func NewViewCatalogSchema() *record.Schema {
	var schema record.Schema
	schema.AddVarcharField("viewname", MaxNameLength)
	schema.AddVarcharField("viewdef", MaxViewDefinitionLength)
	return &schema
}
