package metadata

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

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

// InitializeCatalogs creates and registers the complete metadata catalog set for a new database.
func (m *MetadataManager) InitializeCatalogs(transaction record.TableScanTransaction) error {
	if err := m.tables.InitializeCatalogs(transaction); err != nil {
		return fmt.Errorf("initialize metadata catalogs: table catalogs: %w", err)
	}
	if err := m.views.InitializeCatalog(transaction); err != nil {
		return fmt.Errorf("initialize metadata catalogs: view catalog: %w", err)
	}
	if err := m.indexes.InitializeCatalog(transaction); err != nil {
		return fmt.Errorf("initialize metadata catalogs: index catalog: %w", err)
	}
	return nil
}

// CreateTable stores a table schema through the metadata facade.
func (m *MetadataManager) CreateTable(tableName string, schema *record.Schema, transaction record.TableScanTransaction) error {
	if err := m.tables.CreateTable(tableName, schema, transaction); err != nil {
		return fmt.Errorf("create table %q through metadata manager: %w", tableName, err)
	}
	return nil
}

// Layout reconstructs a table's physical layout through the metadata facade.
func (m *MetadataManager) Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error) {
	layout, err := m.tables.Layout(tableName, transaction)
	if err != nil {
		return nil, fmt.Errorf("look up layout for table %q through metadata manager: %w", tableName, err)
	}
	return layout, nil
}

// CreateView stores a view definition through the metadata facade.
func (m *MetadataManager) CreateView(viewName, definition string, transaction record.TableScanTransaction) error {
	if err := m.views.CreateView(viewName, definition, transaction); err != nil {
		return fmt.Errorf("create view %q through metadata manager: %w", viewName, err)
	}
	return nil
}

// ViewDefinition returns a stored view definition and reports whether it exists.
func (m *MetadataManager) ViewDefinition(viewName string, transaction record.TableScanTransaction) (string, bool, error) {
	definition, found, err := m.views.ViewDefinition(viewName, transaction)
	if err != nil {
		return "", false, fmt.Errorf("look up view %q through metadata manager: %w", viewName, err)
	}
	return definition, found, nil
}

// CreateIndex stores an index registration through the metadata facade.
func (m *MetadataManager) CreateIndex(indexName, tableName, fieldName string, transaction record.TableScanTransaction) error {
	if err := m.indexes.CreateIndex(indexName, tableName, fieldName, transaction); err != nil {
		return fmt.Errorf("create index %q through metadata manager: %w", indexName, err)
	}
	return nil
}

// IndexInfo returns a table's registered indexes through the metadata facade.
func (m *MetadataManager) IndexInfo(tableName string, transaction record.TableScanTransaction) (map[string]*IndexInfo, error) {
	indexes, err := m.indexes.IndexInfo(tableName, transaction)
	if err != nil {
		return nil, fmt.Errorf("look up indexes for table %q through metadata manager: %w", tableName, err)
	}
	return indexes, nil
}

// Statistics returns a table's statistics through the metadata facade.
func (m *MetadataManager) Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*StatInfo, error) {
	stats, err := m.stats.Statistics(tableName, layout, transaction)
	if err != nil {
		return nil, fmt.Errorf("look up statistics for table %q through metadata manager: %w", tableName, err)
	}
	return stats, nil
}
