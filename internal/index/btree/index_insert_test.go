package btree

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBTreeIndexInsertAddsEntryWithoutDirectoryChangesWhenLeafDoesNotSplit(t *testing.T) {
	index, transaction, root, leafBlock := newBTreeIndexInsertFixture(t, 80, [][2]int32{{-2147483648, 3}}, 3, [][3]int32{{10, 4, 7}, {20, 5, 8}})
	wantRID := record.NewRID(5, 9)

	if err := index.Insert(query.NewIntConstant(20), wantRID); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if got := transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 4}]; got != 3 {
		t.Fatalf("leaf record count = %d, want 3", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 1); got != wantRID {
		t.Fatalf("inserted RID = %v, want %v", got, wantRID)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 2); got != record.NewRID(5, 8) {
		t.Fatalf("shifted RID = %v, want original RID [5, 8]", got)
	}
	if transaction.ints[pageFindLeafOffset{block: root, offset: 4}] != 1 || len(transaction.appendCalls) != 0 {
		t.Fatalf("non-split insert changed directory: count=%d appends=%v", transaction.ints[pageFindLeafOffset{block: root, offset: 4}], transaction.appendCalls)
	}
	if index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("successful insert retained cursor/pages: leaf=%#v pending=%#v active=%v", index.leaf, index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexInsertPropagatesLeafSplitAndGrowsBlockZeroRoot(t *testing.T) {
	index, transaction, root, leafBlock := newBTreeIndexInsertFixture(t, 44, [][2]int32{{-2147483648, 0}, {20, 1}}, 1, [][3]int32{{20, 5, 8}})
	wantRID := record.NewRID(7, 9)

	if err := index.Insert(query.NewIntConstant(30), wantRID); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	leafRight := storage.NewBlockID("studentsleaf", 2)
	directoryRight := storage.NewBlockID("studentsdir", 1)
	directoryMoved := storage.NewBlockID("studentsdir", 2)
	wantAppends := []storage.BlockID{leafRight, directoryRight, directoryMoved}
	if !reflect.DeepEqual(transaction.appendCalls, wantAppends) {
		t.Fatalf("append calls = %v, want leaf split then directory split and root move %v", transaction.appendCalls, wantAppends)
	}
	if transaction.ints[pageFindLeafOffset{block: root, offset: 0}] != 1 || transaction.ints[pageFindLeafOffset{block: root, offset: 4}] != 2 {
		t.Fatalf("grown root level/count = %d/%d, want 1/2", transaction.ints[pageFindLeafOffset{block: root, offset: 0}], transaction.ints[pageFindLeafOffset{block: root, offset: 4}])
	}
	if got := transaction.ints[pageFindLeafOffset{block: root, offset: pageInsertDirPosition(index.directoryLayout, 0, "block")}]; got != 2 {
		t.Fatalf("root left child block = %d, want moved block 2", got)
	}
	if got := transaction.ints[pageFindLeafOffset{block: root, offset: pageInsertDirPosition(index.directoryLayout, 1, "block")}]; got != 1 {
		t.Fatalf("root right child block = %d, want split block 1", got)
	}
	if got := transaction.ints[pageFindLeafOffset{block: directoryRight, offset: 4}]; got != 2 {
		t.Fatalf("right directory record count = %d, want 2", got)
	}
	if got := transaction.ints[pageFindLeafOffset{block: directoryRight, offset: pageInsertDirPosition(index.directoryLayout, 1, "block")}]; got != 2 {
		t.Fatalf("right directory leaf child = %d, want new leaf block 2", got)
	}
	if got := transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 4}]; got != 1 {
		t.Fatalf("left leaf record count = %d, want 1 after split", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafRight, 0); got != wantRID {
		t.Fatalf("new right leaf RID = %v, want %v", got, wantRID)
	}
	if index.rootBlock != root || index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("root growth changed ownership/state: root=%v leaf=%#v pending=%#v active=%v", index.rootBlock, index.leaf, index.pendingDirectory, transaction.active)
	}
	if transaction.maxActive > 2 {
		t.Fatalf("root growth pinned %d pages simultaneously, want at most two", transaction.maxActive)
	}
}

func TestBTreeIndexInsertRejectsNilAndInvalidKeysBeforePageAccess(t *testing.T) {
	var nilIndex *BTreeIndex
	if err := nilIndex.Insert(query.NewIntConstant(1), record.NewRID(0, 0)); err == nil {
		t.Fatal("nil index Insert() error = nil, want error")
	}
	index, transaction, _, _ := newBTreeIndexInsertFixture(t, 64, [][2]int32{{-2147483648, 0}}, 0, nil)
	if err := index.Insert(query.NewStringConstant("wrong type"), record.NewRID(0, 0)); err == nil {
		t.Fatal("Insert() with a varchar key and integer layout returned nil error")
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("invalid key touched pages: pins=%v unpins=%v", transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexInsertJoinsLeafWriteAndCloseErrors(t *testing.T) {
	index, transaction, _, leafBlock := newBTreeIndexInsertFixture(t, 64, [][2]int32{{-2147483648, 3}}, 3, [][3]int32{{10, 4, 7}})
	unpinCause := errors.New("leaf unpin failed")
	transaction.unpinFailures[leafBlock] = map[int]error{1: unpinCause}
	largeRID := record.NewRID(int(1<<31), 0)

	err := index.Insert(query.NewIntConstant(20), largeRID)
	if err == nil || !errors.Is(err, unpinCause) {
		t.Fatalf("Insert() error = %v, want leaf cleanup cause", err)
	}
	if index.leaf == nil || index.leaf.contents == nil || len(transaction.active) != 1 {
		t.Fatalf("failed leaf cleanup lost retry state: leaf=%#v active=%v", index.leaf, transaction.active)
	}
	if !strings.Contains(err.Error(), "RID") {
		t.Fatalf("Insert() error = %v, want the preceding RID write failure", err)
	}
	delete(transaction.unpinFailures, leafBlock)
	if err := index.Close(); err != nil {
		t.Fatalf("Close() retry after insert failure = %v", err)
	}
}

func TestBTreeIndexInsertRetainsDirectoryAfterSplitAndCloseErrors(t *testing.T) {
	index, transaction, root, _ := newBTreeIndexInsertFixture(t, 44, [][2]int32{{-2147483648, 0}, {20, 1}}, 1, [][3]int32{{20, 5, 8}})
	appendCause := errors.New("directory split append failed")
	closeCause := errors.New("directory root unpin failed")
	transaction.appendFailures["studentsdir"] = appendCause
	transaction.unpinFailures[root] = map[int]error{2: closeCause}

	err := index.Insert(query.NewIntConstant(30), record.NewRID(7, 9))
	if !errors.Is(err, appendCause) || !errors.Is(err, closeCause) {
		t.Fatalf("Insert() error = %v, want split and root close causes", err)
	}
	if index.pendingDirectory == nil || index.pendingDirectory.block != root || len(transaction.active) != 1 {
		t.Fatalf("failed directory close lost retry state: pending=%#v active=%v", index.pendingDirectory, transaction.active)
	}
	delete(transaction.appendFailures, "studentsdir")
	if err := index.Close(); err != nil {
		t.Fatalf("Close() retry error = %v, want successful cleanup", err)
	}
	if index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("successful cleanup retry retained page: pending=%#v active=%v", index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexInsertPreservesRootPinFailureAfterLeafSplit(t *testing.T) {
	index, transaction, root, _ := newBTreeIndexInsertFixture(t, 44, [][2]int32{{-2147483648, 0}, {20, 1}}, 1, [][3]int32{{20, 5, 8}})
	cause := errors.New("directory root repin failed")
	transaction.pinFailures[root] = map[int]error{2: cause}

	err := index.Insert(query.NewIntConstant(30), record.NewRID(7, 9))
	if !errors.Is(err, cause) {
		t.Fatalf("Insert() error = %v, want root repin cause", err)
	}
	if index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("root pin failure leaked retained pages: leaf=%#v pending=%#v active=%v", index.leaf, index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexInsertReturnsRootGrowthFailure(t *testing.T) {
	index, transaction, root, _ := newBTreeIndexInsertFixture(t, 44, [][2]int32{{-2147483648, 0}, {20, 1}}, 1, [][3]int32{{20, 5, 8}})
	cause := errors.New("raise directory root level failed")
	transaction.setIntFailures[btreeIndexInsertSetInt{block: root, offset: 0, value: 1}] = cause

	err := index.Insert(query.NewIntConstant(30), record.NewRID(7, 9))
	if !errors.Is(err, cause) {
		t.Fatalf("Insert() error = %v, want root growth failure", err)
	}
	if index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("root growth error leaked directory pages: pending=%#v active=%v", index.pendingDirectory, transaction.active)
	}
}

type btreeIndexInsertTransaction struct {
	*btreeIndexSearchTransaction
	blockSize      int
	fileSizes      map[string]int
	appendFailures map[string]error
	appendCalls    []storage.BlockID
	pinCalls       map[storage.BlockID]int
	pinFailures    map[storage.BlockID]map[int]error
	unpinCalls     map[storage.BlockID]int
	unpinFailures  map[storage.BlockID]map[int]error
	setIntFailures map[btreeIndexInsertSetInt]error
}

type btreeIndexInsertSetInt struct {
	block  storage.BlockID
	offset int32
	value  int32
}

func newBTreeIndexInsertFixture(t *testing.T, blockSize int, directoryEntries [][2]int32, leafBlockNumber int, leafEntries [][3]int32) (*BTreeIndex, *btreeIndexInsertTransaction, storage.BlockID, storage.BlockID) {
	t.Helper()
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	root := storage.NewBlockID("studentsdir", 0)
	leafBlock := storage.NewBlockID("studentsleaf", leafBlockNumber)
	transaction := &btreeIndexInsertTransaction{
		btreeIndexSearchTransaction: newBTreeIndexSearchTransaction(),
		blockSize:                   blockSize,
		fileSizes:                   map[string]int{"studentsdir": 1, "studentsleaf": leafBlockNumber + 1},
		appendFailures:              make(map[string]error),
		pinCalls:                    make(map[storage.BlockID]int),
		pinFailures:                 make(map[storage.BlockID]map[int]error),
		unpinCalls:                  make(map[storage.BlockID]int),
		unpinFailures:               make(map[storage.BlockID]map[int]error),
		setIntFailures:              make(map[btreeIndexInsertSetInt]error),
	}
	setBTreeIndexInsertDirectory(transaction, directoryLayout, root, 0, directoryEntries)
	entries := make([][3]int32, len(leafEntries))
	copy(entries, leafEntries)
	setBTreeIndexSearchLeaf(transaction.btreeIndexSearchTransaction, leafLayout, leafBlock, entries...)
	index := &BTreeIndex{
		transaction:       transaction,
		indexName:         "students",
		leafFileName:      "studentsleaf",
		leafLayout:        leafLayout,
		directoryFileName: "studentsdir",
		directoryLayout:   directoryLayout,
		rootBlock:         root,
	}
	return index, transaction, root, leafBlock
}

func setBTreeIndexInsertDirectory(transaction *btreeIndexInsertTransaction, layout *record.Layout, block storage.BlockID, level int32, entries [][2]int32) {
	base := transaction.btreeIndexSearchTransaction.pageFindLeafTransaction
	base.ints[pageFindLeafOffset{block: block, offset: 0}] = level
	base.ints[pageFindLeafOffset{block: block, offset: 4}] = int32(len(entries))
	for slot, entry := range entries {
		base.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}] = entry[0]
		base.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, "block")}] = entry[1]
	}
}

func readBTreeIndexInsertRID(transaction *btreeIndexInsertTransaction, layout *record.Layout, block storage.BlockID, slot int) record.RID {
	base := transaction.btreeIndexSearchTransaction.pageFindLeafTransaction
	blockNumber := base.ints[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, slot, "block")}]
	id := base.ints[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, slot, "id")}]
	return record.NewRID(int(blockNumber), int(id))
}

func (transaction *btreeIndexInsertTransaction) BlockSize() (int, error) {
	return transaction.blockSize, nil
}

func (transaction *btreeIndexInsertTransaction) Size(fileName string) (int, error) {
	return transaction.fileSizes[fileName], nil
}

func (transaction *btreeIndexInsertTransaction) Append(fileName string) (storage.BlockID, error) {
	if err := transaction.appendFailures[fileName]; err != nil {
		return storage.BlockID{}, err
	}
	block := storage.NewBlockID(fileName, transaction.fileSizes[fileName])
	transaction.fileSizes[fileName]++
	transaction.appendCalls = append(transaction.appendCalls, block)
	return block, nil
}

func (transaction *btreeIndexInsertTransaction) Pin(block storage.BlockID) error {
	transaction.pinCalls[block]++
	if err := transaction.pinFailures[block][transaction.pinCalls[block]]; err != nil {
		return err
	}
	return transaction.btreeIndexSearchTransaction.pageFindLeafTransaction.Pin(block)
}

func (transaction *btreeIndexInsertTransaction) Unpin(block storage.BlockID) error {
	transaction.unpinCalls[block]++
	if err := transaction.unpinFailures[block][transaction.unpinCalls[block]]; err != nil {
		return err
	}
	return transaction.btreeIndexSearchTransaction.pageFindLeafTransaction.Unpin(block)
}

func (transaction *btreeIndexInsertTransaction) SetInt(block storage.BlockID, offset int32, value int32, logged bool) error {
	if err := transaction.setIntFailures[btreeIndexInsertSetInt{block: block, offset: offset, value: value}]; err != nil {
		return err
	}
	return transaction.btreeIndexSearchTransaction.pageFindLeafTransaction.SetInt(block, offset, value, logged)
}

var _ btreeFileTransaction = (*btreeIndexInsertTransaction)(nil)
