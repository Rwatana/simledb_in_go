package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewIndexCatalogSchemaDescribesStoredIndexRegistrations(t *testing.T) {
	schema := NewIndexCatalogSchema()
	if schema == nil {
		t.Fatal("NewIndexCatalogSchema() returned nil")
	}
	if got, want := schema.Fields(), []string{"indexname", "tablename", "fieldname"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for _, name := range schema.Fields() {
		if got, ok := schema.TypeOf(name); !ok || got != record.VarcharField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.VarcharField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != MaxNameLength {
			t.Fatalf("%s length = (%d, %v), want (%d, true)", name, got, ok, MaxNameLength)
		}
	}
}

func TestNewIndexManagerRetainsMetadataDependencies(t *testing.T) {
	tables := NewTableManager()
	stats := NewStatManager(tables)
	manager := NewIndexManager(tables, stats)
	if manager == nil {
		t.Fatal("NewIndexManager() returned nil")
	}
	if manager.tables != tables {
		t.Fatalf("table manager = %p, want %p", manager.tables, tables)
	}
	if manager.stats != stats {
		t.Fatalf("statistics manager = %p, want %p", manager.stats, stats)
	}
}

func TestIndexManagerInitializeCatalogRegistersIndexMetadata(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	transaction.appendCalls = nil
	manager := NewIndexManager(tables, NewStatManager(tables))

	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}

	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertTableCatalogRow(t, tablePage, 152, "idxcat", record.NewLayout(NewIndexCatalogSchema()).SlotSize())
	fieldPage := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)]
	assertFieldCatalogRow(t, fieldPage, 1064, "idxcat", "indexname", record.VarcharField, MaxNameLength, 4)
	assertFieldCatalogRow(t, fieldPage, 1216, "idxcat", "tablename", record.VarcharField, MaxNameLength, 72)
	assertFieldCatalogRow(t, fieldPage, 1368, "idxcat", "fieldname", record.VarcharField, MaxNameLength, 140)
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none for catalog metadata registration", transaction.appendCalls)
	}
}

func TestIndexManagerInitializeCatalogRejectsMissingTableManager(t *testing.T) {
	manager := NewIndexManager(nil, nil)
	if err := manager.InitializeCatalog(newEmptyCatalogTransaction(2048)); err == nil {
		t.Fatal("InitializeCatalog() error = nil, want missing table manager")
	}
}

func TestIndexManagerCreateIndexStoresRegistrationInNewIndexCatalogFile(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewIndexManager(tables, NewStatManager(tables))
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	transaction.appendCalls = nil

	if err := manager.CreateIndex("idx_student_major", "student", "majorid", transaction); err != nil {
		t.Fatalf("CreateIndex() error = %v", err)
	}

	indexPage := transaction.pages[storage.NewBlockID("idxcat.tbl", 0)]
	assertPageInt(t, indexPage, 0, 1)
	assertPageString(t, indexPage, 4, "idx_student_major")
	assertPageString(t, indexPage, 72, "student")
	assertPageString(t, indexPage, 140, "majorid")
	if got, want := transaction.appendCalls, []string{"idxcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("append calls = %v, want %v", got, want)
	}
}
