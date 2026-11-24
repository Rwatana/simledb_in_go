package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageFindOverflowBlockReturnsMatchingTarget(t *testing.T) {
	for _, test := range []struct {
		name      string
		fieldType record.FieldType
		searchKey query.Constant
		flag      int32
		setKey    func(*pageFindLeafTransaction, storage.BlockID, *record.Layout)
	}{
		{
			name:      "integer key",
			fieldType: record.IntegerField,
			searchKey: query.NewIntConstant(7),
			flag:      9,
			setKey: func(tx *pageFindLeafTransaction, block storage.BlockID, layout *record.Layout) {
				tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 7
			},
		},
		{
			name:      "varchar key",
			fieldType: record.VarcharField,
			searchKey: query.NewStringConstant("same"),
			flag:      9,
			setKey: func(tx *pageFindLeafTransaction, block storage.BlockID, layout *record.Layout) {
				tx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "same"
			},
		},
		{
			name:      "block zero is a valid overflow target",
			fieldType: record.IntegerField,
			searchKey: query.NewIntConstant(7),
			flag:      0,
			setKey: func(tx *pageFindLeafTransaction, block storage.BlockID, layout *record.Layout) {
				tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 7
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			block := storage.NewBlockID("overflow.idx", 3)
			layout := pageInsertDirLayout(test.fieldType)
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: 0}] = test.flag
			test.setKey(tx, block, layout)
			page := &Page{transaction: tx, block: block, layout: layout}

			target, found, err := page.FindOverflowBlock(test.searchKey)
			if err != nil || !found || target != storage.NewBlockID("overflow.idx", int(test.flag)) {
				t.Fatalf("FindOverflowBlock() = (%v, %t, %v), want overflow.idx[%d], true, nil", target, found, err, test.flag)
			}
			if len(tx.pins) != 0 || len(tx.unpins) != 0 || page.block != block || page.layout != layout || page.transaction != tx {
				t.Fatal("FindOverflowBlock changed page ownership or state")
			}
			if test.fieldType == record.IntegerField && (len(tx.intReads) != 2 || tx.intReads[1] != (pageFindLeafOffset{block: block, offset: 0})) {
				t.Fatalf("integer reads = %v, want the dataval followed by flag offset zero", tx.intReads)
			}
		})
	}
}

func TestPageFindOverflowBlockRejectsDifferentFirstKeyAndNegativeFlag(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 4)
	layout := pageInsertDirLayout(record.IntegerField)
	for _, test := range []struct {
		name     string
		firstKey int32
		flag     int32
	}{
		{name: "different first key", firstKey: 8, flag: 12},
		{name: "negative flag", firstKey: 7, flag: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = test.firstKey
			tx.ints[pageFindLeafOffset{block: block, offset: 0}] = test.flag
			page := &Page{transaction: tx, block: block, layout: layout}

			if target, found, err := page.FindOverflowBlock(query.NewIntConstant(7)); err != nil || found || target != (storage.BlockID{}) {
				t.Fatalf("FindOverflowBlock() = (%v, %t, %v), want no target", target, found, err)
			}
			if len(tx.intReads) != 2 || tx.intReads[1] != (pageFindLeafOffset{block: block, offset: 0}) {
				t.Fatalf("integer reads = %v, want first key then leaf flag", tx.intReads)
			}
		})
	}
}

func TestPageFindOverflowBlockRejectsInvalidPageAndKeyState(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 1)
	layout := pageInsertDirLayout(record.IntegerField)
	tx := newPageFindLeafTransaction()
	var typedNil *pageFindLeafTransaction
	var emptySchema record.Schema
	for _, test := range []struct {
		name string
		page *Page
		key  query.Constant
	}{
		{name: "nil page", key: query.NewIntConstant(1)},
		{name: "closed page", page: &Page{transaction: tx, layout: layout}, key: query.NewIntConstant(1)},
		{name: "nil transaction", page: &Page{block: block, layout: layout}, key: query.NewIntConstant(1)},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: layout}, key: query.NewIntConstant(1)},
		{name: "nil layout", page: &Page{transaction: tx, block: block}, key: query.NewIntConstant(1)},
		{name: "nil schema", page: &Page{transaction: tx, block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8)}, key: query.NewIntConstant(1)},
		{name: "missing dataval", page: &Page{transaction: tx, block: block, layout: record.NewLayout(&emptySchema)}, key: query.NewIntConstant(1)},
		{name: "unset key", page: &Page{transaction: tx, block: block, layout: layout}},
		{name: "varchar key for integer field", page: &Page{transaction: tx, block: block, layout: layout}, key: query.NewStringConstant("1")},
		{name: "integer key for varchar field", page: &Page{transaction: tx, block: block, layout: pageInsertDirLayout(record.VarcharField)}, key: query.NewIntConstant(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := test.page.FindOverflowBlock(test.key); err == nil {
				t.Fatal("FindOverflowBlock() returned no error for invalid page or key state")
			}
		})
	}
	if len(tx.intReads) != 0 || len(tx.pins) != 0 || len(tx.unpins) != 0 {
		t.Fatalf("invalid inputs reached transaction: reads %v, pins %v, unpins %v", tx.intReads, tx.pins, tx.unpins)
	}
}

func TestPageFindOverflowBlockRejectsMalformedLayouts(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 2)
	validLayout := pageInsertDirLayout(record.IntegerField)
	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("dataval", record.FieldType(99), 1)
	for _, test := range []struct {
		name   string
		layout *record.Layout
	}{
		{name: "unsupported dataval type", layout: record.NewLayout(&unsupportedSchema)},
		{name: "missing dataval offset", layout: record.NewLayoutFromMetadata(validLayout.Schema(), map[string]int{}, validLayout.SlotSize())},
		{name: "dataval offset outside slot", layout: record.NewLayoutFromMetadata(validLayout.Schema(), map[string]int{"dataval": validLayout.SlotSize()}, validLayout.SlotSize())},
	} {
		t.Run(test.name, func(t *testing.T) {
			page := &Page{transaction: newPageFindLeafTransaction(), block: block, layout: test.layout}
			if _, _, err := page.FindOverflowBlock(query.NewIntConstant(1)); err == nil {
				t.Fatal("FindOverflowBlock() with malformed layout returned no error")
			}
		})
	}
}

func TestPageFindOverflowBlockWrapsFirstKeyAndFlagReadErrors(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 5)
	layout := pageInsertDirLayout(record.IntegerField)
	for _, test := range []struct {
		name   string
		offset int
	}{
		{name: "first key", offset: pageInsertDirPosition(layout, 0, "dataval")},
		{name: "flag", offset: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " read failed")
			tx := newPageFindLeafTransaction()
			tx.intErrors[pageFindLeafOffset{block: block, offset: test.offset}] = cause
			page := &Page{transaction: tx, block: block, layout: layout}
			if _, _, err := page.FindOverflowBlock(query.NewIntConstant(7)); !errors.Is(err, cause) {
				t.Fatalf("FindOverflowBlock() error = %v, want wrapped %v", err, cause)
			}
		})
	}
}
