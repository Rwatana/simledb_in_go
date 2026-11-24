package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageDeleteShiftsCompleteRecordsLeftInAscendingOrder(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 5)
	layout := pageInsertLeafLayout(record.VarcharField)
	transaction := pageInsertDirTransactionWithCount(layout, 4)
	for slot, entry := range []struct {
		key   string
		block int32
		id    int32
	}{{"alpha", 10, 1}, {"bravo", 20, 2}, {"charlie", 30, 3}, {"delta", 40, 4}} {
		transaction.strings[pageInsertLeafPosition(layout, slot, "dataval")] = entry.key
		transaction.ints[pageInsertLeafPosition(layout, slot, "block")] = entry.block
		transaction.ints[pageInsertLeafPosition(layout, slot, "id")] = entry.id
	}
	page := &Page{transaction: transaction, block: block, layout: layout}

	if err := page.Delete(1); err != nil {
		t.Fatalf("Delete(1) error = %v", err)
	}
	for slot, want := range []struct {
		key   string
		block int32
		id    int32
	}{{"alpha", 10, 1}, {"charlie", 30, 3}, {"delta", 40, 4}} {
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
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "dataval")), textValue: "charlie", isString: true, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "block")), intValue: 30, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 1, "id")), intValue: 3, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "dataval")), textValue: "delta", isString: true, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "block")), intValue: 40, logged: true},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 2, "id")), intValue: 4, logged: true},
		{block: block, offset: 4, intValue: 3, logged: true},
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
		t.Fatal("Delete changed the page pin lifecycle")
	}
}

func TestPageDeleteLastRecordOnlyDecrementsCount(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	transaction := pageInsertDirTransactionWithCount(layout, 1)
	transaction.ints[pageInsertLeafPosition(layout, 0, "dataval")] = 17
	block := storage.NewBlockID("leaf.idx", 2)
	page := &Page{transaction: transaction, block: block, layout: layout}

	if err := page.Delete(0); err != nil {
		t.Fatalf("Delete(0) error = %v", err)
	}
	if got := transaction.ints[4]; got != 0 {
		t.Fatalf("record count = %d, want 0", got)
	}
	if len(transaction.intReads) != 1 || transaction.intReads[0] != 4 {
		t.Fatalf("integer reads = %v, want only record-count read", transaction.intReads)
	}
	wantWrites := []pageInsertDirWrite{{block: block, offset: 4, intValue: 0, logged: true}}
	if len(transaction.writes) != 1 || transaction.writes[0] != wantWrites[0] {
		t.Fatalf("writes = %+v, want %+v", transaction.writes, wantWrites)
	}
}

func TestPageDeleteRejectsInvalidPageBeforeMutation(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	validBlock := storage.NewBlockID("leaf.idx", 1)
	var typedNil *pageInsertDirTransaction
	tests := []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{layout: layout}},
		{name: "nil transaction", page: &Page{block: validBlock, layout: layout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: validBlock, layout: layout}},
		{name: "nil layout", page: &Page{transaction: &pageInsertDirTransaction{}, block: validBlock}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.page.Delete(0); err == nil {
				t.Fatal("Delete() error = nil, want invalid page error")
			}
			if test.page != nil && !isNilPageTransaction(test.page.transaction) {
				transaction := test.page.transaction.(*pageInsertDirTransaction)
				if len(transaction.writes) != 0 {
					t.Fatalf("writes = %+v, want no mutation", transaction.writes)
				}
			}
		})
	}
}

func TestPageDeleteRejectsInvalidCountsSlotsAndOffsetsBeforeMutation(t *testing.T) {
	valid := pageInsertLeafLayout(record.IntegerField)
	missingOffset := pageDeleteLayoutMissingFieldOffset()
	int32Overflow := pageDeleteLayoutWithSlotSize(int(math.MaxInt32) + 20)
	intOverflow := pageDeleteLayoutWithSlotSize(math.MaxInt)
	tests := []struct {
		name   string
		layout *record.Layout
		count  int32
		slot   int
	}{
		{name: "negative count", layout: valid, count: -1, slot: 0},
		{name: "negative slot", layout: valid, count: 1, slot: -1},
		{name: "slot equals count", layout: valid, count: 1, slot: 1},
		{name: "empty page", layout: valid, count: 0, slot: 0},
		{name: "missing field offset", layout: missingOffset, count: 1, slot: 0},
		{name: "int32 offset overflow", layout: int32Overflow, count: 2, slot: 0},
		{name: "integer position overflow", layout: intOverflow, count: 2, slot: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := pageInsertDirTransactionWithCount(test.layout, test.count)
			page := &Page{transaction: transaction, block: storage.NewBlockID("leaf.idx", 1), layout: test.layout}
			if err := page.Delete(test.slot); err == nil {
				t.Fatal("Delete() error = nil, want count, slot, or layout validation error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("writes = %+v, want no mutation", transaction.writes)
			}
		})
	}
}

func TestPageDeleteWrapsCountCopyAndWriteFailures(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	block := storage.NewBlockID("leaf.idx", 1)

	countErr := errors.New("count read failed")
	transaction := pageInsertDirTransactionWithCount(layout, 1)
	transaction.readErrors[4] = countErr
	page := &Page{transaction: transaction, block: block, layout: layout}
	if err := page.Delete(0); !errors.Is(err, countErr) {
		t.Fatalf("count read error = %v, want wrapped cause", err)
	}
	if got := transaction.ints[4]; got != 1 {
		t.Fatalf("record count after read failure = %d, want 1", got)
	}

	stringReadErr := errors.New("varchar copy read failed")
	transaction = pageInsertDirTransactionWithCount(layout, 2)
	transaction.stringErrors[pageInsertLeafPosition(layout, 1, "dataval")] = stringReadErr
	page = &Page{transaction: transaction, block: block, layout: layout}
	if err := page.Delete(0); !errors.Is(err, stringReadErr) {
		t.Fatalf("varchar copy read error = %v, want wrapped cause", err)
	}
	if got := transaction.ints[4]; got != 2 {
		t.Fatalf("record count after varchar read failure = %d, want 2", got)
	}

	intReadErr := errors.New("integer copy read failed")
	transaction = pageInsertDirTransactionWithCount(layout, 2)
	transaction.readErrors[pageInsertLeafPosition(layout, 1, "block")] = intReadErr
	page = &Page{transaction: transaction, block: block, layout: layout}
	if err := page.Delete(0); !errors.Is(err, intReadErr) {
		t.Fatalf("integer copy read error = %v, want wrapped cause", err)
	}
	if got := transaction.ints[4]; got != 2 {
		t.Fatalf("record count after integer read failure = %d, want 2", got)
	}

	for _, test := range []struct {
		name       string
		count      int32
		slot       int
		writeError int
	}{
		{name: "varchar copy write", count: 2, slot: 0, writeError: 1},
		{name: "integer copy write", count: 2, slot: 0, writeError: 2},
		{name: "count write", count: 1, slot: 0, writeError: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " failed")
			transaction := pageInsertDirTransactionWithCount(layout, test.count)
			transaction.writeErrorAt = test.writeError
			transaction.writeError = cause
			page := &Page{transaction: transaction, block: block, layout: layout}
			if err := page.Delete(test.slot); !errors.Is(err, cause) {
				t.Fatalf("Delete() error = %v, want wrapped cause", err)
			}
			if got := transaction.ints[4]; got != test.count {
				t.Fatalf("record count after failed write = %d, want %d", got, test.count)
			}
		})
	}
}

func pageDeleteLayoutMissingFieldOffset() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0, "block": 4}, 12)
}

func pageDeleteLayoutWithSlotSize(slotSize int) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0, "block": 4, "id": 8}, slotSize)
}
