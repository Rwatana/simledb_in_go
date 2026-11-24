package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageGetChildNumReadsIntegerBlockFieldFromSlot(t *testing.T) {
	layout := pageChildNumLayout()
	blockOffset, _ := layout.Offset("block")
	position := 8 + 2*layout.SlotSize() + blockOffset
	transaction := &pageDataValueTransaction{integerData: map[int]int32{position: -7}}
	page := newPageForChildNum(t, transaction, layout)

	got, err := page.GetChildNum(2)
	if err != nil || got != -7 {
		t.Fatalf("GetChildNum(2) = (%d, %v), want (-7, nil)", got, err)
	}
	if len(transaction.intOffsets) != 1 || transaction.intOffsets[0] != position || len(transaction.strOffsets) != 0 {
		t.Fatalf("integer reads = %v, string reads = %v, want one integer read at %d", transaction.intOffsets, transaction.strOffsets, position)
	}
	if len(transaction.pins) != 1 || len(transaction.unpins) != 0 {
		t.Fatal("GetChildNum changed the page pin lifecycle")
	}
}

func TestPageGetChildNumRejectsInvalidPageLayoutAndSlot(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 0)
	transaction := &pageDataValueTransaction{}
	layout := pageChildNumLayout()
	var typedNil *pageDataValueTransaction

	var nilSchemaLayout *record.Layout
	if nilSchemaLayout = record.NewLayoutFromMetadata(nil, nil, 8); nilSchemaLayout == nil {
		t.Fatal("NewLayoutFromMetadata returned nil")
	}
	missingOffsetLayout := record.NewLayoutFromMetadata(pageChildNumSchema(record.IntegerField), nil, 12)
	zeroSizeLayout := record.NewLayoutFromMetadata(pageChildNumSchema(record.IntegerField), map[string]int{"block": 4}, 0)
	negativeOffsetLayout := record.NewLayoutFromMetadata(pageChildNumSchema(record.IntegerField), map[string]int{"block": -1}, 12)
	endOffsetLayout := record.NewLayoutFromMetadata(pageChildNumSchema(record.IntegerField), map[string]int{"block": 12}, 12)

	for _, test := range []struct {
		name string
		page *Page
		slot int
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: block, layout: layout}},
		{name: "typed-nil transaction", page: &Page{transaction: typedNil, block: block, layout: layout}},
		{name: "nil layout", page: &Page{transaction: transaction, block: block}},
		{name: "nil schema", page: &Page{transaction: transaction, block: block, layout: nilSchemaLayout}},
		{name: "missing block field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(&record.Schema{})}},
		{name: "missing block offset", page: &Page{transaction: transaction, block: block, layout: missingOffsetLayout}},
		{name: "varchar block field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(pageChildNumSchema(record.VarcharField))}},
		{name: "unsupported block field", page: &Page{transaction: transaction, block: block, layout: record.NewLayout(pageChildNumSchema(record.FieldType(99)))}},
		{name: "zero slot size", page: &Page{transaction: transaction, block: block, layout: zeroSizeLayout}},
		{name: "negative block offset", page: &Page{transaction: transaction, block: block, layout: negativeOffsetLayout}},
		{name: "block offset at slot end", page: &Page{transaction: transaction, block: block, layout: endOffsetLayout}},
		{name: "negative slot", page: &Page{transaction: transaction, block: block, layout: layout}, slot: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.GetChildNum(test.slot); err == nil {
				t.Fatal("GetChildNum() error = nil, want validation error")
			}
		})
	}

	page := &Page{transaction: transaction, block: block, layout: layout}
	if _, err := page.GetChildNum(math.MaxInt); err == nil {
		t.Fatal("GetChildNum() with overflowing slot position error = nil, want error")
	}
}

func TestPageGetChildNumWrapsIntegerReadFailure(t *testing.T) {
	readError := errors.New("child number read failed")
	transaction := &pageDataValueTransaction{integerErr: readError}
	page := newPageForChildNum(t, transaction, pageChildNumLayout())
	if _, err := page.GetChildNum(0); !errors.Is(err, readError) {
		t.Fatalf("GetChildNum(0) error = %v, want wrapped cause", err)
	}
}

func newPageForChildNum(t *testing.T, transaction *pageDataValueTransaction, layout *record.Layout) *Page {
	t.Helper()
	page, err := NewPage(transaction, storage.NewBlockID("directory.idx", 4), layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	return page
}

func pageChildNumLayout() *record.Layout {
	return record.NewLayout(pageChildNumSchema(record.IntegerField))
}

func pageChildNumSchema(blockType record.FieldType) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("separator")
	if blockType == record.VarcharField {
		schema.AddVarcharField("block", 8)
	} else {
		schema.AddField("block", blockType, 1)
	}
	return &schema
}
