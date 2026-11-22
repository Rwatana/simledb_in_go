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
	pages             map[storage.BlockID]*storage.Page
	blockSize         int
	pins              []storage.BlockID
	unpins            []storage.BlockID
	appendCalls       []string
	sizeErr           error
	sizeErrFile       string
	sizeErrAfter      int
	sizeErrCalls      int
	setStringErr      error
	setStringErrAfter int
	setStringCalls    int
	setIntErr         error
	setIntOffset      int32
	setIntErrAfter    int
	setIntOffsetCalls int
	setIntCalls       int
	getIntErr         error
	getIntOffset      int
	getStringErr      error
	getStringOffset   int
	unpinErr          error
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
	if t.sizeErr != nil && (t.sizeErrFile == "" || t.sizeErrFile == fileName) {
		t.sizeErrCalls++
		if t.sizeErrAfter == 0 || t.sizeErrCalls == t.sizeErrAfter {
			return 0, t.sizeErr
		}
	}
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
	if t.getIntErr != nil && offset == t.getIntOffset {
		return 0, t.getIntErr
	}
	return t.pages[block].GetInt(offset)
}

func (t *catalogTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	if t.getStringErr != nil && offset == t.getStringOffset {
		return "", t.getStringErr
	}
	value, err := t.pages[block].GetBytes(offset)
	return string(value), err
}

func (t *catalogTransaction) SetInt(block storage.BlockID, offset int32, value int32, _ bool) error {
	t.setIntCalls++
	if t.setIntErr != nil && offset == t.setIntOffset {
		t.setIntOffsetCalls++
		if t.setIntErrAfter == 0 || t.setIntOffsetCalls == t.setIntErrAfter {
			return t.setIntErr
		}
	}
	return t.pages[block].SetInt(int(offset), value)
}

func (t *catalogTransaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	if logUpdate {
		t.setStringCalls++
		if t.setStringErr != nil && (t.setStringErrAfter == 0 || t.setStringCalls == t.setStringErrAfter) {
			return t.setStringErr
		}
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

func TestTableManagerCreateTableRejectsUnencodableSlotSizeBeforeTransactionAccess(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int cannot represent a schema whose slot size exceeds int32 on this platform")
	}
	var schema record.Schema
	schema.AddVarcharField("first", math.MaxInt32/2)
	schema.AddVarcharField("second", math.MaxInt32/2)

	err := NewTableManager().CreateTable("oversized", &schema, nil)
	if err == nil {
		t.Fatal("CreateTable() error = nil, want slot-size range error")
	}
	for _, context := range []string{"oversized", "slot size", "exceeds int32 range"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
}

func TestTableManagerCreateTableRejectsOutOfRangeFieldLengthBeforeTransactionAccess(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int cannot represent field lengths above int32 on this platform")
	}
	fieldLength := int64(math.MaxInt32) + 1
	var schema record.Schema
	schema.AddVarcharField("oversized", int(fieldLength))

	err := NewTableManager().CreateTable("oversized_field", &schema, nil)
	if err == nil {
		t.Fatal("CreateTable() error = nil, want field-length range error")
	}
	for _, context := range []string{"oversized_field", "oversized", "field length", "exceeds int32 range"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
}

func TestTableManagerCreateTableRejectsInvalidFieldMetadataBeforeCatalogWrites(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	var schema record.Schema
	schema.AddVarcharField("badname", -1)

	err := NewTableManager().CreateTable("invalid_table", &schema, transaction)
	if err == nil {
		t.Fatal("CreateTable() error = nil, want invalid field-length error")
	}
	for _, context := range []string{"invalid_table", "badname", "field length"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("pins/unpins = %v/%v, want no catalog scans", transaction.pins, transaction.unpins)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
	assertPageInt(t, transaction.pages[storage.NewBlockID("tblcat.tbl", 0)], 0, 0)
	assertPageInt(t, transaction.pages[storage.NewBlockID("fldcat.tbl", 0)], 0, 0)
}

func TestTableManagerCreateTableReportsCloseFailureAfterSuccessfulTableWrite(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	closeErr := errors.New("catalog unpin failed")
	transaction.unpinErr = closeErr
	var schema record.Schema

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, closeErr) {
		t.Fatalf("CreateTable() error = %v, want close cause", err)
	}
	for _, context := range []string{"student", "table catalog", "close catalog scan"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.unpins, []storage.BlockID{storage.NewBlockID("tblcat.tbl", 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unpins = %v, want %v", got, want)
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != storage.NewBlockID("tblcat.tbl", 0) {
		t.Fatalf("pins = %v, want only the table catalog", transaction.pins)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
}

func TestTableManagerCreateTablePreservesCatalogScanOpenFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	sizeErr := errors.New("catalog size failed")
	transaction.sizeErr = sizeErr
	var schema record.Schema

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, sizeErr) {
		t.Fatalf("CreateTable() error = %v, want size cause", err)
	}
	for _, context := range []string{"student", "table catalog", "open catalog scan", "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("pins/unpins = %v/%v, want none for an unopened scan", transaction.pins, transaction.unpins)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
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

func TestTableManagerCreateTablePreservesTableCatalogInsertFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	insertErr := errors.New("table catalog row insert failed")
	transaction.setIntErr = insertErr
	transaction.setIntOffset = 0
	var schema record.Schema

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, insertErr) {
		t.Fatalf("CreateTable() error = %v, want table catalog insert cause", err)
	}
	for _, context := range []string{"student", "table catalog", "insert table catalog row"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.pins, []storage.BlockID{storage.NewBlockID("tblcat.tbl", 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pins = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(transaction.unpins, transaction.pins) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, transaction.pins)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogInsertFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	insertErr := errors.New("field catalog row insert failed")
	transaction.setIntErr = insertErr
	transaction.setIntOffset = 0
	transaction.setIntErrAfter = 2
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, insertErr) {
		t.Fatalf("CreateTable() error = %v, want field catalog insert cause", err)
	}
	for _, context := range []string{"student", "field catalog", `insert field catalog row for "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
	if got, err := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)].GetInt(0); err != nil || got != 0 {
		t.Fatalf("fldcat occupancy = (%d, %v), want (0, nil)", got, err)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogTableNameWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("field catalog table-name write failed")
	transaction.setStringErr = writeErr
	transaction.setStringErrAfter = 2
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want fldcat table-name write cause", err)
	}
	for _, context := range []string{"student", "field catalog", `store table name for field "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if transaction.setStringCalls != 2 {
		t.Fatalf("string writes = %d, want stop at the field table-name write", transaction.setStringCalls)
	}
	if got, err := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)].GetInt(0); err != nil || got != 1 {
		t.Fatalf("fldcat occupancy = (%d, %v), want the inserted row marked used", got, err)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogOffsetWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("field catalog offset write failed")
	manager := NewTableManager()
	offset, _ := manager.fieldCatalogLayout.Offset("offset")
	transaction.setIntErr = writeErr
	transaction.setIntOffset = int32(offset)
	transaction.setIntErrAfter = 1
	var schema record.Schema
	schema.AddIntegerField("sid")
	schema.AddIntegerField("name")

	err := manager.CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want fldcat offset write cause", err)
	}
	for _, context := range []string{"student", "field catalog", `store field offset for "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if transaction.setIntCalls != 6 {
		t.Fatalf("integer writes = %d, want stop at the first field-offset write", transaction.setIntCalls)
	}
	if transaction.setStringCalls != 3 {
		t.Fatalf("string writes = %d, want table name and first field metadata only", transaction.setStringCalls)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogLengthWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("field catalog length write failed")
	manager := NewTableManager()
	lengthOffset, _ := manager.fieldCatalogLayout.Offset("length")
	transaction.setIntErr = writeErr
	transaction.setIntOffset = int32(lengthOffset)
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := manager.CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want fldcat length write cause", err)
	}
	for _, context := range []string{"student", "field catalog", `store field length for "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if transaction.setIntCalls != 5 {
		t.Fatalf("integer writes = %d, want stop at the field-length write", transaction.setIntCalls)
	}
	if transaction.setStringCalls != 3 {
		t.Fatalf("string writes = %d, want table name, field table name, and field name", transaction.setStringCalls)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogTypeWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("field catalog type write failed")
	manager := NewTableManager()
	typeOffset, _ := manager.fieldCatalogLayout.Offset("type")
	transaction.setIntErr = writeErr
	transaction.setIntOffset = int32(typeOffset)
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := manager.CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want fldcat type write cause", err)
	}
	for _, context := range []string{"student", "field catalog", `store field type for "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if transaction.setIntCalls != 4 {
		t.Fatalf("integer writes = %d, want stop at the field-type write", transaction.setIntCalls)
	}
	if transaction.setStringCalls != 3 {
		t.Fatalf("string writes = %d, want table name, field table name, and field name", transaction.setStringCalls)
	}
}

func TestTableManagerCreateTablePreservesFieldCatalogFieldNameWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("field catalog field-name write failed")
	transaction.setStringErr = writeErr
	transaction.setStringErrAfter = 3
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want fldcat field-name write cause", err)
	}
	for _, context := range []string{"student", "field catalog", `store field name "sid"`} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	wantScans := []storage.BlockID{
		storage.NewBlockID("tblcat.tbl", 0),
		storage.NewBlockID("fldcat.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.pins, wantScans) {
		t.Fatalf("pins = %v, want %v", transaction.pins, wantScans)
	}
	if !reflect.DeepEqual(transaction.unpins, wantScans) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, wantScans)
	}
	if transaction.setStringCalls != 3 {
		t.Fatalf("string writes = %d, want stop at the field name write", transaction.setStringCalls)
	}
	if got, err := transaction.pages[storage.NewBlockID("fldcat.tbl", 0)].GetInt(0); err != nil || got != 1 {
		t.Fatalf("fldcat occupancy = (%d, %v), want the inserted row marked used", got, err)
	}
}

func TestTableManagerCreateTablePreservesTableSlotSizeWriteFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	writeErr := errors.New("integer metadata write failed")
	transaction.setIntErr = writeErr
	transaction.setIntOffset = 72
	var schema record.Schema
	schema.AddIntegerField("sid")

	err := NewTableManager().CreateTable("student", &schema, transaction)
	if !errors.Is(err, writeErr) {
		t.Fatalf("CreateTable() error = %v, want slot-size write cause", err)
	}
	for _, context := range []string{"student", "table catalog", "store table slot size"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("CreateTable() error = %q, want context %q", err, context)
		}
	}
	if got, want := transaction.pins, []storage.BlockID{storage.NewBlockID("tblcat.tbl", 0)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pins = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(transaction.unpins, transaction.pins) {
		t.Fatalf("unpins = %v, want %v", transaction.unpins, transaction.pins)
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

func TestTableManagerInitializeCatalogsPreservesTableCatalogFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	sizeErr := errors.New("table catalog size failed")
	transaction.sizeErr = sizeErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := NewTableManager().InitializeCatalogs(transaction)
	if !errors.Is(err, sizeErr) {
		t.Fatalf("InitializeCatalogs() error = %v, want size cause", err)
	}
	for _, context := range []string{"initialize table catalog", "open catalog scan", "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalogs() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.appendCalls) != 0 || len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("catalog side effects = append %v, pins %v, unpins %v; want none", transaction.appendCalls, transaction.pins, transaction.unpins)
	}
}

func TestTableManagerInitializeCatalogsPreservesFieldCatalogFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	sizeErr := errors.New("field catalog registration size failed")
	transaction.sizeErr = sizeErr
	transaction.sizeErrFile = "tblcat.tbl"
	transaction.sizeErrAfter = 2

	err := NewTableManager().InitializeCatalogs(transaction)
	if !errors.Is(err, sizeErr) {
		t.Fatalf("InitializeCatalogs() error = %v, want size cause", err)
	}
	for _, context := range []string{"initialize field catalog", "fldcat", "table catalog", "open catalog scan", "read size"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("InitializeCatalogs() error = %q, want context %q", err, context)
		}
	}

	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	assertPageString(t, tablePage, 4, "tblcat")
	assertPageInt(t, tablePage, 76, 0)
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

func TestTableManagerLayoutPreservesTableCatalogScanFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	scanErr := errors.New("table catalog occupancy read failed")
	transaction.getIntErr = scanErr
	transaction.getIntOffset = 0

	layout, err := NewTableManager().Layout("student", transaction)
	if layout != nil || !errors.Is(err, scanErr) {
		t.Fatalf("Layout() = (%v, %v), want table-catalog scan cause", layout, err)
	}
	for _, context := range []string{"student", "table catalog", "scan table catalog"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("Layout() error = %q, want context %q", err, context)
		}
	}
	wantBlock := storage.NewBlockID("tblcat.tbl", 0)
	if got, want := transaction.pins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pins = %v, want %v", got, want)
	}
	if got, want := transaction.unpins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unpins = %v, want %v", got, want)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
	}
}

func TestTableManagerLayoutPreservesRegisteredTableNameReadFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 512)
	readErr := errors.New("registered table name read failed")
	transaction.getStringErr = readErr
	transaction.getStringOffset = 4
	tablePage := transaction.pages[storage.NewBlockID("tblcat.tbl", 0)]
	if err := tablePage.SetInt(0, 1); err != nil {
		t.Fatalf("mark tblcat row occupied: %v", err)
	}
	if err := tablePage.SetBytes(4, []byte("student")); err != nil {
		t.Fatalf("store tblcat table name: %v", err)
	}

	layout, err := NewTableManager().Layout("student", transaction)
	if layout != nil || !errors.Is(err, readErr) {
		t.Fatalf("Layout() = (%v, %v), want registered-name read cause", layout, err)
	}
	for _, context := range []string{"student", "table catalog", "read registered table name"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("Layout() error = %q, want context %q", err, context)
		}
	}
	wantBlock := storage.NewBlockID("tblcat.tbl", 0)
	if got, want := transaction.pins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pins = %v, want %v", got, want)
	}
	if got, want := transaction.unpins, []storage.BlockID{wantBlock}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unpins = %v, want %v", got, want)
	}
	if len(transaction.appendCalls) != 0 {
		t.Fatalf("append calls = %v, want none", transaction.appendCalls)
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
