package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewViewCatalogSchemaDescribesStoredViewDefinitions(t *testing.T) {
	schema := NewViewCatalogSchema()
	if schema == nil {
		t.Fatal("NewViewCatalogSchema() returned nil")
	}
	if got, want := schema.Fields(), []string{"viewname", "viewdef"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for name, wantLength := range map[string]int{
		"viewname": MaxNameLength,
		"viewdef":  MaxViewDefinitionLength,
	} {
		if got, ok := schema.TypeOf(name); !ok || got != record.VarcharField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.VarcharField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != wantLength {
			t.Fatalf("%s length = (%d, %v), want (%d, true)", name, got, ok, wantLength)
		}
	}
}

func TestNewViewManagerRetainsTableManager(t *testing.T) {
	tables := NewTableManager()
	manager := NewViewManager(tables)
	if manager == nil {
		t.Fatal("NewViewManager() returned nil")
	}
	if manager.tables != tables {
		t.Fatalf("table manager = %p, want %p", manager.tables, tables)
	}
}

func TestViewManagerInitializeCatalogRegistersViewMetadata(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	transaction.appendCalls = nil
	manager := NewViewManager(tables)

	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}

	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertTableCatalogRow(t, tablePage, 152, "viewcat", record.NewLayout(NewViewCatalogSchema()).SlotSize())
	fieldPage := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)]
	assertFieldCatalogRow(t, fieldPage, 1064, "viewcat", "viewname", record.VarcharField, MaxNameLength, 4)
	assertFieldCatalogRow(t, fieldPage, 1216, "viewcat", "viewdef", record.VarcharField, MaxViewDefinitionLength, 72)
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none for catalog metadata registration", transaction.appendCalls)
	}
}

func TestViewManagerCreateViewStoresDefinitionInNewViewFile(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewViewManager(tables)
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	transaction.appendCalls = nil

	if err := manager.CreateView("goodstudents", "select sname from student where gradyear=2020", transaction); err != nil {
		t.Fatalf("CreateView() error = %v", err)
	}

	viewPage := transaction.pages[storage.NewBlockID("viewcat.tbl", 0)]
	assertPageInt(t, viewPage, 0, 1)
	assertPageString(t, viewPage, 4, "goodstudents")
	assertPageString(t, viewPage, 72, "select sname from student where gradyear=2020")
	if got, want := transaction.appendCalls, []string{"viewcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("append calls = %v, want %v", got, want)
	}
}
