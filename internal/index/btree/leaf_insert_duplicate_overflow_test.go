package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafInsertSplitsFullDuplicatePageIntoOverflowChain(t *testing.T) {
	leaf, tx, source, destination, layout := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 23

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil || entry != nil {
		t.Fatalf("Insert() = (%v, %v), want nil directory entry and no error", entry, err)
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot = %d, want the inserted first duplicate at slot 0", leaf.currentSlot)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: 4}]; got != 1 {
		t.Fatalf("source record count = %d, want only the inserted entry", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: 0}]; got != int32(destination.Number()) {
		t.Fatalf("source overflow flag = %d, want destination block %d", got, destination.Number())
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "dataval")}]; got != 5 {
		t.Fatalf("source key = %d, want inserted key 5", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "block")}]; got != 30 {
		t.Fatalf("source RID block = %d, want 30", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "id")}]; got != 4 {
		t.Fatalf("source RID slot = %d, want 4", got)
	}
	if got := tx.ints[pageSplitLocation{block: destination, offset: 0}]; got != 23 {
		t.Fatalf("destination overflow flag = %d, want retained flag 23", got)
	}
	if got := tx.ints[pageSplitLocation{block: destination, offset: 4}]; got != 2 {
		t.Fatalf("destination record count = %d, want 2", got)
	}
	for slot, wantRID := range [][2]int32{{100, 0}, {101, 1}} {
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "dataval")}]; got != 5 {
			t.Errorf("destination key at slot %d = %d, want 5", slot, got)
		}
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "block")}]; got != wantRID[0] {
			t.Errorf("destination RID block at slot %d = %d, want %d", slot, got, wantRID[0])
		}
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "id")}]; got != wantRID[1] {
			t.Errorf("destination RID slot at slot %d = %d, want %d", slot, got, wantRID[1])
		}
	}
	if len(tx.appendCalls) != 1 || tx.appendCalls[0] != source.FileName() {
		t.Fatalf("append calls = %v, want one append to %q", tx.appendCalls, source.FileName())
	}
	if len(tx.pinCalls) != 2 || tx.pinCalls[0] != source || tx.pinCalls[1] != destination || len(tx.unpinCalls) != 1 || tx.unpinCalls[0] != destination {
		t.Fatalf("pin lifecycle = pins %v, unpins %v; want retained source and closed destination", tx.pinCalls, tx.unpinCalls)
	}
}

func TestLeafInsertSplitsVarcharDuplicatePageIntoOverflowChain(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	source := storage.NewBlockID("varchar-duplicate.idx", 2)
	destination := storage.NewBlockID("varchar-duplicate.idx", 7)
	tx := newPageSplitTransaction(layout, 2)
	tx.blockSize = 8 + 4*layout.SlotSize()
	tx.appendedBlock = destination
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 13
	tx.ints[pageSplitLocation{block: source, offset: 4}] = 2
	for slot, rid := range [][2]int32{{100, 0}, {101, 1}} {
		tx.strings[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "dataval")}] = "same"
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "block")}] = rid[0]
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "id")}] = rid[1]
	}
	leaf, err := NewLeaf(tx, source, layout, query.NewStringConstant("same"))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil || entry != nil {
		t.Fatalf("Insert() = (%v, %v), want nil directory entry and no error", entry, err)
	}
	if leaf.currentSlot != 0 || tx.ints[pageSplitLocation{block: source, offset: 0}] != int32(destination.Number()) || tx.ints[pageSplitLocation{block: destination, offset: 0}] != 13 {
		t.Fatalf("varchar overflow state = cursor %d, source flag %d, destination flag %d", leaf.currentSlot, tx.ints[pageSplitLocation{block: source, offset: 0}], tx.ints[pageSplitLocation{block: destination, offset: 0}])
	}
	if got := tx.strings[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, 0, "dataval")}]; got != "same" {
		t.Fatalf("destination key = %q, want same", got)
	}
}

func TestLeafInsertReportsMixedKeysWhenInsertedPageBecomesFull(t *testing.T) {
	leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 7}, query.NewIntConstant(5))

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if entry != nil || err == nil {
		t.Fatalf("Insert() = (%v, %v), want explicit mixed-key split error", entry, err)
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot after insert = %d, want inserted slot 0", leaf.currentSlot)
	}
	if len(tx.appendCalls) != 0 {
		t.Fatalf("mixed-key page appended blocks %v; non-duplicate split remains a separate behavior", tx.appendCalls)
	}
}

func TestLeafInsertDuplicateOverflowWrapsPostInsertErrors(t *testing.T) {
	for _, test := range []struct {
		name         string
		countErrorAt int
		dataReadSlot int
		flagRead     bool
		blockSizeAt  int
	}{
		{name: "post-insert capacity count", countErrorAt: 1, dataReadSlot: -1},
		{name: "post-insert record count", countErrorAt: 2, dataReadSlot: -1},
		{name: "first key", dataReadSlot: 0},
		{name: "last key", dataReadSlot: 2},
		{name: "overflow flag", dataReadSlot: -1, flagRead: true},
		{name: "post-insert block size", dataReadSlot: -1, blockSizeAt: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
			cause := errors.New("page read failed")
			if test.countErrorAt != 0 || test.dataReadSlot >= 0 || test.flagRead {
				dataReadOffset := 0
				if test.dataReadSlot >= 0 {
					dataReadOffset = pageInsertLeafPosition(leaf.contents.layout, test.dataReadSlot, "dataval")
				}
				leaf.contents.transaction = &leafDuplicateOverflowFailRead{
					pageSplitTransaction: tx,
					source:               source,
					countErrorAt:         test.countErrorAt,
					dataReadOffset:       dataReadOffset,
					failFlagRead:         test.flagRead,
					cause:                cause,
				}
			} else {
				leaf.contents.transaction = &leafDuplicateOverflowFailBlockSize{pageSplitTransaction: tx, failAt: test.blockSizeAt, cause: cause}
			}

			if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || !errors.Is(err, cause) {
				t.Fatalf("Insert() = (%v, %v), want wrapped %v", entry, err, cause)
			}
			if leaf.currentSlot != 0 {
				t.Fatalf("cursor slot after post-insert failure = %d, want inserted slot 0 for transaction rollback", leaf.currentSlot)
			}
			if got := tx.ints[pageSplitLocation{block: source, offset: 4}]; got != 3 {
				t.Fatalf("record count after post-insert failure = %d, want 3 before caller rollback", got)
			}
		})
	}
}

func TestLeafInsertDuplicateOverflowRejectsInvalidPostInsertCount(t *testing.T) {
	leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
	leaf.contents.transaction = &leafDuplicateOverflowFailRead{
		pageSplitTransaction: tx,
		source:               source,
		zeroCountAt:          2,
	}

	if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || err == nil {
		t.Fatalf("Insert() = (%v, %v), want invalid post-insert count error", entry, err)
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot after invalid post-insert count = %d, want 0 before transaction rollback", leaf.currentSlot)
	}
}

func TestLeafInsertDuplicateOverflowWrapsSplitAndLinkErrors(t *testing.T) {
	t.Run("split append", func(t *testing.T) {
		leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
		cause := errors.New("append overflow page failed")
		tx.appendErr = cause
		if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped split error", entry, err)
		}
		if leaf.currentSlot != 0 {
			t.Fatalf("cursor slot = %d, want inserted slot 0 before transaction rollback", leaf.currentSlot)
		}
	})

	t.Run("source link flag write", func(t *testing.T) {
		leaf, tx, source, destination, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
		cause := errors.New("overflow link write failed")
		leaf.contents.transaction = &leafInsertBeforeFirstFailSetInt{pageSplitTransaction: tx, block: source, offset: 0, value: int32(destination.Number()), cause: cause}
		if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped link-write error", entry, err)
		}
		if leaf.currentSlot != 0 {
			t.Fatalf("cursor slot = %d, want inserted slot 0 before transaction rollback", leaf.currentSlot)
		}
	})
}

func TestLeafInsertRejectsOverflowBlockNumberOutsideFlagRange(t *testing.T) {
	leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5, 5}, query.NewIntConstant(5))
	tooLarge := int64(math.MaxInt32) + 1
	tx.appendedBlock = storage.NewBlockID("leaf-before-first.idx", int(tooLarge))

	if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || err == nil {
		t.Fatalf("Insert() with overflow block %d = (%v, %v), want signed flag range error", tooLarge, entry, err)
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot after range error = %d, want inserted slot 0 for transaction rollback", leaf.currentSlot)
	}
	if len(tx.appendCalls) != 1 {
		t.Fatalf("append calls = %v, want split to append before returned block validation", tx.appendCalls)
	}
}

type leafDuplicateOverflowFailBlockSize struct {
	*pageSplitTransaction
	failAt int
	calls  int
	cause  error
}

type leafDuplicateOverflowFailRead struct {
	*pageSplitTransaction
	source         storage.BlockID
	countErrorAt   int
	countReads     int
	zeroCountAt    int
	dataReadOffset int
	failFlagRead   bool
	cause          error
}

func (tx *leafDuplicateOverflowFailRead) GetInt(block storage.BlockID, offset int) (int32, error) {
	insertedCount := tx.pageSplitTransaction.ints[pageSplitLocation{block: tx.source, offset: 4}]
	if block == tx.source && insertedCount == 3 && offset == 4 {
		tx.countReads++
		if tx.countErrorAt == tx.countReads {
			return 0, tx.cause
		}
		if tx.zeroCountAt == tx.countReads {
			return 0, nil
		}
	}
	if block == tx.source && insertedCount == 3 && tx.dataReadOffset > 0 && offset == tx.dataReadOffset {
		return 0, tx.cause
	}
	if block == tx.source && insertedCount == 3 && tx.failFlagRead && offset == 0 {
		return 0, tx.cause
	}
	return tx.pageSplitTransaction.GetInt(block, offset)
}

func (tx *leafDuplicateOverflowFailBlockSize) BlockSize() (int, error) {
	tx.calls++
	if tx.calls == tx.failAt {
		return 0, tx.cause
	}
	return tx.pageSplitTransaction.BlockSize()
}
