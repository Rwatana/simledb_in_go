package metadata

import "github.com/Rwatana/simledb_in_go/internal/record"

// TableManager stores the catalog layouts used to persist and retrieve table metadata.
type TableManager struct {
	tableCatalogLayout *record.Layout
	fieldCatalogLayout *record.Layout
}

// NewTableManager initializes a table manager with the table and field catalog layouts.
func NewTableManager() *TableManager {
	return &TableManager{
		tableCatalogLayout: NewTableCatalogLayout(),
		fieldCatalogLayout: NewFieldCatalogLayout(),
	}
}
