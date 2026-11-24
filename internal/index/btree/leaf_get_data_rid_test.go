package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafGetDataRIDReadsCurrentCursorSlotWithoutChangingOwnership(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 2
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	for slot, values := range [][3]int32{{7, 41, 4}, {7, 43, 5}} {
		for fieldIndex, field := range []string{"dataval", "block", "id"} {
			tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, field)}] = values[fieldIndex]
		}
	}

	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	for wantSlot := 0; wantSlot < 2; wantSlot++ {
		if found, err := leaf.Next(); err != nil || !found || leaf.currentSlot != wantSlot {
			t.Fatalf("Next() = (%t, %v) at slot %d, want match at slot %d", found, err, leaf.currentSlot, wantSlot)
		}
	}

	page := leaf.contents
	pinsBefore, unpinsBefore := len(tx.pins), len(tx.unpins)
	tx.intReads = nil
	got, err := leaf.GetDataRID()
	if err != nil || got != record.NewRID(43, 5) {
		t.Fatalf("GetDataRID() = (%v, %v), want (%v, nil)", got, err, record.NewRID(43, 5))
	}
	blockPosition := pageInsertDirPosition(layout, 1, "block")
	idPosition := pageInsertDirPosition(layout, 1, "id")
	if len(tx.intReads) != 2 || tx.intReads[0] != (pageFindLeafOffset{block: block, offset: blockPosition}) || tx.intReads[1] != (pageFindLeafOffset{block: block, offset: idPosition}) {
		t.Fatalf("RID reads = %v, want current-slot block/id offsets %d/%d", tx.intReads, blockPosition, idPosition)
	}
	if leaf.contents != page || leaf.currentSlot != 1 || len(tx.pins) != pinsBefore || len(tx.unpins) != unpinsBefore || !tx.active[block] {
		t.Fatalf("GetDataRID changed cursor/page ownership: page %p slot %d pins %v unpins %v active %v", leaf.contents, leaf.currentSlot, tx.pins, tx.unpins, tx.active)
	}
}

func TestLeafGetDataRIDUsesCurrentOverflowPageAndSlot(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 2)
	overflowBlock := storage.NewBlockID("overflow.idx", 9)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = int32(overflowBlock.Number())
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 3
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "block")}] = 21
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "id")}] = 1
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 0}] = -1
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 3
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 0, "block")}] = 34
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 0, "id")}] = 8

	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(3))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	for _, wantBlock := range []storage.BlockID{block, overflowBlock} {
		if found, err := leaf.Next(); err != nil || !found || leaf.contents.block != wantBlock || leaf.currentSlot != 0 {
			t.Fatalf("Next() = (%t, %v) page %v slot %d, want page %v slot 0", found, err, leaf.contents.block, leaf.currentSlot, wantBlock)
		}
	}
	if got, err := leaf.GetDataRID(); err != nil || got != record.NewRID(34, 8) {
		t.Fatalf("GetDataRID() after overflow = (%v, %v), want (%v, nil)", got, err, record.NewRID(34, 8))
	}
}

func TestLeafGetDataRIDWrapsPageErrorsAndRejectsMissingState(t *testing.T) {
	var nilLeaf *Leaf
	if got, err := nilLeaf.GetDataRID(); got != (record.RID{}) || err == nil {
		t.Fatalf("nil Leaf.GetDataRID() = (%v, %v), want empty RID and error", got, err)
	}
	if got, err := (&Leaf{}).GetDataRID(); got != (record.RID{}) || err == nil {
		t.Fatalf("Leaf.GetDataRID() without page = (%v, %v), want empty RID and error", got, err)
	}

	block := storage.NewBlockID("leaf.idx", 4)
	layout := pageOpenLeafLayout()
	for _, field := range []string{"block", "id"} {
		t.Run(field, func(t *testing.T) {
			cause := errors.New(field + " field read failed")
			tx := newPageFindLeafTransaction()
			offset := pageInsertDirPosition(layout, 0, field)
			tx.intErrors[pageFindLeafOffset{block: block, offset: offset}] = cause
			page, err := NewPage(tx, block, layout)
			if err != nil {
				t.Fatalf("NewPage() error = %v", err)
			}
			leaf := &Leaf{contents: page, searchKey: query.NewIntConstant(7), currentSlot: 0}
			if got, err := leaf.GetDataRID(); got != (record.RID{}) || !errors.Is(err, cause) {
				t.Fatalf("GetDataRID() = (%v, %v), want empty RID and wrapped %v", got, err, cause)
			}
			if leaf.contents != page || leaf.currentSlot != 0 || len(tx.pins) != 1 || len(tx.unpins) != 0 {
				t.Fatalf("failed RID read changed cursor/page ownership: page %p slot %d pins %v unpins %v", leaf.contents, leaf.currentSlot, tx.pins, tx.unpins)
			}
		})
	}
}
