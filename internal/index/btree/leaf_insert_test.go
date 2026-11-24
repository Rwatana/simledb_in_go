package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafInsertAddsTypedEntryAtCursorPosition(t *testing.T) {
	for _, test := range []struct {
		name   string
		layout *record.Layout
		key    query.Constant
		values [][3]any
	}{
		{
			name:   "integer key",
			layout: pageOpenLeafLayout(),
			key:    query.NewIntConstant(5),
			values: [][3]any{{int32(3), int32(10), int32(1)}, {int32(9), int32(20), int32(2)}},
		},
		{
			name:   "varchar key",
			layout: pageInsertLeafLayout(record.VarcharField),
			key:    query.NewStringConstant("bravo"),
			values: [][3]any{{"alpha", int32(10), int32(1)}, {"delta", int32(20), int32(2)}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			block := storage.NewBlockID("leaf.idx", 2)
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: 4}] = int32(len(test.values))
			tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
			for slot, values := range test.values {
				dataValOffset := pageInsertDirPosition(test.layout, slot, "dataval")
				if text, ok := values[0].(string); ok {
					tx.strings[pageFindLeafOffset{block: block, offset: dataValOffset}] = text
				} else {
					tx.ints[pageFindLeafOffset{block: block, offset: dataValOffset}] = values[0].(int32)
				}
				tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, slot, "block")}] = values[1].(int32)
				tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, slot, "id")}] = values[2].(int32)
			}
			leaf, err := NewLeaf(tx, block, test.layout, test.key)
			if err != nil {
				t.Fatalf("NewLeaf() error = %v", err)
			}

			entry, err := leaf.Insert(record.NewRID(40, 6))
			if err != nil || entry != nil {
				t.Fatalf("Insert() = (%v, %v), want (nil, nil) for the in-page path", entry, err)
			}
			if leaf.currentSlot != 1 || tx.ints[pageFindLeafOffset{block: block, offset: 4}] != 3 {
				t.Fatalf("cursor/count after Insert() = %d/%d, want 1/3", leaf.currentSlot, tx.ints[pageFindLeafOffset{block: block, offset: 4}])
			}
			if got := tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, 1, "block")}]; got != 40 {
				t.Fatalf("inserted data block = %d, want 40", got)
			}
			if got := tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, 1, "id")}]; got != 6 {
				t.Fatalf("inserted data id = %d, want 6", got)
			}
			if _, isVarchar := test.key.AsString(); isVarchar {
				if got := tx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, 1, "dataval")}]; got != "bravo" {
					t.Fatalf("inserted varchar key = %q, want bravo", got)
				}
			} else if got := tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(test.layout, 1, "dataval")}]; got != 5 {
				t.Fatalf("inserted integer key = %d, want 5", got)
			}
			if tx.maxActive != 1 || len(tx.pins) != 1 || len(tx.unpins) != 0 {
				t.Fatalf("Insert() changed page ownership: pins %v unpins %v max active %d", tx.pins, tx.unpins, tx.maxActive)
			}
		})
	}
}

func TestLeafInsertRejectsFullPageAndInvalidCursorBeforeMutation(t *testing.T) {
	block := storage.NewBlockID("full.idx", 1)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	fullCount := int32((4096 - 8) / layout.SlotSize())
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = fullCount
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(0))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if entry, err := leaf.Insert(record.NewRID(2, 3)); entry != nil || err == nil {
		t.Fatalf("Insert() on full page = (%v, %v), want error before mutation", entry, err)
	}
	if leaf.currentSlot != -1 || tx.ints[pageFindLeafOffset{block: block, offset: 4}] != fullCount {
		t.Fatalf("full-page rejection changed cursor/count: %d/%d", leaf.currentSlot, tx.ints[pageFindLeafOffset{block: block, offset: 4}])
	}

	for _, slot := range []int{-2, math.MaxInt} {
		badCursor := &Leaf{contents: &Page{transaction: tx, block: block, layout: layout}, searchKey: query.NewIntConstant(1), currentSlot: slot}
		if entry, err := badCursor.Insert(record.NewRID(2, 3)); entry != nil || err == nil {
			t.Fatalf("Insert() with cursor slot %d = (%v, %v), want error", slot, entry, err)
		}
	}
}

func TestLeafInsertWrapsCapacityAndPageInsertErrors(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 3)
	layout := pageOpenLeafLayout()
	countErr := errors.New("count read failed")
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 0
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	tx.intErrors[pageFindLeafOffset{block: block, offset: 4}] = countErr
	if entry, err := leaf.Insert(record.NewRID(12, 2)); entry != nil || !errors.Is(err, countErr) {
		t.Fatalf("Insert() capacity error = (%v, %v), want wrapped count error", entry, err)
	}
	if leaf.currentSlot != -1 {
		t.Fatalf("cursor after capacity error = %d, want -1", leaf.currentSlot)
	}

	var schema record.Schema
	schema.AddIntegerField("dataval")
	invalidLayout := record.NewLayout(&schema)
	insertTx := newPageFindLeafTransaction()
	page := &Page{transaction: insertTx, block: block, layout: invalidLayout}
	insertLeaf := &Leaf{contents: page, searchKey: query.NewIntConstant(7), currentSlot: -1}
	if entry, err := insertLeaf.Insert(record.NewRID(12, 2)); entry != nil || err == nil {
		t.Fatalf("Insert() with invalid leaf layout = (%v, %v), want wrapped insert error", entry, err)
	}
	if insertLeaf.currentSlot != -1 || insertTx.ints[pageFindLeafOffset{block: block, offset: 4}] != 0 {
		t.Fatalf("failed page insert changed cursor/count: %d/%d", insertLeaf.currentSlot, insertTx.ints[pageFindLeafOffset{block: block, offset: 4}])
	}
}

func TestLeafInsertRejectsNilState(t *testing.T) {
	var nilLeaf *Leaf
	if entry, err := nilLeaf.Insert(record.NewRID(1, 2)); entry != nil || err == nil {
		t.Fatalf("nil Leaf.Insert() = (%v, %v), want error", entry, err)
	}
	if entry, err := (&Leaf{}).Insert(record.NewRID(1, 2)); entry != nil || err == nil {
		t.Fatalf("Leaf.Insert() without page = (%v, %v), want error", entry, err)
	}
}
