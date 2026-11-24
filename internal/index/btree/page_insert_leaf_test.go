package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageInsertLeafShiftsCompleteRecordsBeforeWritingVarcharEntry(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 4)
	layout := pageInsertLeafLayout(record.VarcharField)
	transaction := pageInsertDirTransactionWithCount(layout, 2)
	transaction.strings[pageInsertLeafPosition(layout, 0, "dataval")] = "alpha"
	transaction.ints[pageInsertLeafPosition(layout, 0, "block")] = 10
	transaction.ints[pageInsertLeafPosition(layout, 0, "id")] = 1
	transaction.strings[pageInsertLeafPosition(layout, 1, "dataval")] = "omega"
	transaction.ints[pageInsertLeafPosition(layout, 1, "block")] = 20
	transaction.ints[pageInsertLeafPosition(layout, 1, "id")] = 2
	page := &Page{transaction: transaction, block: block, layout: layout}

	if err := page.InsertLeaf(1, query.NewStringConstant("middle"), record.NewRID(15, 7)); err != nil {
		t.Fatalf("InsertLeaf(1) error = %v", err)
	}
	for slot, want := range []struct {
		key   string
		block int32
		id    int32
	}{{"alpha", 10, 1}, {"middle", 15, 7}, {"omega", 20, 2}} {
		if got := transaction.strings[pageInsertLeafPosition(layout, slot, "dataval")]; got != want.key {
			t.Errorf("slot %d key = %q, want %q", slot, got, want.key)
		}
		if got := transaction.ints[pageInsertLeafPosition(layout, slot, "block")]; got != want.block {
			t.Errorf("slot %d RID block = %d, want %d", slot, got, want.block)
		}
		if got := transaction.ints[pageInsertLeafPosition(layout, slot, "id")]; got != want.id {
			t.Errorf("slot %d RID id = %d, want %d", slot, got, want.id)
		}
	}
	if got := transaction.ints[4]; got != 3 {
		t.Fatalf("record count = %d, want 3", got)
	}
	wantWrites := []pageInsertDirWrite{
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "dataval")), textValue: "omega", isString: true, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "block")), intValue: 20, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "id")), intValue: 2, logged: true},
		{block: block, offset: 4, intValue: 3, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "dataval")), textValue: "middle", isString: true, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "block")), intValue: 15, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "id")), intValue: 7, logged: true},
	}
	if len(transaction.writes) != len(wantWrites) {
		t.Fatalf("writes = %+v, want %+v", transaction.writes, wantWrites)
	}
	for i := range wantWrites {
		if transaction.writes[i] != wantWrites[i] {
			t.Errorf("write %d = %+v, want %+v", i, transaction.writes[i], wantWrites[i])
		}
	}
	for _, write := range transaction.writes {
		if !write.logged {
			t.Errorf("write %+v did not request transaction logging", write)
		}
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 {
		t.Fatal("InsertLeaf changed the page pin lifecycle")
	}
}

func TestPageInsertLeafAppendsIntegerEntryWithoutCopyReads(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(layout, 0)
	page := &Page{transaction: transaction, block: storage.NewBlockID("leaf.idx", 1), layout: layout}

	if err := page.InsertLeaf(0, query.NewIntConstant(23), record.NewRID(8, 6)); err != nil {
		t.Fatalf("InsertLeaf(0) error = %v", err)
	}
	if got := transaction.ints[4]; got != 1 {
		t.Fatalf("record count = %d, want 1", got)
	}
	for field, want := range map[string]int32{"dataval": 23, "block": 8, "id": 6} {
		if got := transaction.ints[pageInsertLeafPosition(layout, 0, field)]; got != want {
			t.Errorf("%s = %d, want %d", field, got, want)
		}
	}
	if len(transaction.intReads) != 1 || transaction.intReads[0] != 4 {
		t.Fatalf("integer reads = %v, want only record-count read", transaction.intReads)
	}
}

func TestPageInsertLeafRejectsInvalidPageBeforeMutation(t *testing.T) {
	validLayout := pageInsertLeafLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(validLayout, 0)
	validBlock := storage.NewBlockID("leaf.idx", 1)
	var typedNil *pageInsertDirTransaction
	tests := []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{transaction: transaction, layout: validLayout}},
		{name: "nil transaction", page: &Page{block: validBlock, layout: validLayout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: validBlock, layout: validLayout}},
		{name: "nil layout", page: &Page{transaction: transaction, block: validBlock}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.page.InsertLeaf(0, query.NewIntConstant(1), record.NewRID(2, 3)); err == nil {
				t.Fatal("InsertLeaf() error = nil, want invalid page error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("writes = %+v, want no mutation", transaction.writes)
			}
		})
	}
}

func TestPageInsertLeafRejectsInvalidRIDMetadataAndValuesBeforeMutation(t *testing.T) {
	missingID := pageInsertLeafLayoutWithoutID()
	wrongID := pageInsertLeafLayoutWithIDType(record.VarcharField)
	missingIDOffset := pageInsertLeafLayoutMissingIDOffset()
	missingBlock := pageInsertLeafLayoutWithoutBlock()
	wrongBlock := pageInsertLeafLayoutWithBlockType(record.VarcharField)
	valid := pageInsertLeafLayout(record.IntegerField)
	tests := []struct {
		name   string
		layout *record.Layout
		value  query.Constant
		custom bool
		rid    record.RID
	}{
		{name: "missing id", layout: missingID, rid: record.NewRID(2, 3)},
		{name: "missing id offset", layout: missingIDOffset, rid: record.NewRID(2, 3)},
		{name: "noninteger id", layout: wrongID, rid: record.NewRID(2, 3)},
		{name: "missing block", layout: missingBlock, rid: record.NewRID(2, 3)},
		{name: "noninteger block", layout: wrongBlock, rid: record.NewRID(2, 3)},
		{name: "mismatched key type", layout: valid, value: query.NewStringConstant("bad"), custom: true, rid: record.NewRID(2, 3)},
		{name: "RID block overflow", layout: valid, rid: record.NewRID(int(math.MaxInt32)+1, 3)},
		{name: "RID id overflow", layout: valid, rid: record.NewRID(2, int(math.MinInt32)-1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(test.layout, 1)
			page := &Page{transaction: transaction, block: storage.NewBlockID("leaf.idx", 1), layout: test.layout}
			value := query.NewIntConstant(1)
			if test.custom {
				value = test.value
			}
			if err := page.InsertLeaf(0, value, test.rid); err == nil {
				t.Fatal("InsertLeaf() error = nil, want invalid RID metadata/value error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("writes = %+v, want no mutation", transaction.writes)
			}
		})
	}
}

func TestPageInsertLeafWrapsStorageFailures(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("leaf.idx", 1)
	readErr := errors.New("count read failed")
	transaction := pageInsertDirTransactionWithCount(layout, 0)
	transaction.readErrors[4] = readErr
	page := &Page{transaction: transaction, block: block, layout: layout}
	if err := page.InsertLeaf(0, query.NewIntConstant(1), record.NewRID(2, 3)); !errors.Is(err, readErr) {
		t.Fatalf("count read error = %v, want wrapped cause", err)
	}

	for _, test := range []struct {
		name       string
		writeError int
	}{
		{name: "record count", writeError: 1},
		{name: "dataval", writeError: 2},
		{name: "RID block", writeError: 3},
		{name: "RID id", writeError: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " write failed")
			transaction := pageInsertDirTransactionWithCount(layout, 0)
			transaction.writeErrorAt = test.writeError
			transaction.writeError = cause
			page := &Page{transaction: transaction, block: block, layout: layout}
			if err := page.InsertLeaf(0, query.NewIntConstant(1), record.NewRID(2, 3)); !errors.Is(err, cause) {
				t.Fatalf("write error = %v, want wrapped cause", err)
			}
		})
	}
}

func pageInsertLeafLayout(datavalType record.FieldType) *record.Layout {
	var schema record.Schema
	if datavalType == record.VarcharField {
		schema.AddVarcharField("dataval", 5)
	} else {
		schema.AddField("dataval", datavalType, 0)
	}
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}

func pageInsertLeafPosition(layout *record.Layout, slot int, field string) int {
	offset, _ := layout.Offset(field)
	return 8 + slot*layout.SlotSize() + offset
}

func pageInsertLeafLayoutWithoutID() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	return record.NewLayout(&schema)
}

func pageInsertLeafLayoutWithIDType(idType record.FieldType) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddField("id", idType, 0)
	return record.NewLayout(&schema)
}

func pageInsertLeafLayoutWithoutBlock() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}

func pageInsertLeafLayoutMissingIDOffset() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0, "block": 4}, 12)
}

func pageInsertLeafLayoutWithBlockType(blockType record.FieldType) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddField("block", blockType, 0)
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}
