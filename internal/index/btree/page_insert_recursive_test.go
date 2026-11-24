package btree

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recursiveDirectoryTransaction struct {
	*pageSplitTransaction
	appendBlocks    []storage.BlockID
	appendErrorAt   int
	appendError     error
	pinErrorBlock   storage.BlockID
	pinError        error
	unpinErrorBlock storage.BlockID
	unpinError      error
}

func (tx *recursiveDirectoryTransaction) Append(fileName string) (storage.BlockID, error) {
	tx.appendCalls = append(tx.appendCalls, fileName)
	if len(tx.appendCalls) == tx.appendErrorAt {
		return storage.BlockID{}, tx.appendError
	}
	index := len(tx.appendCalls) - 1
	if index >= len(tx.appendBlocks) {
		return storage.BlockID{}, errors.New("unexpected directory append")
	}
	return tx.appendBlocks[index], nil
}

func (tx *recursiveDirectoryTransaction) Pin(block storage.BlockID) error {
	tx.pinCalls = append(tx.pinCalls, block)
	if block == tx.pinErrorBlock {
		return tx.pinError
	}
	return nil
}

func (tx *recursiveDirectoryTransaction) Unpin(block storage.BlockID) error {
	tx.unpinCalls = append(tx.unpinCalls, block)
	if block == tx.unpinErrorBlock {
		return tx.unpinError
	}
	return nil
}

func TestPageInsertDirectoryEntryRecursiveInsertsThroughOneInternalLevel(t *testing.T) {
	page, tx, root, child, layout := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10}, []int32{100}, 0,
		4,
	)

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200))
	if err != nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want no separator and no error", separator, err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10}, []int32{1})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, child, layout, []int32{10, 20}, []int32{100, 200})
	if len(tx.pinCalls) != 2 || tx.pinCalls[1] != child || len(tx.unpinCalls) != 1 || tx.unpinCalls[0] != child {
		t.Fatalf("child pin lifecycle = pins %v, unpins %v", tx.pinCalls, tx.unpinCalls)
	}
}

func TestPageInsertDirectoryEntryRecursivePropagatesThroughMultipleInternalLevels(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	root := storage.NewBlockID("recursive-directory.idx", 0)
	middle := storage.NewBlockID(root.FileName(), 1)
	leaf := storage.NewBlockID(root.FileName(), 2)
	leafRight := storage.NewBlockID(root.FileName(), 8)
	middleRight := storage.NewBlockID(root.FileName(), 9)
	tx := &recursiveDirectoryTransaction{
		pageSplitTransaction: newPageSplitTransaction(layout, 4),
		appendBlocks:         []storage.BlockID{leafRight, middleRight},
	}
	setRecursiveDirectoryRows(tx.pageSplitTransaction, root, layout, 2, []int32{10}, []int32{1})
	setRecursiveDirectoryRows(tx.pageSplitTransaction, middle, layout, 1, []int32{10, 20, 30}, []int32{2, 2, 2})
	setRecursiveDirectoryRows(tx.pageSplitTransaction, leaf, layout, 0, []int32{10, 20, 30}, []int32{100, 200, 300})
	page, err := NewPage(tx, root, layout)
	if err != nil {
		t.Fatalf("NewPage(root) error = %v", err)
	}

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(40), 400))
	if err != nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want propagated insertion without root split", separator, err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 30}, []int32{1, 9})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, middle, layout, []int32{10, 20}, []int32{2, 2})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, middleRight, layout, []int32{30, 30}, []int32{8, 2})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, leaf, layout, []int32{10, 20}, []int32{100, 200})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, leafRight, layout, []int32{30, 40}, []int32{300, 400})
	wantUnpins := []storage.BlockID{leafRight, leaf, middleRight, middle}
	if len(tx.unpinCalls) != len(wantUnpins) {
		t.Fatalf("recursive split unpins = %v, want %v", tx.unpinCalls, wantUnpins)
	}
	for index := range wantUnpins {
		if tx.unpinCalls[index] != wantUnpins[index] {
			t.Errorf("recursive split unpin %d = %v, want %v", index, tx.unpinCalls[index], wantUnpins[index])
		}
	}
}

func newRecursiveDirectoryPage(t *testing.T, parentKeys, parentChildren []int32, parentLevel int32, childKeys, childChildren []int32, childLevel int32, capacity int) (*Page, *recursiveDirectoryTransaction, storage.BlockID, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertDirLayout(record.IntegerField)
	parentBlock := storage.NewBlockID("recursive-directory.idx", 0)
	childBlock := storage.NewBlockID("recursive-directory.idx", int(parentChildren[0]))
	base := newPageSplitTransaction(layout, capacity)
	tx := &recursiveDirectoryTransaction{pageSplitTransaction: base}
	setRecursiveDirectoryRows(tx.pageSplitTransaction, parentBlock, layout, parentLevel, parentKeys, parentChildren)
	setRecursiveDirectoryRows(tx.pageSplitTransaction, childBlock, layout, childLevel, childKeys, childChildren)
	page, err := NewPage(tx, parentBlock, layout)
	if err != nil {
		t.Fatalf("NewPage(%v) error = %v", parentBlock, err)
	}
	return page, tx, parentBlock, childBlock, layout
}

func TestPageInsertDirectoryEntryRecursiveInsertsAtLevelZero(t *testing.T) {
	page, tx, root, _, layout := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{100}, 0,
		[]int32{1}, []int32{2}, 0,
		4,
	)

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200))
	if err != nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want no separator and no error", separator, err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 20}, []int32{100, 200})
	if len(tx.pinCalls) != 1 || len(tx.unpinCalls) != 0 {
		t.Fatalf("level-zero insert pin lifecycle = pins %v, unpins %v", tx.pinCalls, tx.unpinCalls)
	}
}

func TestPageInsertDirectoryEntryRecursiveSplitsAtLevelZero(t *testing.T) {
	page, tx, root, _, layout := newRecursiveDirectoryPage(t,
		[]int32{10, 20, 30}, []int32{100, 200, 300}, 0,
		[]int32{1}, []int32{2}, 0,
		4,
	)
	right := storage.NewBlockID(root.FileName(), 8)
	tx.appendBlocks = []storage.BlockID{right}

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(40), 400))
	if err != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() error = %v", err)
	}
	if separator == nil || separator.BlockNumber() != right.Number() {
		t.Fatalf("InsertDirectoryEntryRecursive() separator = %v, want right block %v", separator, right)
	}
	if key, ok := separator.DataVal().AsInt(); !ok || key != 30 {
		t.Fatalf("separator key = %v, want 30", separator.DataVal())
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 20}, []int32{100, 200})
}

func TestPageInsertDirectoryEntryRecursiveRejectsInvalidPageAndLevel(t *testing.T) {
	if separator, err := (*Page)(nil).InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(1), 1)); err == nil || separator != nil {
		t.Fatalf("nil page InsertDirectoryEntryRecursive() = (%v, %v), want an error", separator, err)
	}

	page, _, _, _, _ := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, -1,
		[]int32{10}, []int32{100}, 0,
		4,
	)
	if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200)); err == nil || separator != nil {
		t.Fatalf("negative-level InsertDirectoryEntryRecursive() = (%v, %v), want an error", separator, err)
	}
	if err := page.Close(); err != nil {
		t.Fatalf("page.Close() error = %v", err)
	}
	if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200)); err == nil || separator != nil {
		t.Fatalf("closed-page InsertDirectoryEntryRecursive() = (%v, %v), want an error", separator, err)
	}
}

func TestPageInsertDirectoryEntryRecursiveWrapsLevelZeroInsertionError(t *testing.T) {
	page, _, _, _, _ := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{100}, 0,
		[]int32{1}, []int32{2}, 0,
		4,
	)
	if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), -1)); err == nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want nil separator and invalid child error", separator, err)
	}
}

func TestPageInsertDirectoryEntryRecursiveWrapsChildOpenError(t *testing.T) {
	page, tx, _, child, _ := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10}, []int32{100}, 0,
		4,
	)
	cause := errors.New("child pin failed")
	tx.pinErrorBlock = child
	tx.pinError = cause
	if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200)); !errors.Is(err, cause) || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want nil separator and wrapped %v", separator, err, cause)
	}
	if len(tx.unpinCalls) != 0 {
		t.Fatalf("failed child open unpinned blocks %v, want none", tx.unpinCalls)
	}
}

func TestPageInsertDirectoryEntryRecursiveClosesChildAfterRecursiveError(t *testing.T) {
	page, tx, _, child, _ := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10}, []int32{100}, -1,
		4,
	)
	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200))
	if err == nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want recursive error", separator, err)
	}
	if len(tx.unpinCalls) != 1 || tx.unpinCalls[0] != child {
		t.Fatalf("recursive-error child unpins = %v, want [%v]", tx.unpinCalls, child)
	}
}

func TestPageInsertDirectoryEntryRecursiveJoinsRecursiveAndCloseErrors(t *testing.T) {
	page, tx, _, child, _ := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10}, []int32{100}, -1,
		4,
	)
	closeCause := errors.New("child unpin failed")
	tx.unpinErrorBlock = child
	tx.unpinError = closeCause
	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200))
	if err == nil || separator != nil || !errors.Is(err, closeCause) || !strings.Contains(err.Error(), "directory level -1 is negative") {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want joined recursive and close errors", separator, err)
	}
}

func TestPageInsertDirectoryEntryRecursiveReturnsWithoutParentMutationWhenChildDoesNotSplit(t *testing.T) {
	page, tx, root, child, layout := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10}, []int32{100}, 0,
		4,
	)
	cause := errors.New("child unpin failed")
	tx.unpinErrorBlock = child
	tx.unpinError = cause
	if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(20), 200)); !errors.Is(err, cause) || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want wrapped close error", separator, err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10}, []int32{1})
}

func TestPageInsertDirectoryEntryRecursivePropagatesChildAndParentSplits(t *testing.T) {
	page, tx, root, child, layout := newRecursiveDirectoryPage(t,
		[]int32{10, 20, 30}, []int32{1, 1, 1}, 1,
		[]int32{10, 20, 30}, []int32{100, 200, 300}, 0,
		4,
	)
	childRight := storage.NewBlockID(root.FileName(), 8)
	parentRight := storage.NewBlockID(root.FileName(), 9)
	tx.appendBlocks = []storage.BlockID{childRight, parentRight}

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(40), 400))
	if err != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() error = %v", err)
	}
	if separator == nil || separator.BlockNumber() != parentRight.Number() {
		t.Fatalf("InsertDirectoryEntryRecursive() separator = %v, want parent block %v", separator, parentRight)
	}
	if key, ok := separator.DataVal().AsInt(); !ok || key != 30 {
		t.Fatalf("parent separator key = %v, want 30", separator.DataVal())
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 20}, []int32{1, 1})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, child, layout, []int32{10, 20}, []int32{100, 200})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, childRight, layout, []int32{30, 40}, []int32{300, 400})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, parentRight, layout, []int32{30, 30}, []int32{8, 1})
}

func TestPageInsertDirectoryEntryRecursivePropagatesChildSplitWithoutParentSplit(t *testing.T) {
	page, tx, root, child, layout := newRecursiveDirectoryPage(t,
		[]int32{10}, []int32{1}, 1,
		[]int32{10, 20, 30}, []int32{100, 200, 300}, 0,
		4,
	)
	childRight := storage.NewBlockID(root.FileName(), 8)
	tx.appendBlocks = []storage.BlockID{childRight}

	separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(40), 400))
	if err != nil || separator != nil {
		t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want no root separator and no error", separator, err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 30}, []int32{1, 8})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, child, layout, []int32{10, 20}, []int32{100, 200})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, childRight, layout, []int32{30, 40}, []int32{300, 400})
}

func TestPageInsertDirectoryEntryRecursivePreservesFailedParentMutationCauses(t *testing.T) {
	t.Run("parent initially full", func(t *testing.T) {
		page, tx, _, child, _ := newRecursiveDirectoryPage(t,
			[]int32{10, 20, 30, 40}, []int32{1, 1, 1, 1}, 1,
			[]int32{10, 20, 30}, []int32{100, 200, 300}, 0,
			4,
		)
		tx.appendBlocks = []storage.BlockID{storage.NewBlockID("recursive-directory.idx", 8)}
		if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(60), 600)); err == nil || separator != nil {
			t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want full-parent error", separator, err)
		}
		if len(tx.unpinCalls) != 2 || tx.unpinCalls[1] != child {
			t.Fatalf("full-parent child unpins = %v, want split destination then [%v]", tx.unpinCalls, child)
		}
	})

	t.Run("parent split append", func(t *testing.T) {
		page, tx, _, _, _ := newRecursiveDirectoryPage(t,
			[]int32{10, 20, 30}, []int32{1, 1, 1}, 1,
			[]int32{10, 20, 30}, []int32{100, 200, 300}, 0,
			4,
		)
		cause := errors.New("parent split append failed")
		tx.appendBlocks = []storage.BlockID{storage.NewBlockID("recursive-directory.idx", 8)}
		tx.appendErrorAt = 2
		tx.appendError = cause
		if separator, err := page.InsertDirectoryEntryRecursive(NewDirEntry(query.NewIntConstant(40), 400)); !errors.Is(err, cause) || separator != nil {
			t.Fatalf("InsertDirectoryEntryRecursive() = (%v, %v), want nil separator and wrapped %v", separator, err, cause)
		}
	})
}

func setRecursiveDirectoryRows(tx *pageSplitTransaction, block storage.BlockID, layout *record.Layout, level int32, keys, children []int32) {
	tx.ints[pageSplitLocation{block: block, offset: 0}] = level
	tx.ints[pageSplitLocation{block: block, offset: 4}] = int32(len(keys))
	for slot, key := range keys {
		tx.ints[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}] = key
		tx.ints[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "block")}] = children[slot]
	}
}

func assertRecursiveDirectoryRows(t *testing.T, tx *pageSplitTransaction, block storage.BlockID, layout *record.Layout, keys, children []int32) {
	t.Helper()
	if got := tx.ints[pageSplitLocation{block: block, offset: 4}]; got != int32(len(keys)) {
		t.Fatalf("block %v record count = %d, want %d", block, got, len(keys))
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
