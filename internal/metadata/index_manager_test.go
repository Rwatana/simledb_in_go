package metadata

import (
	"errors"
	"reflect"
	"strings"
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

func TestIndexManagerIndexInfoRejectsMissingStatisticsManager(t *testing.T) {
	manager := NewIndexManager(NewTableManager(), nil)

	indexes, err := manager.IndexInfo("student", nil)
	if indexes != nil {
		t.Errorf("IndexInfo() indexes = %v, want nil when statistics manager is missing", indexes)
	}
	if err == nil {
		t.Fatal("IndexInfo() error = nil, want missing statistics manager error")
	}
	for _, context := range []string{"student", "statistics manager is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("IndexInfo() error = %q, want context %q", err, context)
		}
	}
}

func TestIndexManagerCreateIndexRejectsMissingTableManager(t *testing.T) {
	manager := NewIndexManager(nil, nil)

	err := manager.CreateIndex("idx_student_sid", "student", "sid", nil)
	if err == nil {
		t.Fatal("CreateIndex() error = nil, want missing table manager error")
	}
	for _, context := range []string{"idx_student_sid", "table manager is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateIndex() error = %q, want context %q", err, context)
		}
	}
}

func TestIndexManagerIndexInfoRejectsMissingTableManager(t *testing.T) {
	manager := NewIndexManager(nil, NewStatManager(nil))

	indexes, err := manager.IndexInfo("student", nil)
	if indexes != nil {
		t.Errorf("IndexInfo() indexes = %v, want nil when table manager is missing", indexes)
	}
	if err == nil {
		t.Fatal("IndexInfo() error = nil, want missing table manager error")
	}
	for _, context := range []string{"student", "table manager is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("IndexInfo() error = %q, want context %q", err, context)
		}
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

func TestIndexManagerCreateIndexPreservesCatalogLayoutError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewIndexManager(tables, NewStatManager(tables))
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	appendCallsBefore := append([]string(nil), transaction.appendCalls...)
	wantErr := errors.New("index catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := manager.CreateIndex("idx_student_major", "student", "majorid", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateIndex() error = %v, want wrapped size cause", err)
	}
	for _, context := range []string{"idx_student_major", "load index catalog layout", "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateIndex() error = %q, want context %q", err, context)
		}
	}
	if !reflect.DeepEqual(transaction.appendCalls, appendCallsBefore) {
		t.Fatalf("append calls = %v, want unchanged from %v", transaction.appendCalls, appendCallsBefore)
	}
}

func TestIndexManagerCreateIndexPreservesIndexNameWriteError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewIndexManager(tables, NewStatManager(tables))
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	wantErr := errors.New("index name write unavailable")
	transaction.setStringCalls = 0
	transaction.setStringErr = wantErr
	transaction.setStringErrAfter = 1

	err := manager.CreateIndex("idx_student_major", "student", "majorid", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateIndex() error = %v, want wrapped index-name write cause", err)
	}
	for _, context := range []string{"idx_student_major", "store index name", `field "indexname"`} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateIndex() error = %q, want context %q", err, context)
		}
	}
}

func TestIndexManagerCreateIndexPreservesTableNameWriteError(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	manager := NewIndexManager(tables, NewStatManager(tables))
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	wantErr := errors.New("indexed table name write unavailable")
	transaction.setStringCalls = 0
	transaction.setStringErr = wantErr
	transaction.setStringErrAfter = 2

	err := manager.CreateIndex("idx_student_major", "student", "majorid", transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateIndex() error = %v, want wrapped table-name write cause", err)
	}
	for _, context := range []string{"idx_student_major", "store indexed table name", `field "tablename"`} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("CreateIndex() error = %q, want context %q", err, context)
		}
	}
}

func TestIndexManagerIndexInfoReturnsRegistrationsForRequestedTable(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var studentSchema record.Schema
	studentSchema.AddIntegerField("sid")
	studentSchema.AddIntegerField("majorid")
	if err := tables.CreateTable("student", &studentSchema, transaction); err != nil {
		t.Fatalf("CreateTable(student) error = %v", err)
	}
	stats := NewStatManager(tables)
	manager := NewIndexManager(tables, stats)
	if err := manager.InitializeCatalog(transaction); err != nil {
		t.Fatalf("InitializeCatalog() error = %v", err)
	}
	for _, registration := range [][3]string{
		{"idx_student_id", "student", "sid"},
		{"idx_department_name", "department", "dname"},
		{"idx_student_major", "student", "majorid"},
	} {
		if err := manager.CreateIndex(registration[0], registration[1], registration[2], transaction); err != nil {
			t.Fatalf("CreateIndex(%s) error = %v", registration[0], err)
		}
	}

	indexes, err := manager.IndexInfo("student", transaction)
	if err != nil {
		t.Fatalf("IndexInfo(student) error = %v", err)
	}
	if len(indexes) != 2 {
		t.Fatalf("student indexes = %v, want two entries", indexes)
	}
	for fieldName, indexName := range map[string]string{
		"sid":     "idx_student_id",
		"majorid": "idx_student_major",
	} {
		info := indexes[fieldName]
		if info == nil || info.indexName != indexName || info.fieldName != fieldName {
			t.Fatalf("index %s = %#v, want %s on %s", fieldName, info, indexName, fieldName)
		}
		if info.transaction != transaction || info.stats == nil {
			t.Fatalf("index %s dependencies = (%v, %v), want transaction and statistics", fieldName, info.transaction, info.stats)
		}
	}

	missing, err := manager.IndexInfo("course", transaction)
	if err != nil || len(missing) != 0 {
		t.Fatalf("IndexInfo(course) = (%v, %v), want empty map and nil error", missing, err)
	}
}
