package metadata

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// MaxViewDefinitionLength is the maximum number of characters stored for a view definition.
const MaxViewDefinitionLength = 100

// ViewManager stores and retrieves view definitions through the metadata catalogs.
type ViewManager struct {
	tables *TableManager
}

// NewViewManager initializes a view manager with its table metadata dependency.
func NewViewManager(tables *TableManager) *ViewManager {
	return &ViewManager{tables: tables}
}

// InitializeCatalog registers the view catalog for a new database.
func (m *ViewManager) InitializeCatalog(transaction record.TableScanTransaction) error {
	if m.tables == nil {
		return fmt.Errorf("initialize view catalog: table manager is nil")
	}
	if err := m.tables.CreateTable("viewcat", NewViewCatalogSchema(), transaction); err != nil {
		return fmt.Errorf("initialize view catalog: %w", err)
	}
	return nil
}

// CreateView stores a named SQL view definition in the view catalog.
func (m *ViewManager) CreateView(viewName, definition string, transaction record.TableScanTransaction) error {
	if m.tables == nil {
		return fmt.Errorf("create view %q: table manager is nil", viewName)
	}
	layout, err := m.tables.Layout("viewcat", transaction)
	if err != nil {
		return fmt.Errorf("create view %q: load view catalog layout: %w", viewName, err)
	}
	if err := withCatalogScan(transaction, "viewcat", layout, func(scan *record.TableScan) error {
		if err := scan.Insert(); err != nil {
			return fmt.Errorf("insert view catalog row: %w", err)
		}
		if err := scan.SetString("viewname", viewName); err != nil {
			return fmt.Errorf("store view name: %w", err)
		}
		if err := scan.SetString("viewdef", definition); err != nil {
			return fmt.Errorf("store view definition: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create view %q: %w", viewName, err)
	}
	return nil
}

// NewViewCatalogSchema constructs the logical schema of the view catalog.
func NewViewCatalogSchema() *record.Schema {
	var schema record.Schema
	schema.AddVarcharField("viewname", MaxNameLength)
	schema.AddVarcharField("viewdef", MaxViewDefinitionLength)
	return &schema
}
