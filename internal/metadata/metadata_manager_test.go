package metadata

import (
	"reflect"
	"testing"
)

func TestNewMetadataManagerWiresSharedDependencies(t *testing.T) {
	manager := NewMetadataManager()
	if manager == nil {
		t.Fatal("NewMetadataManager() returned nil")
	}
	if manager.tables == nil || manager.views == nil || manager.stats == nil || manager.indexes == nil {
		t.Fatalf("managers = (%v, %v, %v, %v), want all initialized", manager.tables, manager.views, manager.stats, manager.indexes)
	}
	if manager.views.tables != manager.tables {
		t.Fatalf("view table manager = %p, want %p", manager.views.tables, manager.tables)
	}
	if manager.stats.tables != manager.tables {
		t.Fatalf("statistics table manager = %p, want %p", manager.stats.tables, manager.tables)
	}
	if manager.indexes.tables != manager.tables || manager.indexes.stats != manager.stats {
		t.Fatalf("index dependencies = (%p, %p), want (%p, %p)", manager.indexes.tables, manager.indexes.stats, manager.tables, manager.stats)
	}
}

func TestMetadataManagerInitializeCatalogsRegistersCompleteCatalogSet(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()

	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}

	wantFields := map[string][]string{
		"tblcat":  {"tblname", "slotsize"},
		"fldcat":  {"tblname", "fldname", "type", "length", "offset"},
		"viewcat": {"viewname", "viewdef"},
		"idxcat":  {"indexname", "tablename", "fieldname"},
	}
	for tableName, fields := range wantFields {
		layout, err := manager.tables.Layout(tableName, transaction)
		if err != nil {
			t.Fatalf("Layout(%s) error = %v", tableName, err)
		}
		if got := layout.Schema().Fields(); !reflect.DeepEqual(got, fields) {
			t.Fatalf("Layout(%s) fields = %v, want %v", tableName, got, fields)
		}
	}
	if got, want := transaction.appendCalls, []string{"tblcat.tbl", "fldcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("append calls = %v, want %v", got, want)
	}
}
