package metadata

import (
	"errors"
	"reflect"
	"strings"
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

func TestViewManagerInitializeCatalogRequiresTableManager(t *testing.T) {
	err := NewViewManager(nil).InitializeCatalog(nil)
	if err == nil || err.Error() != "initialize view catalog: table manager is nil" {
		t.Fatalf("InitializeCatalog() error = %v, want missing table manager context", err)
	}
}

func TestViewManagerViewDefinitionRequiresTableManager(t *testing.T) {
	definition, found, err := NewViewManager(nil).ViewDefinition("seniors", nil)
	if definition != "" || found {
		t.Fatalf("ViewDefinition() = (%q, %v), want empty not-found result", definition, found)
	}
	if err == nil || !strings.Contains(err.Error(), `look up view "seniors"`) || !strings.Contains(err.Error(), "table manager is nil") {
		t.Fatalf("ViewDefinition() error = %v, want view-specific missing table manager context", err)
	}
}

func TestViewManagerInitializeCatalogWrapsCatalogCloseFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	closeErr := errors.New("catalog unpin failed")
	transaction.unpinErr = closeErr

	err := NewViewManager(tables).InitializeCatalog(transaction)
	if !errors.Is(err, closeErr) {
		t.Fatalf("InitializeCatalog() error = %v, want wrapped close cause", err)
	}
	for _, context := range []string{"initialize view catalog", "close catalog scan"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalog() error = %q, want context %q", err, context)
		}
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

func TestViewManagerCreateViewWrapsCatalogLayoutFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	views := NewViewManager(tables)
	if err := views.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := views.CreateView("seniors", "select sid from student", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateView() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{"seniors", "load view catalog layout"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateView() error = %q, want context %q", err, context)
		}
	}
}

func TestViewManagerCreateViewPreservesViewNameWriteFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewViewManager(tables)
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}

	wantErr := errors.New("view name write failed")
	transaction.setStringErr = wantErr
	transaction.setStringErrAfter = transaction.setStringCalls + 1
	transaction.unpins = nil

	err := manager.CreateView("seniors", "select sid from student", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateView() error = %v, want wrapped write cause", err)
	}
	for _, context := range []string{`create view "seniors"`, "store view name"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateView() error = %q, want context %q", err, context)
		}
	}
	viewBlock := storage.NewBlockID("viewcat.tbl", 0)
	if len(transaction.unpins) == 0 || transaction.unpins[len(transaction.unpins)-1] != viewBlock {
		t.Errorf("last catalog scan unpin = %v, want %v", transaction.unpins, viewBlock)
	}
	assertPageString(t, transaction.pages[viewBlock], 4, "")
}

func TestViewManagerCreateViewPreservesViewDefinitionWriteFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewViewManager(tables)
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}

	wantErr := errors.New("view definition write failed")
	transaction.setStringErr = wantErr
	transaction.setStringErrAfter = transaction.setStringCalls + 2
	transaction.unpins = nil

	err := manager.CreateView("seniors", "select sid from student", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateView() error = %v, want wrapped write cause", err)
	}
	for _, context := range []string{`create view "seniors"`, "store view definition"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateView() error = %q, want context %q", err, context)
		}
	}
	viewBlock := storage.NewBlockID("viewcat.tbl", 0)
	if len(transaction.unpins) == 0 || transaction.unpins[len(transaction.unpins)-1] != viewBlock {
		t.Errorf("last catalog scan unpin = %v, want %v", transaction.unpins, viewBlock)
	}
	viewPage := transaction.pages[viewBlock]
	assertPageString(t, viewPage, 4, "seniors")
	assertPageString(t, viewPage, 72, "")
}

func TestViewManagerViewDefinitionFindsMatchAndReportsAbsence(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewViewManager(tables)
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	if err := manager.CreateView("seniors", "select sid from student where gradyear=2020", transaction); err != nil {
		t.Fatalf("CreateView(seniors) error = %v", err)
	}
	if err := manager.CreateView("majors", "select sid from student where majorid=10", transaction); err != nil {
		t.Fatalf("CreateView(majors) error = %v", err)
	}

	definition, found, err := manager.ViewDefinition("majors", transaction)
	if err != nil || !found || definition != "select sid from student where majorid=10" {
		t.Fatalf("ViewDefinition(majors) = (%q, %v, %v), want matching definition", definition, found, err)
	}
	definition, found, err = manager.ViewDefinition("missing", transaction)
	if err != nil || found || definition != "" {
		t.Fatalf("ViewDefinition(missing) = (%q, %v, %v), want (\"\", false, nil)", definition, found, err)
	}
}
