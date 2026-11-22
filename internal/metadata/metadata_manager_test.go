package metadata

import (
	"errors"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"reflect"
	"strings"
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

func TestMetadataManagerInitializeCatalogsPreservesTableCatalogFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := NewMetadataManager().InitializeCatalogs(transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("InitializeCatalogs() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{"initialize metadata catalogs", "table catalogs"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalogs() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("catalog append calls = %v, want none before table-catalog initialization failure", transaction.appendCalls)
	}
}

func TestMetadataManagerInitializeCatalogsPreservesViewCatalogFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	wantErr := errors.New("view catalog table size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"
	transaction.sizeErrAfter = 3

	err := NewMetadataManager().InitializeCatalogs(transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("InitializeCatalogs() error = %v, want wrapped view-catalog cause", err)
	}
	for _, context := range []string{"initialize metadata catalogs", "view catalog", "initialize view catalog"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalogs() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.appendCalls, []string{"tblcat.tbl", "fldcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog append calls = %v, want only the table and field catalog bootstrap", got)
	}
	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertPageInt(t, tablePage, 0, 1)
	assertPageString(t, tablePage, 4, "tblcat")
	assertPageInt(t, tablePage, 76, 1)
	assertPageString(t, tablePage, 80, "fldcat")
	assertPageInt(t, tablePage, 152, 0)
}

func TestMetadataManagerInitializeCatalogsPreservesIndexCatalogFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	wantErr := errors.New("index catalog table size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"
	transaction.sizeErrAfter = 4

	err := NewMetadataManager().InitializeCatalogs(transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("InitializeCatalogs() error = %v, want wrapped index-catalog cause", err)
	}
	for _, context := range []string{"initialize metadata catalogs", "index catalog", "initialize index catalog"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalogs() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.appendCalls, []string{"tblcat.tbl", "fldcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog append calls = %v, want only the table and field catalog bootstrap", got)
	}
	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertPageInt(t, tablePage, 0, 1)
	assertPageString(t, tablePage, 4, "tblcat")
	assertPageInt(t, tablePage, 76, 1)
	assertPageString(t, tablePage, 80, "fldcat")
	assertPageInt(t, tablePage, 152, 1)
	assertPageString(t, tablePage, 156, "viewcat")
	assertPageInt(t, tablePage, 228, 0)
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

func TestMetadataManagerCreateTablePreservesNilSchemaError(t *testing.T) {
	err := NewMetadataManager().CreateTable("student", nil, nil)
	if err == nil {
		t.Fatal("CreateTable() error = nil, want nil-schema validation error")
	}
	for _, context := range []string{`create table "student" through metadata manager`, "schema is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
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

func TestMetadataManagerLayoutPreservesCatalogReadError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	layout, err := manager.Layout("student", transaction)
	if layout != nil {
		t.Fatalf("Layout() layout = %v, want nil on catalog failure", layout)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Layout() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{`look up layout for table "student" through metadata manager`, "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("Layout() error = %q, want context %q", err, context)
		}
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

func TestMetadataManagerCreateViewPreservesLayoutError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := manager.CreateView("seniors", "select sid from student", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateView() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{`create view "seniors" through metadata manager`, "load view catalog layout"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateView() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.appendCalls) != 2 {
		t.Fatalf("catalog append calls = %v, want only initialized table and field catalogs", transaction.appendCalls)
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

func TestMetadataManagerViewDefinitionPreservesCatalogReadError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	definition, found, err := manager.ViewDefinition("seniors", transaction)
	if definition != "" || found {
		t.Fatalf("ViewDefinition() = (%q, %v), want empty and not found on catalog failure", definition, found)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("ViewDefinition() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{`look up view "seniors" through metadata manager`, "load view catalog layout"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("ViewDefinition() error = %q, want context %q", err, context)
		}
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

func TestMetadataManagerCreateIndexPreservesMissingTableManagerError(t *testing.T) {
	manager := NewMetadataManager()
	manager.indexes.tables = nil

	err := manager.CreateIndex("idx_student_major", "student", "majorid", nil)
	if err == nil {
		t.Fatal("CreateIndex() error = nil, want missing table-manager error")
	}
	for _, context := range []string{`create index "idx_student_major" through metadata manager`, "table manager is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateIndex() error = %q, want context %q", err, context)
		}
	}
}

func TestMetadataManagerIndexInfoLooksUpRegisteredAndUnindexedTables(t *testing.T) {
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

	indexes, err := manager.IndexInfo("student", transaction)
	if err != nil {
		t.Fatalf("IndexInfo(student) error = %v", err)
	}
	info := indexes["majorid"]
	if len(indexes) != 1 || info == nil || info.indexName != "idx_student_major" {
		t.Fatalf("student indexes = %v, want idx_student_major on majorid", indexes)
	}
	missing, err := manager.IndexInfo("course", transaction)
	if err != nil || missing == nil || len(missing) != 0 {
		t.Fatalf("IndexInfo(course) = (%v, %v), want initialized empty map", missing, err)
	}
}

func TestMetadataManagerIndexInfoPreservesCatalogReadError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	wantErr := errors.New("index catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	indexes, err := manager.IndexInfo("student", transaction)
	if indexes != nil {
		t.Fatalf("IndexInfo() indexes = %v, want nil on catalog failure", indexes)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("IndexInfo() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{`look up indexes for table "student" through metadata manager`, `look up indexes for table "student"`, "load index catalog layout", "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("IndexInfo() error = %q, want context %q", err, context)
		}
	}
}

func TestMetadataManagerStatisticsReturnsSharedManagerSnapshot(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewMetadataManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("sid")
	if err := manager.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	for sid := int32(1); sid <= 2; sid++ {
		if err := scan.Insert(); err != nil {
			t.Fatalf("Insert(%d) error = %v", sid, err)
		}
		if err := scan.SetInt("sid", sid); err != nil {
			t.Fatalf("SetInt(%d) error = %v", sid, err)
		}
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	stats, err := manager.Statistics("student", layout, transaction)
	if err != nil {
		t.Fatalf("Statistics() error = %v", err)
	}
	if stats.RecordsOutput() != 2 || manager.stats.stats["student"] != stats {
		t.Fatalf("statistics = %p (%d records), want shared cached two-record snapshot", stats, stats.RecordsOutput())
	}
}

func TestMetadataManagerStatisticsPreservesRefreshFailure(t *testing.T) {
	statsManager := NewStatManager(nil)
	statsManager.calls = 100
	manager := &MetadataManager{stats: statsManager}

	stats, err := manager.Statistics("student", nil, nil)
	if stats != nil {
		t.Errorf("Statistics() stats = %v, want nil on refresh failure", stats)
	}
	if err == nil {
		t.Fatal("Statistics() error = nil, want wrapped refresh failure")
	}
	for _, context := range []string{
		`look up statistics for table "student" through metadata manager`,
		"refresh table statistics",
		"table manager is nil",
	} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Statistics() error = %q, want context %q", err, context)
		}
	}
	if statsManager.calls != 101 {
		t.Errorf("statistics lookup calls = %d, want 101", statsManager.calls)
	}
	if len(statsManager.stats) != 0 {
		t.Errorf("statistics cache = %v, want empty cache after refresh failure", statsManager.stats)
	}
}
