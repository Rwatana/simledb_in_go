package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageInsertDirectoryEntryAndSplitReturnsNilWhenSpaceRemains(t *testing.T) {
	page, tx, source, _, layout := newDirectoryInsertSplitFixture(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(30)}, []int32{100, 300}, 4, 1)

	separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200))
	if err != nil {
		t.Fatalf("InsertDirectoryEntryAndSplit() error = %v", err)
	}
	if separator != nil {
		t.Fatalf("InsertDirectoryEntryAndSplit() separator = %v, want nil while space remains", separator)
	}
	assertInsertSplitIntegerRows(t, tx.pageSplitTransaction, layout, source,
		[]int32{10, 20, 30}, []int32{100, 200, 300})
	if len(tx.appendCalls) != 0 {
		t.Fatalf("nonfull insertion appended blocks: %v", tx.appendCalls)
	}
}

func TestPageInsertDirectoryEntryAndSplitSplitsWhenInsertionFillsPage(t *testing.T) {
	page, tx, source, destination, layout := newDirectoryInsertSplitFixture(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(30)}, []int32{100, 200, 300}, 4, 2)

	separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(40), 400))
	if err != nil {
		t.Fatalf("InsertDirectoryEntryAndSplit() error = %v", err)
	}
	if separator == nil {
		t.Fatal("InsertDirectoryEntryAndSplit() separator = nil, want split entry")
	}
	if key, ok := separator.DataVal().AsInt(); !ok || key != 30 {
		t.Fatalf("separator key = %v, want 30", separator.DataVal())
	}
	if separator.BlockNumber() != destination.Number() {
		t.Fatalf("separator block = %d, want %d", separator.BlockNumber(), destination.Number())
	}
	assertInsertSplitIntegerRows(t, tx.pageSplitTransaction, layout, source,
		[]int32{10, 20}, []int32{100, 200})
	assertInsertSplitIntegerRows(t, tx.pageSplitTransaction, layout, destination,
		[]int32{30, 40}, []int32{300, 400})
	if tx.ints[pageSplitLocation{block: source, offset: 0}] != 2 || tx.ints[pageSplitLocation{block: destination, offset: 0}] != 2 {
		t.Fatalf("directory levels source=%d destination=%d, want both 2", tx.ints[pageSplitLocation{block: source, offset: 0}], tx.ints[pageSplitLocation{block: destination, offset: 0}])
	}
	if len(tx.appendCalls) != 1 || len(tx.pinCalls) != 1 || len(tx.unpinCalls) != 1 || tx.pinCalls[0] != destination || tx.unpinCalls[0] != destination {
		t.Fatalf("split lifecycle append=%v pin=%v unpin=%v", tx.appendCalls, tx.pinCalls, tx.unpinCalls)
	}
}

func TestPageInsertDirectoryEntryAndSplitSupportsVarchar(t *testing.T) {
	page, tx, source, destination, layout := newDirectoryInsertSplitFixture(t, record.VarcharField,
		[]query.Constant{query.NewStringConstant("alpha"), query.NewStringConstant("middle"), query.NewStringConstant("zulu")}, []int32{10, 20, 30}, 4, 0)

	separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewStringConstant("zoom"), 40))
	if err != nil {
		t.Fatalf("InsertDirectoryEntryAndSplit() error = %v", err)
	}
	if separator == nil {
		t.Fatal("InsertDirectoryEntryAndSplit() separator = nil, want split entry")
	}
	if key, ok := separator.DataVal().AsString(); !ok || key != "zoom" {
		t.Fatalf("separator key = %v, want zoom", separator.DataVal())
	}
	if separator.BlockNumber() != destination.Number() {
		t.Fatalf("separator block = %d, want %d", separator.BlockNumber(), destination.Number())
	}
	for slot, key := range []string{"alpha", "middle"} {
		if got := tx.strings[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != key {
			t.Errorf("source slot %d key = %q, want %q", slot, got, key)
		}
	}
	for slot, key := range []string{"zoom", "zulu"} {
		if got := tx.strings[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != key {
			t.Errorf("destination slot %d key = %q, want %q", slot, got, key)
		}
	}
}

func TestPageInsertDirectoryEntryAndSplitRejectsFullInputBeforeMutation(t *testing.T) {
	page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 2, 0)
	if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(30), 300)); err == nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and full-page error", separator, err)
	}
	if len(tx.appendCalls) != 0 || len(tx.writes) != 0 {
		t.Fatalf("full input mutated storage: append=%v writes=%v", tx.appendCalls, tx.writes)
	}
}

func TestPageInsertDirectoryEntryAndSplitWrapsInsertionAndPostInsertErrors(t *testing.T) {
	t.Run("insert", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), -1)); err == nil || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and insertion error", separator, err)
		}
		if len(tx.writes) != 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("failed insertion mutated storage: writes=%v append=%v", tx.writes, tx.appendCalls)
		}
	})

	t.Run("post-insert record count", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		cause := errors.New("post-insert record count failed")
		tx.countReadErrorAt = 4
		tx.countReadError = cause
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200)); !errors.Is(err, cause) || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and wrapped %v", separator, err, cause)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("post-insert count failure writes=%v append=%v, want writes and no split", tx.writes, tx.appendCalls)
		}
	})

	t.Run("negative post-insert record count", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		tx.countReadValueAt = 4
		tx.countReadValue = -1
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200)); err == nil || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and negative-count error", separator, err)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("negative count writes=%v append=%v, want writes and no split", tx.writes, tx.appendCalls)
		}
	})

	t.Run("post-insert block size", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		cause := errors.New("post-insert block size failed")
		tx.blockSizeErrorAt = 2
		tx.blockSizeError = cause
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200)); !errors.Is(err, cause) || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and wrapped %v", separator, err, cause)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("post-insert size failure writes=%v append=%v, want writes and no split", tx.writes, tx.appendCalls)
		}
	})

	t.Run("short post-insert header", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		tx.blockSizeValueAt = 2
		tx.blockSizeValue = 7
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200)); err == nil || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and short-header error", separator, err)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("short-header failure writes=%v append=%v, want writes and no split", tx.writes, tx.appendCalls)
		}
	})

	t.Run("count exceeds physical capacity", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 3, 0)
		tx.countReadValueAt = 4
		tx.countReadValue = 4
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(20), 200)); err == nil || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and over-capacity error", separator, err)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 0 {
			t.Fatalf("over-capacity failure writes=%v append=%v, want writes and no split", tx.writes, tx.appendCalls)
		}
	})

	t.Run("split", func(t *testing.T) {
		page, tx, _, _, _ := newDirectoryInsertSplitFixture(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(30)}, []int32{100, 200, 300}, 4, 0)
		cause := errors.New("directory split append failed")
		tx.appendErr = cause
		if separator, err := page.InsertDirectoryEntryAndSplit(NewDirEntry(query.NewIntConstant(40), 400)); !errors.Is(err, cause) || separator != nil {
			t.Fatalf("InsertDirectoryEntryAndSplit() = (%v, %v), want nil separator and wrapped %v", separator, err, cause)
		}
		if len(tx.writes) == 0 || len(tx.appendCalls) != 1 {
			t.Fatalf("failed split writes=%v append=%v, want inserted writes and one split append", tx.writes, tx.appendCalls)
		}
	})
}

type directoryInsertSplitTransaction struct {
	*pageSplitTransaction
	blockSizeCalls   int
	blockSizeErrorAt int
	blockSizeError   error
	blockSizeValueAt int
	blockSizeValue   int
	countReads       int
	countReadErrorAt int
	countReadError   error
	countReadValueAt int
	countReadValue   int32
}

func (tx *directoryInsertSplitTransaction) BlockSize() (int, error) {
	tx.blockSizeCalls++
	if tx.blockSizeCalls == tx.blockSizeErrorAt {
		return 0, tx.blockSizeError
	}
	if tx.blockSizeCalls == tx.blockSizeValueAt {
		return tx.blockSizeValue, nil
	}
	return tx.pageSplitTransaction.BlockSize()
}

func (tx *directoryInsertSplitTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	if offset == 4 {
		tx.countReads++
		if tx.countReads == tx.countReadErrorAt {
			return 0, tx.countReadError
		}
		if tx.countReads == tx.countReadValueAt {
			return tx.countReadValue, nil
		}
	}
	return tx.pageSplitTransaction.GetInt(block, offset)
}

func newDirectoryInsertSplitFixture(t *testing.T, keyType record.FieldType, keys []query.Constant, children []int32, capacity int, level int32) (*Page, *directoryInsertSplitTransaction, storage.BlockID, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertDirLayout(keyType)
	source := storage.NewBlockID("directory-insert-split.idx", 2)
	destination := storage.NewBlockID("directory-insert-split.idx", 7)
	base := newPageSplitTransaction(layout, capacity)
	base.appendedBlock = destination
	base.ints[pageSplitLocation{block: source, offset: 0}] = level
	base.ints[pageSplitLocation{block: source, offset: 4}] = int32(len(keys))
	for slot, key := range keys {
		if value, ok := key.AsInt(); ok {
			base.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = value
		} else if value, ok := key.AsString(); ok {
			base.strings[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = value
		}
		base.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = children[slot]
	}
	tx := &directoryInsertSplitTransaction{pageSplitTransaction: base}
	return &Page{transaction: tx, block: source, layout: layout}, tx, source, destination, layout
}

func assertInsertSplitIntegerRows(t *testing.T, tx *pageSplitTransaction, layout *record.Layout, block storage.BlockID, keys, children []int32) {
	t.Helper()
	if got := tx.ints[pageSplitLocation{block: block, offset: 4}]; got != int32(len(keys)) {
		t.Fatalf("block %v count = %d, want %d", block, got, len(keys))
	}
	for slot, key := range keys {
		if got := tx.ints[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != key {
			t.Errorf("block %v slot %d key = %d, want %d", block, slot, got, key)
		}
		if got := tx.ints[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "block")}]; got != children[slot] {
			t.Errorf("block %v slot %d child = %d, want %d", block, slot, got, children[slot])
		}
	}
}
