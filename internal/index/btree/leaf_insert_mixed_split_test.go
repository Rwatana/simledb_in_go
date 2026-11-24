package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafInsertSplitsMixedPageRightOfFirstDuplicateGroup(t *testing.T) {
	leaf, tx, source, destination, layout := newLeafInsertBeforeFirstFixture(t, []int32{1, 1, 2, 2}, query.NewIntConstant(1))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 27

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil || entry == nil {
		t.Fatalf("Insert() = (%v, %v), want separator entry and no error", entry, err)
	}
	if entry.DataVal() != query.NewIntConstant(2) || entry.BlockNumber() != destination.Number() {
		t.Fatalf("directory entry = (%s, %d), want (2, %d)", entry.DataVal(), entry.BlockNumber(), destination.Number())
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot = %d, want the insertion-selected slot 0", leaf.currentSlot)
	}
	if tx.ints[pageSplitLocation{block: source, offset: 0}] != 27 {
		t.Fatalf("source page flag = %d, want unchanged flag 27", tx.ints[pageSplitLocation{block: source, offset: 0}])
	}
	if tx.ints[pageSplitLocation{block: source, offset: 4}] != 3 || tx.ints[pageSplitLocation{block: destination, offset: 4}] != 2 {
		t.Fatalf("split record counts = %d/%d, want 3/2", tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: destination, offset: 4}])
	}
	if tx.ints[pageSplitLocation{block: destination, offset: 0}] != -1 {
		t.Fatalf("destination flag = %d, want -1", tx.ints[pageSplitLocation{block: destination, offset: 0}])
	}
	for slot, want := range []int32{1, 1, 1} {
		if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "dataval")}]; got != want {
			t.Errorf("source key at slot %d = %d, want %d", slot, got, want)
		}
	}
	for slot, want := range []int32{2, 2} {
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "dataval")}]; got != want {
			t.Errorf("destination key at slot %d = %d, want %d", slot, got, want)
		}
	}
	if len(tx.appendCalls) != 1 || len(tx.pinCalls) != 2 || len(tx.unpinCalls) != 1 || tx.unpinCalls[0] != destination {
		t.Fatalf("split resource calls = append %v, pins %v, unpins %v", tx.appendCalls, tx.pinCalls, tx.unpinCalls)
	}
}

func TestLeafInsertSplitsMixedPageAtBeginningOfMiddleKeyGroup(t *testing.T) {
	leaf, tx, source, destination, layout := newLeafInsertBeforeFirstFixture(t, []int32{1, 2, 2, 2, 3}, query.NewIntConstant(2))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 41

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil || entry == nil {
		t.Fatalf("Insert() = (%v, %v), want separator entry and no error", entry, err)
	}
	if entry.DataVal() != query.NewIntConstant(2) || entry.BlockNumber() != destination.Number() {
		t.Fatalf("directory entry = (%s, %d), want (2, %d)", entry.DataVal(), entry.BlockNumber(), destination.Number())
	}
	if leaf.currentSlot != 1 {
		t.Fatalf("cursor slot = %d, want unchanged insertion-selected slot 1", leaf.currentSlot)
	}
	if tx.ints[pageSplitLocation{block: source, offset: 0}] != 41 || tx.ints[pageSplitLocation{block: destination, offset: 0}] != -1 {
		t.Fatalf("page flags = %d/%d, want unchanged source 41 and new-page -1", tx.ints[pageSplitLocation{block: source, offset: 0}], tx.ints[pageSplitLocation{block: destination, offset: 0}])
	}
	if tx.ints[pageSplitLocation{block: source, offset: 4}] != 1 || tx.ints[pageSplitLocation{block: destination, offset: 4}] != 5 {
		t.Fatalf("split record counts = %d/%d, want 1/5", tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: destination, offset: 4}])
	}
	for slot, want := range []int32{2, 2, 2, 2, 3} {
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "dataval")}]; got != want {
			t.Errorf("destination key at slot %d = %d, want %d", slot, got, want)
		}
	}
}

func TestLeafInsertSplitsMixedVarcharPageAtDuplicateBoundary(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	source := storage.NewBlockID("varchar-mixed.idx", 2)
	destination := storage.NewBlockID("varchar-mixed.idx", 7)
	tx := newPageSplitTransaction(layout, 4)
	tx.blockSize = 8 + 6*layout.SlotSize()
	tx.appendedBlock = destination
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 18
	tx.ints[pageSplitLocation{block: source, offset: 4}] = 4
	for slot, value := range []string{"a", "a", "b", "b"} {
		tx.strings[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "dataval")}] = value
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "block")}] = int32(100 + slot)
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "id")}] = int32(slot)
	}
	leaf, err := NewLeaf(tx, source, layout, query.NewStringConstant("a"))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil || entry == nil || entry.DataVal() != query.NewStringConstant("b") || entry.BlockNumber() != destination.Number() {
		t.Fatalf("Insert() = (%v, %v), want separator (b, %d)", entry, err, destination.Number())
	}
	if tx.ints[pageSplitLocation{block: source, offset: 4}] != 3 || tx.ints[pageSplitLocation{block: destination, offset: 4}] != 2 {
		t.Fatalf("varchar split counts = %d/%d, want 3/2", tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: destination, offset: 4}])
	}
	if got := tx.strings[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, 0, "dataval")}]; got != "b" {
		t.Fatalf("destination first key = %q, want b", got)
	}
}

func TestLeafInsertMixedSplitWrapsBoundaryKeyReadErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		keys []int32
		key  int32
		slot int
	}{
		{name: "middle key", keys: []int32{1, 1, 2, 2}, key: 1, slot: 2},
		{name: "move right", keys: []int32{1, 1, 1, 2}, key: 1, slot: 3},
		{name: "move left", keys: []int32{1, 2, 2, 2, 3}, key: 2, slot: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			leaf, tx, source, _, layout := newLeafInsertBeforeFirstFixture(t, test.keys, query.NewIntConstant(test.key))
			cause := errors.New("boundary key read failed")
			leaf.contents.transaction = &leafMixedSplitFailKeyRead{
				pageSplitTransaction: tx,
				source:               source,
				countAfterInsert:     int32(len(test.keys) + 1),
				keyOffset:            pageInsertLeafPosition(layout, test.slot, "dataval"),
				cause:                cause,
			}

			if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || !errors.Is(err, cause) {
				t.Fatalf("Insert() = (%v, %v), want wrapped boundary read failure", entry, err)
			}
		})
	}
}

func TestLeafInsertMixedSplitWrapsSplitAndBlockRangeErrors(t *testing.T) {
	t.Run("append failure", func(t *testing.T) {
		leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{1, 1, 2, 2}, query.NewIntConstant(1))
		cause := errors.New("append split page failed")
		tx.appendErr = cause
		if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped split failure", entry, err)
		}
	})

	t.Run("block number overflow", func(t *testing.T) {
		leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{1, 1, 2, 2}, query.NewIntConstant(1))
		tooLarge := int64(math.MaxInt32) + 1
		tx.appendedBlock = storage.NewBlockID("leaf-before-first.idx", int(tooLarge))
		if entry, err := leaf.Insert(record.NewRID(30, 4)); entry != nil || err == nil {
			t.Fatalf("Insert() with destination %d = (%v, %v), want range error", tooLarge, entry, err)
		}
	})
}

type leafMixedSplitFailKeyRead struct {
	*pageSplitTransaction
	source           storage.BlockID
	countAfterInsert int32
	keyOffset        int
	cause            error
}

func (tx *leafMixedSplitFailKeyRead) GetInt(block storage.BlockID, offset int) (int32, error) {
	if block == tx.source && offset == tx.keyOffset && tx.pageSplitTransaction.ints[pageSplitLocation{block: tx.source, offset: 4}] == tx.countAfterInsert {
		return 0, tx.cause
	}
	return tx.pageSplitTransaction.GetInt(block, offset)
}
