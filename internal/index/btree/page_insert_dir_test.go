package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageInsertDirWrite struct {
	block     storage.BlockID
	offset    int32
	intValue  int32
	textValue string
	isString  bool
	logged    bool
}

type pageInsertDirTransaction struct {
	ints         map[int]int32
	strings      map[int]string
	intReads     []int
	stringReads  []int
	writes       []pageInsertDirWrite
	readErrors   map[int]error
	stringErrors map[int]error
	writeErrorAt int
	writeError   error
	pinCount     int
	unpinCount   int
}

func (tx *pageInsertDirTransaction) Pin(storage.BlockID) error {
	tx.pinCount++
	return nil
}

func (tx *pageInsertDirTransaction) Unpin(storage.BlockID) error {
	tx.unpinCount++
	return nil
}

func (*pageInsertDirTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageInsertDirTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	tx.intReads = append(tx.intReads, offset)
	if err := tx.readErrors[offset]; err != nil {
		return 0, err
	}
	return tx.ints[offset], nil
}

func (tx *pageInsertDirTransaction) GetString(_ storage.BlockID, offset int) (string, error) {
	tx.stringReads = append(tx.stringReads, offset)
	if err := tx.stringErrors[offset]; err != nil {
		return "", err
	}
	return tx.strings[offset], nil
}

func (tx *pageInsertDirTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	tx.writes = append(tx.writes, pageInsertDirWrite{block: block, offset: offset, intValue: value, logged: logged})
	if tx.writeErrorAt == len(tx.writes) {
		return tx.writeError
	}
	tx.ints[int(offset)] = value
	return nil
}

func (tx *pageInsertDirTransaction) SetString(block storage.BlockID, offset int32, value string, logged bool) error {
	tx.writes = append(tx.writes, pageInsertDirWrite{block: block, offset: offset, textValue: value, isString: true, logged: logged})
	if tx.writeErrorAt == len(tx.writes) {
		return tx.writeError
	}
	tx.strings[int(offset)] = value
	return nil
}

func TestPageInsertDirShiftsCompleteRecordsBeforeWritingIntegerEntry(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 6)
	layout := pageInsertDirLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(layout, 3)
	for slot, entry := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}} {
		transaction.ints[pageInsertDirPosition(layout, slot, "dataval")] = entry.key
		transaction.ints[pageInsertDirPosition(layout, slot, "block")] = entry.child
	}
	page := &Page{transaction: transaction, block: block, layout: layout}

	if err := page.InsertDir(1, query.NewIntConstant(15), 150); err != nil {
		t.Fatalf("InsertDir(1) error = %v", err)
	}
	wantEntries := []struct{ key, child int32 }{{10, 100}, {15, 150}, {20, 200}, {30, 300}}
	for slot, want := range wantEntries {
		if got := transaction.ints[pageInsertDirPosition(layout, slot, "dataval")]; got != want.key {
			t.Errorf("slot %d key = %d, want %d", slot, got, want.key)
		}
		if got := transaction.ints[pageInsertDirPosition(layout, slot, "block")]; got != want.child {
			t.Errorf("slot %d child = %d, want %d", slot, got, want.child)
		}
	}
	if transaction.ints[4] != 4 {
		t.Fatalf("record count = %d, want 4", transaction.ints[4])
	}
	wantWrites := []pageInsertDirWrite{
		{block: block, offset: int32(pageInsertDirPosition(layout, 3, "dataval")), intValue: 30, logged: true},
		{block: block, offset: int32(pageInsertDirPosition(layout, 3, "block")), intValue: 300, logged: true},
		{block: block, offset: int32(pageInsertDirPosition(layout, 2, "dataval")), intValue: 20, logged: true},
		{block: block, offset: int32(pageInsertDirPosition(layout, 2, "block")), intValue: 200, logged: true},
		{block: block, offset: 4, intValue: 4, logged: true},
		{block: block, offset: int32(pageInsertDirPosition(layout, 1, "dataval")), intValue: 15, logged: true},
		{block: block, offset: int32(pageInsertDirPosition(layout, 1, "block")), intValue: 150, logged: true},
	}
	if len(transaction.writes) != len(wantWrites) {
		t.Fatalf("writes = %+v, want %+v", transaction.writes, wantWrites)
	}
	for i := range wantWrites {
		if transaction.writes[i] != wantWrites[i] {
			t.Errorf("write %d = %+v, want %+v", i, transaction.writes[i], wantWrites[i])
		}
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 {
		t.Fatal("InsertDir changed the page pin lifecycle")
	}
}

func TestPageInsertDirCopiesVarcharRecordsAndAppendsAtEnd(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 1)
	layout := pageInsertDirLayout(record.VarcharField)
	transaction := pageInsertDirTransactionWithCount(layout, 2)
	transaction.strings[pageInsertDirPosition(layout, 0, "dataval")] = "alpha"
	transaction.ints[pageInsertDirPosition(layout, 0, "block")] = 11
	transaction.strings[pageInsertDirPosition(layout, 1, "dataval")] = "omega"
	transaction.ints[pageInsertDirPosition(layout, 1, "block")] = 22
	page := &Page{transaction: transaction, block: block, layout: layout}

	if err := page.InsertDir(1, query.NewStringConstant("middle"), 17); err != nil {
		t.Fatalf("InsertDir(1) error = %v", err)
	}
	wantKeys := []string{"alpha", "middle", "omega"}
	wantBlocks := []int32{11, 17, 22}
	for slot := range wantKeys {
		if got := transaction.strings[pageInsertDirPosition(layout, slot, "dataval")]; got != wantKeys[slot] {
			t.Errorf("slot %d key = %q, want %q", slot, got, wantKeys[slot])
		}
		if got := transaction.ints[pageInsertDirPosition(layout, slot, "block")]; got != wantBlocks[slot] {
			t.Errorf("slot %d child = %d, want %d", slot, got, wantBlocks[slot])
		}
	}
	if transaction.ints[4] != 3 {
		t.Fatalf("record count = %d, want 3", transaction.ints[4])
	}
	if len(transaction.stringReads) != 1 || transaction.stringReads[0] != pageInsertDirPosition(layout, 1, "dataval") {
		t.Fatalf("varchar reads = %v, want the shifted record's value", transaction.stringReads)
	}
	for _, write := range transaction.writes {
		if !write.logged {
			t.Fatalf("write %+v did not request transaction logging", write)
		}
	}
}

func TestPageInsertDirAppendsToEmptyPageWithoutCopyReads(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(layout, 0)
	page := &Page{transaction: transaction, block: storage.NewBlockID("directory.idx", 2), layout: layout}
	if err := page.InsertDir(0, query.NewIntConstant(4), 40); err != nil {
		t.Fatalf("InsertDir(0) error = %v", err)
	}
	if len(transaction.intReads) != 1 || transaction.intReads[0] != 4 {
		t.Fatalf("integer reads = %v, want only the record count", transaction.intReads)
	}
	if transaction.ints[4] != 1 || transaction.ints[pageInsertDirPosition(layout, 0, "dataval")] != 4 || transaction.ints[pageInsertDirPosition(layout, 0, "block")] != 40 {
		t.Fatalf("empty-page insertion left incorrect data: ints=%v", transaction.ints)
	}
}

func TestPageInsertDirRejectsInvalidPageAndMetadataBeforeMutation(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 0)
	var typedNil *pageInsertDirTransaction

	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: block, layout: pageInsertDirLayout(record.IntegerField)}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: pageInsertDirLayout(record.IntegerField)}},
		{name: "nil layout", page: &Page{transaction: &pageInsertDirTransaction{}, block: block}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.page.InsertDir(0, query.NewIntConstant(1), 2); err == nil {
				t.Fatal("InsertDir() error = nil, want invalid page error")
			}
		})
	}

	validSchema := pageInsertDirSchema(record.IntegerField, record.IntegerField)
	badLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 16)},
		{name: "missing dataval", layout: record.NewLayout(pageInsertDirSchemaWithoutDataval())},
		{name: "unsupported dataval", layout: record.NewLayout(pageInsertDirSchema(record.FieldType(99), record.IntegerField))},
		{name: "duplicate fields", layout: pageInsertDirLayoutWithDuplicateField()},
		{name: "negative varchar length", layout: record.NewLayout(pageInsertDirSchema(record.VarcharField, record.IntegerField, -1))},
		{name: "missing block", layout: record.NewLayout(pageInsertDirSchemaWithoutBlock())},
		{name: "varchar block", layout: pageInsertDirLayoutWithTypes(record.IntegerField, record.VarcharField)},
		{name: "missing dataval offset", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"block": 4}, 8)},
		{name: "missing block offset", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": 0}, 8)},
		{name: "negative dataval offset", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": -1, "block": 4}, 8)},
		{name: "dataval exceeds slot", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": 6, "block": 0}, 8)},
		{name: "negative block offset", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": 0, "block": -1}, 8)},
		{name: "block exceeds slot", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": 0, "block": 6}, 8)},
		{name: "zero slot size", layout: record.NewLayoutFromMetadata(validSchema, map[string]int{"dataval": 0, "block": 4}, 0)},
	}
	for _, badLayout := range badLayouts {
		t.Run(badLayout.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(badLayout.layout, 0)
			page := &Page{transaction: transaction, block: block, layout: badLayout.layout}
			if err := page.InsertDir(0, query.NewIntConstant(1), 2); err == nil {
				t.Fatal("InsertDir() error = nil, want layout error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("invalid layout caused writes: %+v", transaction.writes)
			}
		})
	}

	for _, test := range []struct {
		name   string
		layout *record.Layout
		value  query.Constant
	}{
		{name: "integer field with string value", layout: pageInsertDirLayout(record.IntegerField), value: query.NewStringConstant("bad")},
		{name: "varchar field with integer value", layout: pageInsertDirLayout(record.VarcharField), value: query.NewIntConstant(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(test.layout, 0)
			page := &Page{transaction: transaction, block: block, layout: test.layout}
			if err := page.InsertDir(0, test.value, 2); err == nil {
				t.Fatal("InsertDir() error = nil, want value-type error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("mismatched value caused writes: %+v", transaction.writes)
			}
		})
	}
}

func TestPageInsertDirRejectsInvalidCountsSlotsAndOffsets(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 0)
	layout := pageInsertDirLayout(record.IntegerField)
	for _, test := range []struct {
		name   string
		count  int32
		slot   int
		layout *record.Layout
	}{
		{name: "negative record count", count: -1},
		{name: "negative slot", count: 1, slot: -1},
		{name: "slot after end", count: 1, slot: 2},
		{name: "record count overflow", count: math.MaxInt32},
		{name: "slot arithmetic overflow", count: 1, slot: 1, layout: record.NewLayoutFromMetadata(pageInsertDirSchema(record.IntegerField, record.IntegerField), map[string]int{"dataval": 0, "block": 4}, math.MaxInt)},
		{name: "int32 offset overflow", count: int32(math.MaxInt32 / layout.SlotSize()), slot: math.MaxInt32 / layout.SlotSize()},
	} {
		t.Run(test.name, func(t *testing.T) {
			selectedLayout := test.layout
			if selectedLayout == nil {
				selectedLayout = layout
			}
			transaction := pageInsertDirTransactionWithCount(selectedLayout, test.count)
			page := &Page{transaction: transaction, block: block, layout: selectedLayout}
			if err := page.InsertDir(test.slot, query.NewIntConstant(5), 6); err == nil {
				t.Fatal("InsertDir() error = nil, want count/slot/offset error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("invalid count/slot/offset caused writes: %+v", transaction.writes)
			}
		})
	}
}

func TestPageInsertDirWrapsReadAndWriteFailures(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	block := storage.NewBlockID("directory.idx", 3)
	countError := errors.New("count read failed")
	transaction := pageInsertDirTransactionWithCount(layout, 0)
	transaction.readErrors[4] = countError
	page := &Page{transaction: transaction, block: block, layout: layout}
	if err := page.InsertDir(0, query.NewIntConstant(1), 2); !errors.Is(err, countError) {
		t.Fatalf("InsertDir() count error = %v, want wrapped cause", err)
	}

	readDatavalError := errors.New("source value read failed")
	transaction = pageInsertDirTransactionWithCount(layout, 1)
	transaction.readErrors[pageInsertDirPosition(layout, 0, "dataval")] = readDatavalError
	page = &Page{transaction: transaction, block: block, layout: layout}
	if err := page.InsertDir(0, query.NewIntConstant(1), 2); !errors.Is(err, readDatavalError) {
		t.Fatalf("InsertDir() source-value error = %v, want wrapped cause", err)
	}

	for _, test := range []struct {
		name string
		at   int
	}{
		{name: "record copy", at: 1},
		{name: "record count", at: 3},
		{name: "new dataval", at: 4},
		{name: "new child number", at: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(layout, 1)
			transaction.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
			transaction.ints[pageInsertDirPosition(layout, 0, "block")] = 100
			cause := errors.New(test.name + " write failed")
			transaction.writeErrorAt = test.at
			transaction.writeError = cause
			page := &Page{transaction: transaction, block: block, layout: layout}
			if err := page.InsertDir(0, query.NewIntConstant(1), 2); !errors.Is(err, cause) {
				t.Fatalf("InsertDir() error = %v, want wrapped cause", err)
			}
		})
	}
}

func TestPageInsertDirWrapsVarcharCopyFailures(t *testing.T) {
	layout := pageInsertDirLayout(record.VarcharField)
	block := storage.NewBlockID("directory.idx", 2)
	sourceOffset := pageInsertDirPosition(layout, 0, "dataval")
	for _, test := range []struct {
		name       string
		readError  bool
		writeError int
	}{
		{name: "read", readError: true},
		{name: "write", writeError: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(layout, 1)
			cause := errors.New("varchar copy " + test.name + " failed")
			if test.readError {
				transaction.stringErrors[sourceOffset] = cause
			} else {
				transaction.writeErrorAt = test.writeError
				transaction.writeError = cause
			}
			page := &Page{transaction: transaction, block: block, layout: layout}
			if err := page.InsertDir(0, query.NewStringConstant("middle"), 3); !errors.Is(err, cause) {
				t.Fatalf("InsertDir() error = %v, want wrapped cause", err)
			}
		})
	}
}

func pageInsertDirTransactionWithCount(layout *record.Layout, count int32) *pageInsertDirTransaction {
	return &pageInsertDirTransaction{
		ints:         map[int]int32{4: count},
		strings:      make(map[int]string),
		readErrors:   make(map[int]error),
		stringErrors: make(map[int]error),
	}
}

func pageInsertDirLayout(datavalType record.FieldType) *record.Layout {
	return pageInsertDirLayoutWithTypes(datavalType, record.IntegerField)
}

func pageInsertDirLayoutWithTypes(datavalType, blockType record.FieldType) *record.Layout {
	return record.NewLayout(pageInsertDirSchema(datavalType, blockType))
}

func pageInsertDirSchema(datavalType, blockType record.FieldType, lengths ...int) *record.Schema {
	var schema record.Schema
	if datavalType == record.VarcharField {
		length := 20
		if len(lengths) > 0 {
			length = lengths[0]
		}
		schema.AddVarcharField("dataval", length)
	} else {
		schema.AddField("dataval", datavalType, 0)
	}
	schema.AddField("block", blockType, 8)
	return &schema
}

func pageInsertDirSchemaWithoutDataval() *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("block")
	return &schema
}

func pageInsertDirSchemaWithoutBlock() *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	return &schema
}

func pageInsertDirLayoutWithDuplicateField() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	return record.NewLayout(&schema)
}

func pageInsertDirPosition(layout *record.Layout, slot int, field string) int {
	offset, _ := layout.Offset(field)
	return 8 + slot*layout.SlotSize() + offset
}
