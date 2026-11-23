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
