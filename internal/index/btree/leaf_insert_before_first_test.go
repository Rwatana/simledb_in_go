package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafInsertBeforeFirstSplitsAndReturnsRightDirectoryEntry(t *testing.T) {
	leaf, tx, source, destination, layout := newLeafInsertBeforeFirstFixture(t, []int32{5, 8}, query.NewIntConstant(2))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 9

	entry, err := leaf.Insert(record.NewRID(30, 4))
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if entry == nil || entry.DataVal() != query.NewIntConstant(5) || entry.BlockNumber() != destination.Number() {
		t.Fatalf("Insert() entry = %#v, want first key 5 and right block %d", entry, destination.Number())
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot = %d, want inserted slot 0", leaf.currentSlot)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: 0}]; got != -1 {
		t.Fatalf("left page flag = %d, want -1", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: 4}]; got != 1 {
		t.Fatalf("left page count = %d, want 1", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "dataval")}]; got != 2 {
		t.Fatalf("inserted left key = %d, want 2", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "block")}]; got != 30 {
		t.Fatalf("inserted left RID block = %d, want 30", got)
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "id")}]; got != 4 {
		t.Fatalf("inserted left RID slot = %d, want 4", got)
	}
	if got := tx.ints[pageSplitLocation{block: destination, offset: 0}]; got != 9 {
		t.Fatalf("right page flag = %d, want retained flag 9", got)
	}
	if got := tx.ints[pageSplitLocation{block: destination, offset: 4}]; got != 2 {
		t.Fatalf("right page count = %d, want 2", got)
	}
	for slot, want := range []int32{5, 8} {
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertLeafPosition(layout, slot, "dataval")}]; got != want {
			t.Errorf("right page key at slot %d = %d, want %d", slot, got, want)
		}
	}
	if len(tx.appendCalls) != 1 || tx.appendCalls[0] != source.FileName() {
		t.Fatalf("append calls = %v, want one append to %q", tx.appendCalls, source.FileName())
	}
	if len(tx.pinCalls) != 2 || tx.pinCalls[0] != source || tx.pinCalls[1] != destination || len(tx.unpinCalls) != 1 || tx.unpinCalls[0] != destination {
		t.Fatalf("pin lifecycle = pins %v, unpins %v; want retained source and closed destination", tx.pinCalls, tx.unpinCalls)
	}
}

func TestLeafInsertBeforeFirstSupportsVarcharKeys(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	source := storage.NewBlockID("varchar-leaf.idx", 2)
	destination := storage.NewBlockID("varchar-leaf.idx", 7)
	tx := newPageSplitTransaction(layout, 1)
	tx.blockSize = 8 + 3*layout.SlotSize()
	tx.appendedBlock = destination
	tx.ints[pageSplitLocation{block: source, offset: 4}] = 1
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 4
	tx.strings[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "dataval")}] = "beta"
	tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "block")}] = 12
	tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "id")}] = 3
	leaf, err := NewLeaf(tx, source, layout, query.NewStringConstant("alpha"))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	entry, err := leaf.Insert(record.NewRID(21, 6))
	if err != nil || entry == nil {
		t.Fatalf("Insert() = (%v, %v), want directory entry", entry, err)
	}
	if entry.DataVal() != query.NewStringConstant("beta") || entry.BlockNumber() != destination.Number() {
		t.Fatalf("Insert() entry = (%s, %d), want (beta, %d)", entry.DataVal(), entry.BlockNumber(), destination.Number())
	}
	if got := tx.strings[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, 0, "dataval")}]; got != "alpha" {
		t.Fatalf("left key = %q, want alpha", got)
	}
	if leaf.currentSlot != 0 {
		t.Fatalf("cursor slot = %d, want 0", leaf.currentSlot)
	}
}

func TestLeafInsertBeforeFirstUsesOrdinaryPathWhenPageIsEmptyOrFirstKeyIsNotGreater(t *testing.T) {
	t.Run("empty page", func(t *testing.T) {
		leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, nil, query.NewIntConstant(3))
		tx.ints[pageSplitLocation{block: source, offset: 0}] = 4
		entry, err := leaf.Insert(record.NewRID(2, 1))
		if err != nil || entry != nil {
			t.Fatalf("Insert() = (%v, %v), want nil entry and no error", entry, err)
		}
		if leaf.currentSlot != 0 || tx.ints[pageSplitLocation{block: source, offset: 4}] != 1 || tx.ints[pageSplitLocation{block: source, offset: 0}] != 4 {
			t.Fatalf("empty-page insert state = slot %d, count %d, flag %d", leaf.currentSlot, tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: source, offset: 0}])
		}
		if len(tx.appendCalls) != 0 {
			t.Fatalf("empty-page insertion appended blocks: %v", tx.appendCalls)
		}
	})

	t.Run("first key equals search key", func(t *testing.T) {
		leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(5))
		tx.ints[pageSplitLocation{block: source, offset: 0}] = 0
		entry, err := leaf.Insert(record.NewRID(2, 1))
		if err != nil || entry != nil {
			t.Fatalf("Insert() = (%v, %v), want nil entry and no error", entry, err)
		}
		if leaf.currentSlot != 0 || tx.ints[pageSplitLocation{block: source, offset: 4}] != 2 || tx.ints[pageSplitLocation{block: source, offset: 0}] != 0 {
			t.Fatalf("equal-key insert state = slot %d, count %d, flag %d", leaf.currentSlot, tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: source, offset: 0}])
		}
		if len(tx.appendCalls) != 0 {
			t.Fatalf("equal-key insertion appended blocks: %v", tx.appendCalls)
		}
	})
}

func TestLeafInsertBeforeFirstWrapsReadAndComparisonErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		readOffset int
	}{
		{name: "record count", readOffset: 1},
		{name: "page flag", readOffset: 2},
		{name: "first key", readOffset: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
			cause := errors.New("page read failed")
			tx.readErrorAt = tx.readCalls + test.readOffset
			tx.readError = cause
			if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || !errors.Is(err, cause) {
				t.Fatalf("Insert() = (%v, %v), want wrapped read failure", entry, err)
			}
			if leaf.currentSlot != -1 {
				t.Fatalf("cursor after read failure = %d, want unchanged -1", leaf.currentSlot)
			}
		})
	}

	t.Run("negative record count", func(t *testing.T) {
		leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
		tx.ints[pageSplitLocation{block: source, offset: 4}] = -1
		if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || err == nil {
			t.Fatalf("Insert() = (%v, %v), want negative-count error", entry, err)
		}
	})

	t.Run("key types are incomparable", func(t *testing.T) {
		leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
		leaf.searchKey = query.NewStringConstant("two")
		if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || err == nil {
			t.Fatalf("Insert() = (%v, %v), want comparison error", entry, err)
		}
		if len(tx.appendCalls) != 0 {
			t.Fatalf("incomparable key appended blocks: %v", tx.appendCalls)
		}
	})
}

func TestLeafInsertBeforeFirstWrapsSplitFlagAndInsertionFailures(t *testing.T) {
	t.Run("split append", func(t *testing.T) {
		leaf, tx, _, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
		cause := errors.New("append right page failed")
		tx.appendErr = cause
		if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped split failure", entry, err)
		}
		if leaf.currentSlot != -1 {
			t.Fatalf("cursor after split failure = %d, want -1", leaf.currentSlot)
		}
	})

	t.Run("left flag write", func(t *testing.T) {
		leaf, base, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
		cause := errors.New("left flag write failed")
		leaf.contents.transaction = &leafInsertBeforeFirstFailSetInt{pageSplitTransaction: base, block: source, offset: 0, value: -1, cause: cause}
		if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped flag-write failure", entry, err)
		}
		if leaf.currentSlot != -1 {
			t.Fatalf("cursor after flag-write failure = %d, want -1", leaf.currentSlot)
		}
	})

	t.Run("left entry write", func(t *testing.T) {
		leaf, base, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
		cause := errors.New("left entry count write failed")
		leaf.contents.transaction = &leafInsertBeforeFirstFailSetInt{pageSplitTransaction: base, block: source, offset: 4, value: 1, cause: cause}
		if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || !errors.Is(err, cause) {
			t.Fatalf("Insert() = (%v, %v), want wrapped insertion failure", entry, err)
		}
		if leaf.currentSlot != -1 {
			t.Fatalf("cursor after insertion failure = %d, want -1", leaf.currentSlot)
		}
	})
}

func TestLeafInsertWrapsCapacityCheckFailureAfterSkippingMinimumSplit(t *testing.T) {
	leaf, tx, source, _, _ := newLeafInsertBeforeFirstFixture(t, []int32{5}, query.NewIntConstant(2))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = -1
	cause := errors.New("block size read failed")
	leaf.contents.transaction = &leafInsertBeforeFirstFailBlockSize{pageSplitTransaction: tx, cause: cause}

	if entry, err := leaf.Insert(record.NewRID(2, 1)); entry != nil || !errors.Is(err, cause) {
		t.Fatalf("Insert() = (%v, %v), want wrapped capacity-check failure", entry, err)
	}
	if leaf.currentSlot != -1 {
		t.Fatalf("cursor after capacity failure = %d, want -1", leaf.currentSlot)
	}
}

func TestCompareBTreeKeysHandlesUnsetKindsAndJavaVarcharOrder(t *testing.T) {
	for _, test := range []struct {
		name  string
		left  query.Constant
		right query.Constant
	}{
		{name: "unset left", left: query.Constant{}, right: query.NewIntConstant(1)},
		{name: "integer and string", left: query.NewIntConstant(1), right: query.NewStringConstant("1")},
		{name: "string and integer", left: query.NewStringConstant("1"), right: query.NewIntConstant(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, comparable := compareBTreeKeys(test.left, test.right); comparable {
				t.Fatalf("compareBTreeKeys(%s, %s) comparable = true, want false", test.left, test.right)
			}
		})
	}

	comparison, comparable := compareBTreeKeys(query.NewStringConstant("\U00010000"), query.NewStringConstant("\uE000"))
	if !comparable || comparison >= 0 {
		t.Fatalf("compareBTreeKeys(supplementary, BMP) = (%d, %t), want Java UTF-16 order with supplementary first", comparison, comparable)
	}
}

func newLeafInsertBeforeFirstFixture(t *testing.T, keys []int32, searchKey query.Constant) (*Leaf, *pageSplitTransaction, storage.BlockID, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertLeafLayout(record.IntegerField)
	source := storage.NewBlockID("leaf-before-first.idx", 2)
	destination := storage.NewBlockID("leaf-before-first.idx", 7)
	tx := newPageSplitTransaction(layout, len(keys))
	tx.blockSize = 8 + (len(keys)+2)*layout.SlotSize()
	tx.appendedBlock = destination
	tx.ints[pageSplitLocation{block: source, offset: 4}] = int32(len(keys))
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 11
	for slot, key := range keys {
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "dataval")}] = key
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "block")}] = int32(100 + slot)
		tx.ints[pageSplitLocation{block: source, offset: pageInsertLeafPosition(layout, slot, "id")}] = int32(slot)
	}
	leaf, err := NewLeaf(tx, source, layout, searchKey)
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	return leaf, tx, source, destination, layout
}

type leafInsertBeforeFirstFailSetInt struct {
	*pageSplitTransaction
	block  storage.BlockID
	offset int
	value  int32
	cause  error
}

type leafInsertBeforeFirstFailBlockSize struct {
	*pageSplitTransaction
	cause error
}

func (tx *leafInsertBeforeFirstFailBlockSize) BlockSize() (int, error) {
	return 0, tx.cause
}

func (tx *leafInsertBeforeFirstFailSetInt) SetInt(block storage.BlockID, offset int32, value int32, logged bool) error {
	if block == tx.block && int(offset) == tx.offset && value == tx.value {
		return tx.cause
	}
	return tx.pageSplitTransaction.SetInt(block, offset, value, logged)
}
