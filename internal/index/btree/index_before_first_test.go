package btree

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBTreeIndexBeforeFirstDescendsAndPositionsImmediatelyBeforeMatch(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	leafBlock := storage.NewBlockID("studentsleaf", 3)
	transaction := newBTreeIndexSearchTransaction()
	setBTreeIndexSearchDirectory(transaction, directoryLayout, root, int32(-2147483648), 3)
	setBTreeIndexSearchLeaf(transaction, leafLayout, leafBlock, [3]int32{10, 4, 7}, [3]int32{20, 5, 8})
	index := &BTreeIndex{
		transaction:       transaction,
		leafFileName:      "studentsleaf",
		leafLayout:        leafLayout,
		directoryFileName: "studentsdir",
		directoryLayout:   directoryLayout,
		rootBlock:         root,
	}

	if err := index.BeforeFirst(query.NewIntConstant(20)); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if index.leaf == nil || index.leaf.contents.block != leafBlock || index.leaf.currentSlot != 0 {
		t.Fatalf("positioned leaf = %#v, want block %v immediately before matching slot 1", index.leaf, leafBlock)
	}
	if !reflect.DeepEqual(transaction.pins, []storage.BlockID{root, leafBlock}) || !reflect.DeepEqual(transaction.unpins, []storage.BlockID{root}) {
		t.Fatalf("pin lifecycle = pins %v/unpins %v, want directory then leaf with directory released", transaction.pins, transaction.unpins)
	}
	if transaction.maxActive != 1 {
		t.Fatalf("maximum active pages = %d, want 1", transaction.maxActive)
	}
	found, err := index.leaf.Next()
	if err != nil || !found {
		t.Fatalf("leaf.Next() = (%v, %v), want matching entry", found, err)
	}
	rid, err := index.leaf.GetDataRID()
	if err != nil || !rid.Equal(record.NewRID(5, 8)) {
		t.Fatalf("positioned RID = (%v, %v), want [5, 8]", rid, err)
	}
}

func TestBTreeIndexBeforeFirstClosesExistingLeafBeforeDirectoryPin(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	oldLeaf := storage.NewBlockID("studentsleaf", 1)
	newLeaf := storage.NewBlockID("studentsleaf", 2)
	transaction := newBTreeIndexSearchTransaction()
	setBTreeIndexSearchDirectory(transaction, directoryLayout, root, int32(-2147483648), 2)
	setBTreeIndexSearchLeaf(transaction, leafLayout, newLeaf, [3]int32{20, 6, 9})
	oldPage, err := NewPage(transaction, oldLeaf, leafLayout)
	if err != nil {
		t.Fatalf("NewPage(old leaf) error = %v", err)
	}
	index := &BTreeIndex{
		transaction:       transaction,
		leafFileName:      "studentsleaf",
		leafLayout:        leafLayout,
		directoryFileName: "studentsdir",
		directoryLayout:   directoryLayout,
		rootBlock:         root,
		leaf:              &Leaf{contents: oldPage, searchKey: query.NewIntConstant(10), currentSlot: -1},
	}

	if err := index.BeforeFirst(query.NewIntConstant(20)); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if !reflect.DeepEqual(transaction.unpins, []storage.BlockID{oldLeaf, root}) || !reflect.DeepEqual(transaction.pins, []storage.BlockID{oldLeaf, root, newLeaf}) {
		t.Fatalf("pin order = pins %v/unpins %v, want old leaf closed before root and selected leaf", transaction.pins, transaction.unpins)
	}
	if transaction.maxActive != 1 {
		t.Fatalf("maximum active pages = %d, want 1", transaction.maxActive)
	}
}

func TestBTreeIndexBeforeFirstPositionsVarcharLeaf(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.VarcharField)
	leafLayout := pageInsertLeafLayout(record.VarcharField)
	root := storage.NewBlockID("namesdir", 0)
	leafBlock := storage.NewBlockID("namesleaf", 3)
	transaction := newBTreeIndexSearchTransaction()
	transaction.ints[pageFindLeafOffset{block: root, offset: 0}] = 0
	transaction.ints[pageFindLeafOffset{block: root, offset: 4}] = 1
	transaction.strings[pageFindLeafOffset{block: root, offset: pageInsertDirPosition(directoryLayout, 0, "dataval")}] = ""
	transaction.ints[pageFindLeafOffset{block: root, offset: pageInsertDirPosition(directoryLayout, 0, "block")}] = 3
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 0}] = -1
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 4}] = 2
	transaction.strings[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 0, "dataval")}] = "alpha"
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 0, "block")}] = 4
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 0, "id")}] = 7
	transaction.strings[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 1, "dataval")}] = "beta"
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 1, "block")}] = 5
	transaction.ints[pageFindLeafOffset{block: leafBlock, offset: pageInsertLeafPosition(leafLayout, 1, "id")}] = 8
	index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
	index.leafFileName = "namesleaf"
	index.directoryFileName = "namesdir"

	if err := index.BeforeFirst(query.NewStringConstant("beta")); err != nil {
		t.Fatalf("BeforeFirst(varchar) error = %v", err)
	}
	if index.leaf == nil || index.leaf.contents.block != leafBlock || index.leaf.currentSlot != 0 {
		t.Fatalf("varchar position = %#v, want selected leaf before slot 1", index.leaf)
	}
}

func TestBTreeIndexBeforeFirstReleasesDirectoryAfterSearchFailure(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	cause := errors.New("directory level read failed")
	transaction := newBTreeIndexSearchTransaction()
	transaction.intErrors[pageFindLeafOffset{block: root, offset: 0}] = cause
	index := &BTreeIndex{
		transaction:       transaction,
		leafFileName:      "studentsleaf",
		leafLayout:        leafLayout,
		directoryFileName: "studentsdir",
		directoryLayout:   directoryLayout,
		rootBlock:         root,
	}

	err := index.BeforeFirst(query.NewIntConstant(20))
	if !errors.Is(err, cause) {
		t.Fatalf("BeforeFirst() error = %v, want wrapped directory read failure", err)
	}
	if index.leaf != nil || len(transaction.active) != 0 || !reflect.DeepEqual(transaction.unpins, []storage.BlockID{root}) {
		t.Fatalf("failed search retained cursor/pins: leaf=%#v active=%v unpins=%v", index.leaf, transaction.active, transaction.unpins)
	}
}

func TestBTreeIndexBeforeFirstRetainsDirectoryWhenFailureCleanupNeedsRetry(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	readCause := errors.New("directory level read failed")
	unpinCause := errors.New("directory unpin failed once")
	transaction := newBTreeIndexSearchTransaction()
	transaction.intErrors[pageFindLeafOffset{block: root, offset: 0}] = readCause
	transaction.unpinErrors[root] = unpinCause
	index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)

	err := index.BeforeFirst(query.NewIntConstant(20))
	if !errors.Is(err, readCause) || !errors.Is(err, unpinCause) {
		t.Fatalf("BeforeFirst() error = %v, want read and cleanup causes", err)
	}
	if index.pendingDirectory == nil || index.pendingDirectory.block != root || len(transaction.active) != 1 {
		t.Fatalf("pending directory = %#v active=%v, want retained root pin", index.pendingDirectory, transaction.active)
	}

	delete(transaction.intErrors, pageFindLeafOffset{block: root, offset: 0})
	setBTreeIndexSearchDirectory(transaction, directoryLayout, root, -2147483648, 3)
	leafBlock := storage.NewBlockID("studentsleaf", 3)
	setBTreeIndexSearchLeaf(transaction, leafLayout, leafBlock, [3]int32{20, 5, 8})
	if err := index.BeforeFirst(query.NewIntConstant(20)); err != nil {
		t.Fatalf("BeforeFirst() retry error = %v", err)
	}
	if index.pendingDirectory != nil || index.leaf == nil || index.leaf.contents.block != leafBlock {
		t.Fatalf("retry state = pending %v leaf %#v, want cleared directory and selected leaf", index.pendingDirectory, index.leaf)
	}
}

func TestBTreeIndexBeforeFirstRetriesPendingDirectoryClose(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	cause := errors.New("pending directory unpin failed once")
	transaction := newBTreeIndexSearchTransaction()
	pendingDirectory, err := NewPage(transaction, root, directoryLayout)
	if err != nil {
		t.Fatalf("NewPage(pending directory) error = %v", err)
	}
	transaction.unpinErrors[root] = cause
	index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
	index.pendingDirectory = pendingDirectory

	if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) {
		t.Fatalf("BeforeFirst() error = %v, want wrapped pending-directory unpin error", err)
	}
	if index.pendingDirectory != pendingDirectory || len(transaction.pins) != 1 || len(transaction.unpins) != 1 {
		t.Fatalf("failed pending close changed ownership: pending=%#v pins=%v unpins=%v", index.pendingDirectory, transaction.pins, transaction.unpins)
	}
	if err := index.BeforeFirst(query.NewIntConstant(20)); err == nil {
		t.Fatal("BeforeFirst() after retry error = nil, want empty root error")
	}
	if index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("successful pending close retry retained page: pending=%#v active=%v", index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexBeforeFirstClosesSelectedPagesOnPositionAndPinErrors(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	leafBlock := storage.NewBlockID("studentsleaf", 3)
	t.Run("root pin", func(t *testing.T) {
		cause := errors.New("root pin failed")
		transaction := newBTreeIndexSearchTransaction()
		transaction.pinErrors[root] = cause
		index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
		if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) {
			t.Fatalf("BeforeFirst() error = %v, want wrapped root pin cause", err)
		}
		if len(transaction.active) != 0 || index.leaf != nil {
			t.Fatalf("root pin error retained state: active=%v leaf=%#v", transaction.active, index.leaf)
		}
	})
	t.Run("selected leaf pin", func(t *testing.T) {
		cause := errors.New("leaf pin failed")
		transaction := newBTreeIndexSearchTransaction()
		setBTreeIndexSearchDirectory(transaction, directoryLayout, root, -2147483648, 3)
		transaction.pinErrors[leafBlock] = cause
		index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
		if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) {
			t.Fatalf("BeforeFirst() error = %v, want wrapped leaf pin cause", err)
		}
		if index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 || !reflect.DeepEqual(transaction.unpins, []storage.BlockID{root}) {
			t.Fatalf("leaf pin error retained state: leaf=%#v pending=%#v active=%v unpins=%v", index.leaf, index.pendingDirectory, transaction.active, transaction.unpins)
		}
	})
	t.Run("leaf positioning", func(t *testing.T) {
		cause := errors.New("leaf count read failed")
		transaction := newBTreeIndexSearchTransaction()
		setBTreeIndexSearchDirectory(transaction, directoryLayout, root, -2147483648, 3)
		transaction.intErrors[pageFindLeafOffset{block: leafBlock, offset: 4}] = cause
		index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
		if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) {
			t.Fatalf("BeforeFirst() error = %v, want wrapped leaf positioning cause", err)
		}
		if index.leaf != nil || len(transaction.active) != 0 || !reflect.DeepEqual(transaction.unpins, []storage.BlockID{root, leafBlock}) {
			t.Fatalf("leaf positioning error retained state: leaf=%#v active=%v unpins=%v", index.leaf, transaction.active, transaction.unpins)
		}
	})
	t.Run("leaf cleanup retry", func(t *testing.T) {
		cause := errors.New("leaf count read failed")
		unpinCause := errors.New("leaf unpin failed once")
		transaction := newBTreeIndexSearchTransaction()
		setBTreeIndexSearchDirectory(transaction, directoryLayout, root, -2147483648, 3)
		transaction.intErrors[pageFindLeafOffset{block: leafBlock, offset: 4}] = cause
		transaction.unpinErrors[leafBlock] = unpinCause
		index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
		if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) || !errors.Is(err, unpinCause) {
			t.Fatalf("BeforeFirst() error = %v, want position and cleanup causes", err)
		}
		if index.leaf == nil || index.leaf.contents.block != leafBlock || len(transaction.active) != 1 {
			t.Fatalf("failed cleanup lost its retryable leaf: leaf=%#v active=%v", index.leaf, transaction.active)
		}
		delete(transaction.intErrors, pageFindLeafOffset{block: leafBlock, offset: 4})
		if err := index.BeforeFirst(query.NewIntConstant(20)); err != nil {
			t.Fatalf("BeforeFirst() retry error = %v", err)
		}
		if index.leaf == nil || index.leaf.contents.block != leafBlock || len(transaction.active) != 1 {
			t.Fatalf("retry leaf = %#v active=%v, want selected leaf only", index.leaf, transaction.active)
		}
	})
}

func TestBTreeIndexBeforeFirstCleansDirectoryWhenLeafPageValidationFails(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	for _, test := range []struct {
		name            string
		unpinError      error
		wantPendingPage bool
	}{
		{name: "directory cleanup succeeds"},
		{name: "directory cleanup can be retried", unpinError: errors.New("directory unpin failed once"), wantPendingPage: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			levelError := errors.New("directory level recheck failed")
			base := newBTreeIndexSearchTransaction()
			setBTreeIndexSearchDirectory(base, directoryLayout, root, -2147483648, 3)
			transaction := &btreeIndexOpenLeafFailureTransaction{
				btreeIndexSearchTransaction: base,
				root:                        root,
				failAt:                      2,
				failure:                     levelError,
			}
			transaction.unpinErrors[root] = test.unpinError
			index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
			if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, levelError) {
				t.Fatalf("BeforeFirst() error = %v, want wrapped directory level error", err)
			} else if test.unpinError != nil && !errors.Is(err, test.unpinError) {
				t.Fatalf("BeforeFirst() error = %v, want joined directory cleanup error", err)
			}
			if index.leaf != nil || len(transaction.active) != 0 && !test.wantPendingPage {
				t.Fatalf("leaf validation failure retained unexpected state: leaf=%#v active=%v", index.leaf, transaction.active)
			}
			if test.wantPendingPage && (index.pendingDirectory == nil || index.pendingDirectory.block != root) {
				t.Fatalf("pending directory = %#v, want failed root cleanup retained for retry", index.pendingDirectory)
			}
			if !test.wantPendingPage && index.pendingDirectory != nil {
				t.Fatalf("pending directory = %#v, want none after cleanup", index.pendingDirectory)
			}
		})
	}
}

func TestBTreeIndexBeforeFirstPreservesExistingLeafWhenCloseFails(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	oldLeaf := storage.NewBlockID("studentsleaf", 1)
	cause := errors.New("old leaf unpin failed once")
	transaction := newBTreeIndexSearchTransaction()
	transaction.unpinErrors[oldLeaf] = cause
	oldPage, err := NewPage(transaction, oldLeaf, leafLayout)
	if err != nil {
		t.Fatalf("NewPage(old leaf) error = %v", err)
	}
	oldCursor := &Leaf{contents: oldPage, searchKey: query.NewIntConstant(10), currentSlot: -1}
	index := newBTreeIndexSearchIndex(transaction, directoryLayout, leafLayout, root)
	index.leaf = oldCursor

	if err := index.BeforeFirst(query.NewIntConstant(20)); !errors.Is(err, cause) {
		t.Fatalf("BeforeFirst() error = %v, want wrapped old-leaf close error", err)
	}
	if index.leaf != oldCursor || index.leaf.contents != oldPage || len(transaction.pins) != 1 || len(transaction.unpins) != 1 {
		t.Fatalf("failed close changed ownership: leaf=%#v pins=%v unpins=%v", index.leaf, transaction.pins, transaction.unpins)
	}
	if err := index.BeforeFirst(query.NewIntConstant(20)); err == nil {
		t.Fatal("BeforeFirst() retry error = nil, want valid root search data missing")
	}
	if index.leaf != nil || transaction.active[oldLeaf] {
		t.Fatalf("successful close retry retained old leaf: leaf=%#v active=%v", index.leaf, transaction.active)
	}
}

func TestBTreeIndexBeforeFirstRejectsUnsetAndIncompatibleKeysBeforePinning(t *testing.T) {
	for _, test := range []struct {
		name      string
		keyType   record.FieldType
		searchKey query.Constant
	}{
		{name: "unset", keyType: record.IntegerField},
		{name: "integer for varchar", keyType: record.VarcharField, searchKey: query.NewIntConstant(1)},
		{name: "varchar for integer", keyType: record.IntegerField, searchKey: query.NewStringConstant("1")},
	} {
		t.Run(test.name, func(t *testing.T) {
			directoryLayout, err := directoryLayoutFromLeaf(pageInsertLeafLayout(test.keyType))
			if err != nil {
				t.Fatalf("directoryLayoutFromLeaf() error = %v", err)
			}
			transaction := newBTreeIndexSearchTransaction()
			index := &BTreeIndex{
				transaction:       transaction,
				leafFileName:      "studentsleaf",
				leafLayout:        pageInsertLeafLayout(test.keyType),
				directoryFileName: "studentsdir",
				directoryLayout:   directoryLayout,
				rootBlock:         storage.NewBlockID("studentsdir", 0),
			}

			if err := index.BeforeFirst(test.searchKey); err == nil {
				t.Fatal("BeforeFirst() error = nil, want invalid search-key error")
			}
			if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
				t.Fatalf("invalid key touched pages: pins %v/unpins %v", transaction.pins, transaction.unpins)
			}
		})
	}
}

func TestBTreeIndexBeforeFirstRejectsInvalidIndexStateBeforePinning(t *testing.T) {
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	var typedNil *btreeIndexSearchTransaction
	tests := []struct {
		name  string
		index *BTreeIndex
	}{
		{name: "nil index"},
		{name: "nil transaction", index: newBTreeIndexSearchIndex(nil, directoryLayout, leafLayout, root)},
		{name: "typed nil transaction", index: newBTreeIndexSearchIndex(typedNil, directoryLayout, leafLayout, root)},
		{name: "empty leaf filename", index: func() *BTreeIndex {
			i := newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, leafLayout, root)
			i.leafFileName = " "
			return i
		}()},
		{name: "empty directory filename", index: func() *BTreeIndex {
			i := newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, leafLayout, root)
			i.directoryFileName = " "
			return i
		}()},
		{name: "wrong root filename", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, leafLayout, storage.NewBlockID("otherdir", 0))},
		{name: "nonzero root block", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, leafLayout, storage.NewBlockID("studentsdir", 1))},
		{name: "nil leaf layout", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, nil, root)},
		{name: "invalid leaf layout", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, newLeafLayoutWithoutDataval(), root)},
		{name: "invalid directory layout", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), leafLayout, leafLayout, root)},
		{name: "key type mismatch", index: newBTreeIndexSearchIndex(newBTreeIndexSearchTransaction(), directoryLayout, pageInsertLeafLayout(record.VarcharField), root)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.index.BeforeFirst(query.NewIntConstant(20)); err == nil {
				t.Fatal("BeforeFirst() error = nil, want invalid index state error")
			}
			if test.index != nil {
				if tx, ok := test.index.transaction.(*btreeIndexSearchTransaction); ok && tx != nil && (len(tx.pins) != 0 || len(tx.unpins) != 0) {
					t.Fatalf("invalid state touched pages: pins %v/unpins %v", tx.pins, tx.unpins)
				}
			}
		})
	}
}

type btreeIndexSearchTransaction struct {
	*pageFindLeafTransaction
}

type btreeIndexOpenLeafFailureTransaction struct {
	*btreeIndexSearchTransaction
	root      storage.BlockID
	levelRead int
	failAt    int
	failure   error
}

func (transaction *btreeIndexOpenLeafFailureTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	if block == transaction.root && offset == 0 {
		transaction.levelRead++
		if transaction.levelRead == transaction.failAt {
			return 0, transaction.failure
		}
	}
	return transaction.pageFindLeafTransaction.GetInt(block, offset)
}

func newBTreeIndexSearchTransaction() *btreeIndexSearchTransaction {
	return &btreeIndexSearchTransaction{pageFindLeafTransaction: newPageFindLeafTransaction()}
}

func newBTreeIndexSearchIndex(transaction btreeFileTransaction, directoryLayout, leafLayout *record.Layout, root storage.BlockID) *BTreeIndex {
	return &BTreeIndex{
		transaction:       transaction,
		leafFileName:      "studentsleaf",
		leafLayout:        leafLayout,
		directoryFileName: "studentsdir",
		directoryLayout:   directoryLayout,
		rootBlock:         root,
	}
}

func (*btreeIndexSearchTransaction) Size(string) (int, error) { return 1, nil }

func (*btreeIndexSearchTransaction) Append(string) (storage.BlockID, error) {
	return storage.BlockID{}, errors.New("unexpected append")
}

func setBTreeIndexSearchDirectory(transaction *btreeIndexSearchTransaction, layout *record.Layout, block storage.BlockID, key, child int32) {
	transaction.ints[pageFindLeafOffset{block: block, offset: 0}] = 0
	transaction.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	transaction.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = key
	transaction.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "block")}] = child
}

func setBTreeIndexSearchLeaf(transaction *btreeIndexSearchTransaction, layout *record.Layout, block storage.BlockID, entries ...[3]int32) {
	transaction.ints[pageFindLeafOffset{block: block, offset: 0}] = -1
	transaction.ints[pageFindLeafOffset{block: block, offset: 4}] = int32(len(entries))
	for slot, entry := range entries {
		transaction.ints[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, slot, "dataval")}] = entry[0]
		transaction.ints[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, slot, "block")}] = entry[1]
		transaction.ints[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, slot, "id")}] = entry[2]
	}
}

var _ btreeFileTransaction = (*btreeIndexSearchTransaction)(nil)
var _ btreeFileTransaction = (*btreeIndexOpenLeafFailureTransaction)(nil)
