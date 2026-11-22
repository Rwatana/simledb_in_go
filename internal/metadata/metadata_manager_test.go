package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
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

func TestMetadataManagerCreateTableStoresSchemaThroughFacade(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("sid")
	schema.AddVarcharField("sname", 20)

	if err := manager.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}

	layout, err := manager.tables.Layout("student", transaction)
	if err != nil {
		t.Fatalf("table manager Layout(student) error = %v", err)
	}
	if got, want := layout.Schema().Fields(), []string{"sid", "sname"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("student fields = %v, want %v", got, want)
	}
}

func TestMetadataManagerLayoutReconstructsTableThroughFacade(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("sid")
	schema.AddVarcharField("sname", 20)
	if err := manager.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}

	layout, err := manager.Layout("student", transaction)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	if got, want := layout.Schema().Fields(), []string{"sid", "sname"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	if got, want := layout.SlotSize(), record.NewLayout(&schema).SlotSize(); got != want {
		t.Fatalf("slot size = %d, want %d", got, want)
	}
}

func TestMetadataManagerCreateViewStoresDefinitionThroughFacade(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}

	if err := manager.CreateView("seniors", "select sid from student where gradyear=2020", transaction); err != nil {
		t.Fatalf("CreateView() error = %v", err)
	}

	definition, found, err := manager.views.ViewDefinition("seniors", transaction)
	if err != nil || !found || definition != "select sid from student where gradyear=2020" {
		t.Fatalf("view manager lookup = (%q, %v, %v), want stored definition", definition, found, err)
	}
}

func TestMetadataManagerViewDefinitionLooksUpStoredAndMissingViews(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	if err := manager.CreateView("seniors", "select sid from student where gradyear=2020", transaction); err != nil {
		t.Fatalf("CreateView() error = %v", err)
	}

	definition, found, err := manager.ViewDefinition("seniors", transaction)
	if err != nil || !found || definition != "select sid from student where gradyear=2020" {
		t.Fatalf("ViewDefinition(seniors) = (%q, %v, %v), want stored definition", definition, found, err)
	}
	definition, found, err = manager.ViewDefinition("missing", transaction)
	if err != nil || found || definition != "" {
		t.Fatalf("ViewDefinition(missing) = (%q, %v, %v), want (\"\", false, nil)", definition, found, err)
	}
}

func TestMetadataManagerCreateIndexStoresRegistrationThroughFacade(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("sid")
	schema.AddIntegerField("majorid")
	if err := manager.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}

	if err := manager.CreateIndex("idx_student_major", "student", "majorid", transaction); err != nil {
		t.Fatalf("CreateIndex() error = %v", err)
	}

	indexes, err := manager.indexes.IndexInfo("student", transaction)
	if err != nil {
		t.Fatalf("index manager IndexInfo(student) error = %v", err)
	}
	info := indexes["majorid"]
	if len(indexes) != 1 || info == nil || info.indexName != "idx_student_major" {
		t.Fatalf("student indexes = %v, want idx_student_major on majorid", indexes)
	}
}
