package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafNextAdvancesMatchingSlotsAndStopsAtDifferentKey(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 3
	for slot, key := range []int32{7, 7, 9} {
		tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}] = key
	}
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	for _, wantSlot := range []int{0, 1} {
		found, err := leaf.Next()
		if err != nil || !found || leaf.currentSlot != wantSlot {
			t.Fatalf("Next() = (%t, %v) at slot %d, want true at slot %d", found, err, leaf.currentSlot, wantSlot)
		}
	}
	if found, err := leaf.Next(); err != nil || found || leaf.currentSlot != 2 {
		t.Fatalf("Next() at first different key = (%t, %v), slot %d; want false at slot 2", found, err, leaf.currentSlot)
	}
	if len(tx.pins) != 1 || len(tx.unpins) != 0 || leaf.contents.block != block {
		t.Fatalf("same-page iteration changed ownership: pins %v, unpins %v, block %v", tx.pins, tx.unpins, leaf.contents.block)
	}
}

func TestLeafNextFollowsMatchingOverflowBlockAndContinues(t *testing.T) {
	currentBlock := storage.NewBlockID("overflow.idx", 2)
	overflowBlock := storage.NewBlockID("overflow.idx", 9)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: currentBlock, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: currentBlock, offset: 0}] = 9
	tx.ints[pageFindLeafOffset{block: currentBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 3
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}] = 2
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 0}] = -1
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 3
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 1, "dataval")}] = 3
	leaf, err := NewLeaf(tx, currentBlock, layout, query.NewIntConstant(3))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	if found, err := leaf.Next(); err != nil || !found || leaf.currentSlot != 0 {
		t.Fatalf("Next() on current leaf = (%t, %v), slot %d; want first entry", found, err, leaf.currentSlot)
	}
	if found, err := leaf.Next(); err != nil || !found || leaf.currentSlot != 0 || leaf.contents.block != overflowBlock {
		t.Fatalf("Next() following overflow = (%t, %v), slot %d block %v; want overflow slot zero", found, err, leaf.currentSlot, leaf.contents.block)
	}
	if tx.maxActive != 1 || len(tx.pins) != 2 || tx.pins[0] != currentBlock || tx.pins[1] != overflowBlock || len(tx.unpins) != 1 || tx.unpins[0] != currentBlock {
		t.Fatalf("overflow pin lifecycle = pins %v unpins %v max active %d; want close-before-pin", tx.pins, tx.unpins, tx.maxActive)
	}
	if found, err := leaf.Next(); err != nil || !found || leaf.currentSlot != 1 {
		t.Fatalf("Next() in overflow leaf = (%t, %v), slot %d; want second duplicate", found, err, leaf.currentSlot)
	}
	if found, err := leaf.Next(); err != nil || found || leaf.currentSlot != 2 {
		t.Fatalf("Next() after overflow matches = (%t, %v), slot %d; want false at page end", found, err, leaf.currentSlot)
	}
}

func TestLeafNextStopsWhenNoMatchingOverflowExists(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 1)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = 4
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 8
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(5))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if found, err := leaf.Next(); err != nil || found || leaf.currentSlot != 0 {
		t.Fatalf("Next() with different first key = (%t, %v), slot %d; want no match", found, err, leaf.currentSlot)
	}
	if len(tx.pins) != 1 || len(tx.unpins) != 0 || leaf.contents.block != block {
		t.Fatalf("no-overflow result changed page ownership: pins %v unpins %v", tx.pins, tx.unpins)
	}
}

func TestLeafNextWrapsPageAndOverflowReadErrors(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 4)
	layout := pageOpenLeafLayout()
	for _, test := range []struct {
		name   string
		offset int
		count  int32
	}{
		{name: "record count", offset: 4, count: 1},
		{name: "data value", offset: pageInsertDirPosition(layout, 0, "dataval"), count: 1},
		{name: "overflow flag", offset: 0, count: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: 4}] = test.count
			cause := errors.New(test.name + " read failed")
			tx.intErrors[pageFindLeafOffset{block: block, offset: test.offset}] = cause
			page, err := NewPage(tx, block, layout)
			if err != nil {
				t.Fatalf("NewPage() error = %v", err)
			}
			leaf := &Leaf{contents: page, searchKey: query.NewIntConstant(7), currentSlot: -1}
			if found, err := leaf.Next(); found || !errors.Is(err, cause) {
				t.Fatalf("Next() = (%t, %v), want wrapped %v", found, err, cause)
			}
		})
	}
}

func TestLeafNextWrapsOverflowTransitionFailures(t *testing.T) {
	currentBlock := storage.NewBlockID("leaf.idx", 3)
	overflowBlock := storage.NewBlockID("leaf.idx", 8)
	layout := pageOpenLeafLayout()
	for _, test := range []struct {
		name       string
		pinFailure bool
	}{
		{name: "unpin failure"},
		{name: "overflow pin failure", pinFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name)
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: currentBlock, offset: 4}] = 0
			tx.ints[pageFindLeafOffset{block: currentBlock, offset: 0}] = int32(overflowBlock.Number())
			tx.ints[pageFindLeafOffset{block: currentBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = 7
			if test.pinFailure {
				tx.pinErrors[overflowBlock] = cause
			} else {
				tx.unpinErrors[currentBlock] = cause
			}
			leaf, err := NewLeaf(tx, currentBlock, layout, query.NewIntConstant(7))
			if err != nil {
				t.Fatalf("NewLeaf() error = %v", err)
			}
			if found, err := leaf.Next(); found || !errors.Is(err, cause) {
				t.Fatalf("Next() = (%t, %v), want wrapped transition cause", found, err)
			}
			if test.pinFailure && leaf.contents.block.FileName() != "" {
				t.Fatalf("page after failed overflow pin = %v, want closed", leaf.contents.block)
			}
			if !test.pinFailure && leaf.contents.block != currentBlock {
				t.Fatalf("page after failed unpin = %v, want current page retained", leaf.contents.block)
			}
		})
	}
}

func TestLeafNextRejectsNilAndMissingPageState(t *testing.T) {
	var nilLeaf *Leaf
	if found, err := nilLeaf.Next(); found || err == nil {
		t.Fatalf("nil Leaf.Next() = (%t, %v), want error", found, err)
	}
	if found, err := (&Leaf{searchKey: query.NewIntConstant(1)}).Next(); found || err == nil {
		t.Fatalf("Leaf.Next() without contents = (%t, %v), want error", found, err)
	}
}

func TestLeafNextWithVarcharOverflowDoesNotChangeKeyType(t *testing.T) {
	currentBlock := storage.NewBlockID("names.idx", 1)
	overflowBlock := storage.NewBlockID("names.idx", 5)
	layout := pageInsertDirLayout(record.VarcharField)
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: currentBlock, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: currentBlock, offset: 0}] = 5
	tx.strings[pageFindLeafOffset{block: currentBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "same"
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 0}] = -1
	tx.strings[pageFindLeafOffset{block: overflowBlock, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "same"
	leaf, err := NewLeaf(tx, currentBlock, layout, query.NewStringConstant("same"))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if found, err := leaf.Next(); err != nil || !found || leaf.currentSlot != 0 {
		t.Fatalf("Next() current varchar page = (%t, %v), slot %d", found, err, leaf.currentSlot)
	}
	if found, err := leaf.Next(); err != nil || !found || leaf.contents.block != overflowBlock || leaf.currentSlot != 0 {
		t.Fatalf("Next() varchar overflow = (%t, %v), block %v slot %d", found, err, leaf.contents.block, leaf.currentSlot)
	}
}
