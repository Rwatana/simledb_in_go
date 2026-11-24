package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageInsertDirectoryEntryKeepsIntegerRowsOrdered(t *testing.T) {
	page, tx, layout, block := directoryEntryPageFixture(t,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(30)},
		[]int32{100, 200, 300}, 5, 1)

	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(15), 150)); err != nil {
		t.Fatalf("InsertDirectoryEntry() error = %v", err)
	}
	assertIntegerDirectoryRows(t, tx, layout, block, []int32{10, 15, 20, 30}, []int32{100, 150, 200, 300})
	if tx.ints[4] != 4 {
		t.Fatalf("record count = %d, want 4", tx.ints[4])
	}
}

func TestPageInsertDirectoryEntryPlacesDuplicateBeforeFirstEqualKey(t *testing.T) {
	page, tx, layout, block := directoryEntryPageFixture(t,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(20), query.NewIntConstant(30)},
		[]int32{100, 200, 201, 300}, 6, 0)

	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(20), 199)); err != nil {
		t.Fatalf("InsertDirectoryEntry() error = %v", err)
	}
	assertIntegerDirectoryRows(t, tx, layout, block,
		[]int32{10, 20, 20, 20, 30}, []int32{100, 199, 200, 201, 300})
}

func TestPageInsertDirectoryEntrySupportsVarcharAndEndAppend(t *testing.T) {
	page, tx, layout, _ := directoryEntryPageFixture(t,
		[]query.Constant{query.NewStringConstant("alpha"), query.NewStringConstant("omega")},
		[]int32{11, 22}, 4, 2)

	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewStringConstant("zebra"), 33)); err != nil {
		t.Fatalf("InsertDirectoryEntry() error = %v", err)
	}
	for slot, want := range []string{"alpha", "omega", "zebra"} {
		if got := tx.strings[pageInsertDirPosition(layout, slot, "dataval")]; got != want {
			t.Errorf("slot %d key = %q, want %q", slot, got, want)
		}
		if got := tx.ints[pageInsertDirPosition(layout, slot, "block")]; got != []int32{11, 22, 33}[slot] {
			t.Errorf("slot %d child = %d, want %d", slot, got, []int32{11, 22, 33}[slot])
		}
	}
}

func TestPageInsertDirectoryEntrySupportsEmptyPage(t *testing.T) {
	page, tx, layout, block := directoryEntryPageFixture(t, nil, nil, 2, 0)
	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(4), 40)); err != nil {
		t.Fatalf("InsertDirectoryEntry() on an empty page error = %v", err)
	}
	assertIntegerDirectoryRows(t, tx, layout, block, []int32{4}, []int32{40})
}

func TestPageInsertDirectoryEntryRejectsPageWithoutPhysicalSlotBeforeMutation(t *testing.T) {
	page, tx, _, _ := directoryEntryPageFixture(t,
		[]query.Constant{query.NewIntConstant(1), query.NewIntConstant(2)}, []int32{10, 20}, 2, 0)
	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(3), 30)); err == nil {
		t.Fatal("InsertDirectoryEntry() on a physically full page returned no error")
	}
	if len(tx.writes) != 0 {
		t.Fatalf("full-page insertion wrote before returning: %+v", tx.writes)
	}
}

func TestPageInsertDirectoryEntryRejectsInvalidPageAndLayoutBeforeMutation(t *testing.T) {
	block := storage.NewBlockID("directory-entry.idx", 0)
	validLayout := pageInsertDirLayout(record.IntegerField)
	var typedNil *directoryEntryTransaction
	validEntry := NewDirEntry(query.NewIntConstant(1), 2)

	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: block, layout: validLayout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: validLayout}},
		{name: "nil layout", page: &Page{transaction: &directoryEntryTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(validLayout, 0)}, block: block}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.page.InsertDirectoryEntry(validEntry); err == nil {
				t.Fatal("InsertDirectoryEntry() error = nil, want invalid page error")
			}
		})
	}

	leafLayout := pageInsertLeafLayout(record.IntegerField)
	badLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "leaf layout has RID field", layout: leafLayout},
		{name: "missing data key", layout: record.NewLayout(pageInsertDirSchemaWithoutDataval())},
		{name: "missing child block", layout: record.NewLayout(pageInsertDirSchemaWithoutBlock())},
		{name: "unexpected field", layout: record.NewLayout(directoryEntrySchemaWithUnexpectedField())},
		{name: "noninteger child block", layout: pageInsertDirLayoutWithTypes(record.IntegerField, record.VarcharField)},
		{name: "unsupported data key", layout: pageInsertDirLayoutWithTypes(record.FieldType(99), record.IntegerField)},
	}
	for _, test := range badLayouts {
		t.Run(test.name, func(t *testing.T) {
			transaction := &directoryEntryTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(test.layout, 0), blockSize: 128}
			page := &Page{transaction: transaction, block: block, layout: test.layout}
			if err := page.InsertDirectoryEntry(validEntry); err == nil {
				t.Fatal("InsertDirectoryEntry() error = nil, want directory-layout error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("invalid layout caused writes: %+v", transaction.writes)
			}
		})
	}
}

func TestPageInsertDirectoryEntryRejectsInvalidCountAndHeaderSize(t *testing.T) {
	for _, test := range []struct {
		name      string
		count     int32
		blockSize int
	}{
		{name: "negative count", count: -1, blockSize: 128},
		{name: "count cannot grow", count: math.MaxInt32, blockSize: 128},
		{name: "short page header", count: 0, blockSize: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, tx, _, _ := directoryEntryPageFixture(t, nil, nil, 2, 0)
			tx.ints[4] = test.count
			tx.blockSize = test.blockSize
			if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(1), 1)); err == nil {
				t.Fatal("InsertDirectoryEntry() error = nil, want count or block-size error")
			}
			if len(tx.writes) != 0 {
				t.Fatalf("invalid count or block size caused writes: %+v", tx.writes)
			}
		})
	}
}

func TestPageInsertDirectoryEntryRejectsNegativeLevelAndChildBlockRange(t *testing.T) {
	page, tx, _, _ := directoryEntryPageFixture(t, nil, nil, 2, -1)
	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(1), 1)); err == nil {
		t.Fatal("InsertDirectoryEntry() on negative page level returned no error")
	}
	if len(tx.writes) != 0 {
		t.Fatalf("negative-level page wrote before returning: %+v", tx.writes)
	}

	blockNumbers := []int{-1}
	maxInt := int(^uint(0) >> 1)
	if int64(maxInt) > math.MaxInt32 {
		tooLarge := int64(math.MaxInt32)
		tooLarge++
		blockNumbers = append(blockNumbers, int(tooLarge))
	}
	for _, blockNumber := range blockNumbers {
		full, transaction, _, _ := directoryEntryPageFixture(t, nil, nil, 2, 0)
		if err := full.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(1), blockNumber)); err == nil {
			t.Fatalf("InsertDirectoryEntry() with child block %d returned no error", blockNumber)
		}
		if len(transaction.writes) != 0 {
			t.Fatalf("invalid child block %d caused writes: %+v", blockNumber, transaction.writes)
		}
	}
}

func TestPageInsertDirectoryEntryWrapsFlagCapacityAndPositionErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Page, *directoryEntryTransaction, *record.Layout)
	}{
		{name: "page flag read", mutate: func(_ *Page, tx *directoryEntryTransaction, _ *record.Layout) {
			tx.readErrors[0] = errors.New("level read failed")
		}},
		{name: "record count read", mutate: func(_ *Page, tx *directoryEntryTransaction, _ *record.Layout) {
			tx.readErrors[4] = errors.New("count read failed")
		}},
		{name: "block size read", mutate: func(_ *Page, tx *directoryEntryTransaction, _ *record.Layout) {
			tx.blockSizeError = errors.New("block size read failed")
		}},
		{name: "key read", mutate: func(_ *Page, tx *directoryEntryTransaction, layout *record.Layout) {
			tx.readErrors[pageInsertDirPosition(layout, 0, "dataval")] = errors.New("key read failed")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, tx, layout, _ := directoryEntryPageFixture(t,
				[]query.Constant{query.NewIntConstant(1)}, []int32{10}, 3, 0)
			cause := errors.New(test.name)
			test.mutate(page, tx, layout)
			if test.name == "page flag read" {
				tx.readErrors[0] = cause
			}
			if test.name == "record count read" {
				tx.readErrors[4] = cause
			}
			if test.name == "block size read" {
				tx.blockSizeError = cause
			}
			if test.name == "key read" {
				tx.readErrors[pageInsertDirPosition(layout, 0, "dataval")] = cause
			}
			if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(2), 20)); !errors.Is(err, cause) {
				t.Fatalf("InsertDirectoryEntry() error = %v, want wrapped %v", err, cause)
			}
			if len(tx.writes) != 0 {
				t.Fatalf("failed validation wrote page data: %+v", tx.writes)
			}
		})
	}
}

func TestPageInsertDirectoryEntryWrapsWriteErrors(t *testing.T) {
	page, tx, _, _ := directoryEntryPageFixture(t, nil, nil, 2, 0)
	cause := errors.New("directory entry write failed")
	tx.writeErrorAt = 1
	tx.writeError = cause
	if err := page.InsertDirectoryEntry(NewDirEntry(query.NewIntConstant(1), 2)); !errors.Is(err, cause) {
		t.Fatalf("InsertDirectoryEntry() error = %v, want wrapped %v", err, cause)
	}
}

type directoryEntryTransaction struct {
	*pageInsertDirTransaction
	blockSize      int
	blockSizeError error
}

func (tx *directoryEntryTransaction) BlockSize() (int, error) {
	if tx.blockSizeError != nil {
		return 0, tx.blockSizeError
	}
	return tx.blockSize, nil
}

func directoryEntryPageFixture(t *testing.T, keys []query.Constant, childBlocks []int32, capacity int, level int32) (*Page, *directoryEntryTransaction, *record.Layout, storage.BlockID) {
	t.Helper()
	datavalType := record.IntegerField
	if len(keys) > 0 {
		if _, ok := keys[0].AsString(); ok {
			datavalType = record.VarcharField
		}
	}
	layout := pageInsertDirLayout(datavalType)
	base := pageInsertDirTransactionWithCount(layout, int32(len(keys)))
	base.ints[0] = level
	for slot, key := range keys {
		keyOffset := pageInsertDirPosition(layout, slot, "dataval")
		if value, ok := key.AsInt(); ok {
			base.ints[keyOffset] = value
		} else if value, ok := key.AsString(); ok {
			base.strings[keyOffset] = value
		}
		base.ints[pageInsertDirPosition(layout, slot, "block")] = childBlocks[slot]
	}
	tx := &directoryEntryTransaction{pageInsertDirTransaction: base, blockSize: 8 + capacity*layout.SlotSize()}
	block := storage.NewBlockID("directory-entry.idx", 0)
	return &Page{transaction: tx, block: block, layout: layout}, tx, layout, block
}

func directoryEntrySchemaWithUnexpectedField() *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("child")
	return &schema
}

func assertIntegerDirectoryRows(t *testing.T, tx *directoryEntryTransaction, layout *record.Layout, block storage.BlockID, keys, childBlocks []int32) {
	t.Helper()
	for slot, want := range keys {
		if got := tx.ints[pageInsertDirPosition(layout, slot, "dataval")]; got != want {
			t.Errorf("slot %d key in block %v = %d, want %d", slot, block, got, want)
		}
		if got := tx.ints[pageInsertDirPosition(layout, slot, "block")]; got != childBlocks[slot] {
			t.Errorf("slot %d child in block %v = %d, want %d", slot, block, got, childBlocks[slot])
		}
	}
}
