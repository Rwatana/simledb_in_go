package metadata

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

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

// InitializeCatalog registers the index catalog for a new database.
func (m *IndexManager) InitializeCatalog(transaction record.TableScanTransaction) error {
	if m.tables == nil {
		return fmt.Errorf("initialize index catalog: table manager is nil")
	}
	if err := m.tables.CreateTable("idxcat", NewIndexCatalogSchema(), transaction); err != nil {
		return fmt.Errorf("initialize index catalog: %w", err)
	}
	return nil
}

// CreateIndex stores an index registration in the index catalog.
func (m *IndexManager) CreateIndex(indexName, tableName, fieldName string, transaction record.TableScanTransaction) error {
	if m.tables == nil {
		return fmt.Errorf("create index %q: table manager is nil", indexName)
	}
	layout, err := m.tables.Layout("idxcat", transaction)
	if err != nil {
		return fmt.Errorf("create index %q: load index catalog layout: %w", indexName, err)
	}
	if err := withCatalogScan(transaction, "idxcat", layout, func(scan *record.TableScan) error {
		if err := scan.Insert(); err != nil {
			return fmt.Errorf("insert index catalog row: %w", err)
		}
		if err := scan.SetString("indexname", indexName); err != nil {
			return fmt.Errorf("store index name: %w", err)
		}
		if err := scan.SetString("tablename", tableName); err != nil {
			return fmt.Errorf("store indexed table name: %w", err)
		}
		if err := scan.SetString("fieldname", fieldName); err != nil {
			return fmt.Errorf("store indexed field name: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create index %q: %w", indexName, err)
	}
	return nil
}

// NewIndexCatalogSchema constructs the logical schema of the index catalog.
func NewIndexCatalogSchema() *record.Schema {
	var schema record.Schema
	schema.AddVarcharField("indexname", MaxNameLength)
	schema.AddVarcharField("tablename", MaxNameLength)
	schema.AddVarcharField("fieldname", MaxNameLength)
	return &schema
}
