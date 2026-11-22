package metadata

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type catalogTransaction struct {
	pages        map[storage.BlockID]*storage.Page
	blockSize    int
	pins         []storage.BlockID
	unpins       []storage.BlockID
	appendCalls  []string
	setStringErr error
	unpinErr     error
}

func newCatalogTransaction(t *testing.T, blockSize int) *catalogTransaction {
	t.Helper()
	pages := make(map[storage.BlockID]*storage.Page)
	for _, fileName := range []string{"tblcat.tbl", "fldcat.tbl"} {
		page, err := storage.NewPage(blockSize)
		if err != nil {
			t.Fatalf("NewPage(%d) error = %v", blockSize, err)
		}
		pages[storage.NewBlockID(fileName, 0)] = page
	}
	return &catalogTransaction{pages: pages, blockSize: blockSize}
}

func newEmptyCatalogTransaction(blockSize int) *catalogTransaction {
	return &catalogTransaction{pages: make(map[storage.BlockID]*storage.Page), blockSize: blockSize}
}

func (t *catalogTransaction) Size(fileName string) (int, error) {
	size := 0
	for {
		if _, ok := t.pages[storage.NewBlockID(fileName, size)]; !ok {
			return size, nil
		}
		size++
	}
}

func (t *catalogTransaction) Append(fileName string) (storage.BlockID, error) {
	t.appendCalls = append(t.appendCalls, fileName)
	blockNumber := 0
	for {
		block := storage.NewBlockID(fileName, blockNumber)
		if _, exists := t.pages[block]; !exists {
			page, err := storage.NewPage(t.blockSize)
			if err != nil {
				return storage.BlockID{}, err
			}
			t.pages[block] = page
			return block, nil
		}
		blockNumber++
	}
}

func (t *catalogTransaction) Pin(block storage.BlockID) error {
	if _, ok := t.pages[block]; !ok {
		return fmt.Errorf("unknown block %v", block)
	}
	t.pins = append(t.pins, block)
	return nil
}

func (t *catalogTransaction) Unpin(block storage.BlockID) error {
	t.unpins = append(t.unpins, block)
	return t.unpinErr
}

func (t *catalogTransaction) BlockSize() (int, error) {
	return t.blockSize, nil
}

func (t *catalogTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	return t.pages[block].GetInt(offset)
}

func (t *catalogTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	value, err := t.pages[block].GetBytes(offset)
	return string(value), err
}

func (t *catalogTransaction) SetInt(block storage.BlockID, offset int32, value int32, _ bool) error {
	return t.pages[block].SetInt(int(offset), value)
}

func (t *catalogTransaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	if logUpdate && t.setStringErr != nil {
		return t.setStringErr
	}
	return t.pages[block].SetBytes(int(offset), []byte(value))
}

func TestNewTableManagerRetainsCatalogLayouts(t *testing.T) {
	manager := NewTableManager()
	if manager == nil {
		t.Fatal("NewTableManager() returned nil")
	}

	assertSameLayout(t, manager.tableCatalogLayout, NewTableCatalogLayout())
	assertSameLayout(t, manager.fieldCatalogLayout, NewFieldCatalogLayout())
}

func assertSameLayout(t *testing.T, got, want *record.Layout) {
	t.Helper()
	if got == nil {
		t.Fatal("layout is nil")
	}
	if got.SlotSize() != want.SlotSize() {
		t.Fatalf("slot size = %d, want %d", got.SlotSize(), want.SlotSize())
	}
	if gotFields, wantFields := got.Schema().Fields(), want.Schema().Fields(); !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("fields = %v, want %v", gotFields, wantFields)
	}
	for _, name := range want.Schema().Fields() {
		gotDefinition, gotOK := got.Schema().Field(name)
		wantDefinition, wantOK := want.Schema().Field(name)
		if !gotOK || !wantOK || gotDefinition != wantDefinition {
			t.Fatalf("field %q = (%v, %v), want (%v, %v)", name, gotDefinition, gotOK, wantDefinition, wantOK)
		}
		gotOffset, gotOK := got.Offset(name)
		wantOffset, wantOK := want.Offset(name)
		if !gotOK || !wantOK || gotOffset != wantOffset {
			t.Fatalf("offset %q = (%d, %v), want (%d, %v)", name, gotOffset, gotOK, wantOffset, wantOK)
		}
	}
}

func TestTableManagerCreateTableStoresCatalogRows(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	manager := NewTableManager()
	var schema record.Schema
	schema.AddIntegerField("sid")
	schema.AddVarcharField("sname", 10)

	if err := manager.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}

	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertPageInt(t, tablePage, 0, 1)
	assertPageString(t, tablePage, 4, "student")
	assertPageInt(t, tablePage, 72, int32(record.NewLayout(&schema).SlotSize()))

	fieldPage := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)]
	assertFieldCatalogRow(t, fieldPage, 0, "student", "sid", record.IntegerField, 0, 4)
	assertFieldCatalogRow(t, fieldPage, 152, "student", "sname", record.VarcharField, 10, 8)
	assertPageInt(t, fieldPage, 304, 0)

	wantBlocks := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantBlocks) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantBlocks)
	}
	if !reflect.DeepEqual(transaction.unpins, wantBlocks) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantBlocks)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
}

func TestTableManagerCreateTableRejectsNilSchemaBeforeTransactionAccess(t *testing.T) {
	manager := NewTableManager()
	err := manager.CreateTable("student", nil, nil)
	if err == nil {
		t.Fatal("CreateTable() error = nil, want nil-schema error")
	}
	for _, context := range []string{"student", "schema is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
}

func TestMetadataIntegerPreservesInt32Bounds(t *testing.T) {
	for _, value := range []int{math.MinInt32, math.MaxInt32} {
		got, err := metadataInteger("slot size", value)
		if err != nil || got != int32(value) {
			t.Errorf("metadataInteger(%d) = (%d, %v), want (%d, nil)", value, got, err, int32(value))
		}
	}
}

func TestMetadataIntegerRejectsValuesOutsideInt32(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int cannot represent values outside the int32 range on this platform")
	}

	below := int(int64(math.MinInt32) - 1)
	above := int(int64(math.MaxInt32) + 1)
	for _, value := range []int{below, above} {
		t.Run(strconv.Itoa(value), func(t *testing.T) {
			got, err := metadataInteger("slot size", value)
			if got != 0 || err == nil {
				t.Fatalf("metadataInteger(%d) = (%d, %v), want (0, range error)", value, got, err)
			}
			if want := fmt.Sprintf("slot size %d exceeds int32 range", value); err.Error() != want {
				t.Fatalf("metadataInteger(%d) error = %q, want %q", value, err, want)
			}
		})
	}
}

func TestTableManagerCreateTablePreservesOperationAndCloseFailures(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	transaction.setStringErr = writeErr
	transaction.unpinErr = closeErr
	manager := NewTableManager()
	var schema record.Schema

	err := manager.CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) || !errors.Is(err, closeErr) {
		t.Fatalf("CreateTable() error = %v, want write and close causes", err)
	}
	for _, context := range []string{"student", "table catalog", "store table name", "close catalog scan"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.unpins, []storage.BlockID{storage.NewBlockID("tblcat.tbl", 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unpins = %v, want %v", got, want)
	}
}

func TestTableManagerInitializeCatalogsStoresTheirOwnMetadata(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewTableManager()

	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}

	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertTableCatalogRow(t, tablePage, 0, "tblcat", NewTableCatalogLayout().SlotSize())
	assertTableCatalogRow(t, tablePage, 76, "fldcat", NewFieldCatalogLayout().SlotSize())
	assertPageInt(t, tablePage, 152, 0)

	fieldPage := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)]
	assertFieldCatalogRow(t, fieldPage, 0, "tblcat", "tblname", record.VarcharField, MaxNameLength, 4)
	assertFieldCatalogRow(t, fieldPage, 152, "tblcat", "slotsize", record.IntegerField, 0, 72)
	assertFieldCatalogRow(t, fieldPage, 304, "fldcat", "tblname", record.VarcharField, MaxNameLength, 4)
	assertFieldCatalogRow(t, fieldPage, 456, "fldcat", "fldname", record.VarcharField, MaxNameLength, 72)
	assertFieldCatalogRow(t, fieldPage, 608, "fldcat", "type", record.IntegerField, 0, 140)
	assertFieldCatalogRow(t, fieldPage, 760, "fldcat", "length", record.IntegerField, 0, 144)
	assertFieldCatalogRow(t, fieldPage, 912, "fldcat", "offset", record.IntegerField, 0, 148)
	assertPageInt(t, fieldPage, 1064, 0)

	if got, want := transaction.appendCalls, []string{"tblcat.tbl", "fldcat.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("append calls = %v, want %v", got, want)
	}
}

func TestTableManagerLayoutReconstructsStoredPhysicalMetadata(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewTableManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	transaction.pins = nil
	transaction.unpins = nil

	layout, err := manager.Layout("fldcat", transaction)
	if err != nil {
		t.Fatalf("Layout() error = %v", err)
	}
	assertSameLayout(t, layout, NewFieldCatalogLayout())
	wantBlocks := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantBlocks) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantBlocks)
	}
	if !reflect.DeepEqual(transaction.unpins, wantBlocks) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantBlocks)
	}
}

func TestTableManagerLayoutReportsMissingTableAndClosesTableCatalog(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	manager := NewTableManager()
	if err := manager.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	transaction.pins = nil
	transaction.unpins = nil

	layout, err := manager.Layout("missing", transaction)
	if layout != nil || err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Layout(missing) = (%v, %v), want contextual absence", layout, err)
	}
	wantBlock := storage.NewBlockID("tblcat.tbl", 0)
	if got, want := transaction.pins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pins = %v, want %v", got, want)
	}
	if got, want := transaction.unpins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unpins = %v, want %v", got, want)
	}
}

func assertTableCatalogRow(t *testing.T, page *storage.Page, base int, tableName string, slotSize int) {
	t.Helper()
	assertPageInt(t, page, base, 1)
	assertPageString(t, page, base+4, tableName)
	assertPageInt(t, page, base+72, int32(slotSize))
}

func assertFieldCatalogRow(t *testing.T, page *storage.Page, base int, tableName, fieldName string, fieldType record.FieldType, length, offset int) {
	t.Helper()
	assertPageInt(t, page, base, 1)
	assertPageString(t, page, base+4, tableName)
	assertPageString(t, page, base+72, fieldName)
	assertPageInt(t, page, base+140, int32(fieldType))
	assertPageInt(t, page, base+144, int32(length))
	assertPageInt(t, page, base+148, int32(offset))
}

func assertPageInt(t *testing.T, page *storage.Page, offset int, want int32) {
	t.Helper()
	got, err := page.GetInt(offset)
	if err != nil || got != want {
		t.Fatalf("GetInt(%d) = (%d, %v), want (%d, nil)", offset, got, err, want)
	}
}

func assertPageString(t *testing.T, page *storage.Page, offset int, want string) {
	t.Helper()
	value, err := page.GetBytes(offset)
	got := string(value)
	if err != nil || got != want {
		t.Fatalf("GetBytes(%d) = (%q, %v), want (%q, nil)", offset, got, err, want)
	}
}
