package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafDeleteRemovesFirstMatchingRIDAtCurrentSearchKey(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 3
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leafDeleteSetEntry(tx, layout, block, 0, 7, 12, 1)
	leafDeleteSetEntry(tx, layout, block, 1, 7, 12, 5)
	leafDeleteSetEntry(tx, layout, block, 2, 9, 30, 7)
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	if err := leaf.Delete(record.NewRID(12, 5)); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := tx.ints[pageFindLeafOffset{block: block, offset: 4}]; got != 2 {
		t.Fatalf("record count after matching delete = %d, want 2", got)
	}
	if got := tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 1, "block")}]; got != 30 {
		t.Fatalf("shifted slot 1 data block = %d, want 30", got)
	}
	if got := tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 1, "id")}]; got != 7 {
		t.Fatalf("shifted slot 1 data id = %d, want 7", got)
	}
	if leaf.contents.block != block || leaf.currentSlot != 1 || !tx.active[block] || len(tx.pins) != 1 || len(tx.unpins) != 0 {
		t.Fatalf("delete changed cursor/page ownership: block %v slot %d pins %v unpins %v active %v", leaf.contents.block, leaf.currentSlot, tx.pins, tx.unpins, tx.active)
	}
}

func TestLeafDeleteContinuesThroughOverflowUntilMatchingRID(t *testing.T) {
	block := storage.NewBlockID("overflow.idx", 2)
	overflowBlock := storage.NewBlockID("overflow.idx", 9)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = int32(overflowBlock.Number())
	leafDeleteSetEntry(tx, layout, block, 0, 3, 20, 1)
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}] = 2
	tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 0}] = -1
	leafDeleteSetEntry(tx, layout, overflowBlock, 0, 3, 22, 2)
	leafDeleteSetEntry(tx, layout, overflowBlock, 1, 3, 33, 4)
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(3))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	if err := leaf.Delete(record.NewRID(33, 4)); err != nil {
		t.Fatalf("Delete() through overflow error = %v", err)
	}
	if leaf.contents.block != overflowBlock || leaf.currentSlot != 1 || tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}] != 1 {
		t.Fatalf("overflow delete ended at block %v slot %d with count %d, want overflow slot 1 and count 1", leaf.contents.block, leaf.currentSlot, tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}])
	}
	if tx.ints[pageFindLeafOffset{block: block, offset: 4}] != 1 || tx.maxActive != 1 || len(tx.pins) != 2 || len(tx.unpins) != 1 || tx.unpins[0] != block {
		t.Fatalf("overflow deletion changed unrelated page or pin state: counts %d/%d pins %v unpins %v max active %d", tx.ints[pageFindLeafOffset{block: block, offset: 4}], tx.ints[pageFindLeafOffset{block: overflowBlock, offset: 4}], tx.pins, tx.unpins, tx.maxActive)
	}
}

func TestLeafDeleteDoesNothingWhenCurrentKeyHasNoMatchingRID(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 3)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leafDeleteSetEntry(tx, layout, block, 0, 7, 12, 1)
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if err := leaf.Delete(record.NewRID(99, 99)); err != nil {
		t.Fatalf("Delete() with no matching RID error = %v", err)
	}
	if tx.ints[pageFindLeafOffset{block: block, offset: 4}] != 1 || leaf.currentSlot != 1 || len(tx.unpins) != 0 {
		t.Fatalf("no-match delete mutated the page or ownership: count %d slot %d unpins %v", tx.ints[pageFindLeafOffset{block: block, offset: 4}], leaf.currentSlot, tx.unpins)
	}
}

func TestLeafDeleteWrapsAdvanceAndRIDReadErrors(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 4)
	layout := pageOpenLeafLayout()
	for _, test := range []struct {
		name  string
		field string
	}{
		{name: "advance count", field: "count"},
		{name: "read RID", field: "id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(test.name + " failed")
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
			tx.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
			leafDeleteSetEntry(tx, layout, block, 0, 7, 12, 1)
			leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
			if err != nil {
				t.Fatalf("NewLeaf() error = %v", err)
			}
			position := 4
			if test.field == "id" {
				position = pageInsertDirPosition(layout, 0, "id")
			}
			tx.intErrors[pageFindLeafOffset{block: block, offset: position}] = cause
			if err := leaf.Delete(record.NewRID(12, 1)); !errors.Is(err, cause) {
				t.Fatalf("Delete() error = %v, want wrapped %v", err, cause)
			}
			if tx.ints[pageFindLeafOffset{block: block, offset: 4}] != 1 || leaf.contents.block != block || !tx.active[block] {
				t.Fatalf("failed delete changed page state: count %d page %v active %v", tx.ints[pageFindLeafOffset{block: block, offset: 4}], leaf.contents.block, tx.active)
			}
		})
	}
}

func TestLeafDeleteWrapsPageDeleteFailureAndRetainsCursor(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 5)
	layout := pageOpenLeafLayout()
	base := newPageFindLeafTransaction()
	base.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	base.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	leafDeleteSetEntry(base, layout, block, 0, 7, 12, 1)
	cause := errors.New("delete count write failed")
	tx := &leafDeleteWriteErrorTransaction{pageFindLeafTransaction: base, failOffset: 4, writeError: cause}
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(7))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if err := leaf.Delete(record.NewRID(12, 1)); !errors.Is(err, cause) {
		t.Fatalf("Delete() error = %v, want wrapped %v", err, cause)
	}
	if leaf.contents.block != block || leaf.currentSlot != 0 || base.ints[pageFindLeafOffset{block: block, offset: 4}] != 1 || !base.active[block] {
		t.Fatalf("failed page deletion lost current entry or page pin: block %v slot %d count %d active %v", leaf.contents.block, leaf.currentSlot, base.ints[pageFindLeafOffset{block: block, offset: 4}], base.active)
	}
}

func TestLeafDeleteRejectsNilAndMissingPageState(t *testing.T) {
	var nilLeaf *Leaf
	if err := nilLeaf.Delete(record.NewRID(1, 2)); err == nil {
		t.Fatal("nil Leaf.Delete() error = nil, want error")
	}
	if err := (&Leaf{}).Delete(record.NewRID(1, 2)); err == nil {
		t.Fatal("Leaf.Delete() without page error = nil, want error")
	}
}

func leafDeleteSetEntry(tx *pageFindLeafTransaction, layout *record.Layout, block storage.BlockID, slot int, key, dataBlock, id int32) {
	for field, value := range map[string]int32{"dataval": key, "block": dataBlock, "id": id} {
		tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, field)}] = value
	}
}

type leafDeleteWriteErrorTransaction struct {
	*pageFindLeafTransaction
	failOffset int32
	writeError error
}

func (tx *leafDeleteWriteErrorTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	if offset == tx.failOffset {
		return tx.writeError
	}
	return tx.pageFindLeafTransaction.SetInt(block, offset, value, logged)
}
