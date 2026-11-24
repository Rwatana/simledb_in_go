package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestBTreeIndexDeleteRemovesRequestedRIDForSearchKey(t *testing.T) {
	index, transaction, root, leafBlock := newBTreeIndexInsertFixture(t, 128, [][2]int32{{10, 3}}, 3, [][3]int32{{10, 4, 7}, {20, 5, 8}, {20, 6, 9}, {30, 7, 1}})

	if err := index.Delete(query.NewIntConstant(20), record.NewRID(6, 9)); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 4}]; got != 3 {
		t.Fatalf("leaf record count = %d, want 3", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 0); got != record.NewRID(4, 7) {
		t.Fatalf("first RID = %v, want [4, 7]", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 1); got != record.NewRID(5, 8) {
		t.Fatalf("retained matching RID = %v, want [5, 8]", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 2); got != record.NewRID(7, 1) {
		t.Fatalf("shifted RID = %v, want [7, 1]", got)
	}
	if transaction.ints[pageFindLeafOffset{block: root, offset: 4}] != 1 || len(transaction.appendCalls) != 0 {
		t.Fatalf("delete changed directory: count=%d appends=%v", transaction.ints[pageFindLeafOffset{block: root, offset: 4}], transaction.appendCalls)
	}
	if index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("successful delete retained cursor/pages: leaf=%#v pending=%#v active=%v", index.leaf, index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexDeleteLeavesPageUnchangedWhenRIDDoesNotMatch(t *testing.T) {
	index, transaction, _, leafBlock := newBTreeIndexInsertFixture(t, 128, [][2]int32{{10, 3}}, 3, [][3]int32{{10, 4, 7}, {20, 5, 8}, {20, 6, 9}, {30, 7, 1}})

	if err := index.Delete(query.NewIntConstant(20), record.NewRID(88, 1)); err != nil {
		t.Fatalf("Delete() for a missing RID error = %v", err)
	}
	if got := transaction.ints[pageFindLeafOffset{block: leafBlock, offset: 4}]; got != 4 {
		t.Fatalf("leaf record count = %d, want 4", got)
	}
	if got := readBTreeIndexInsertRID(transaction, index.leafLayout, leafBlock, 2); got != record.NewRID(6, 9) {
		t.Fatalf("second matching RID = %v, want [6, 9]", got)
	}
	if index.leaf != nil || len(transaction.active) != 0 {
		t.Fatalf("missing-RID delete retained page: leaf=%#v active=%v", index.leaf, transaction.active)
	}
}

func TestBTreeIndexDeleteRejectsNilAndInvalidKeyBeforePageAccess(t *testing.T) {
	var nilIndex *BTreeIndex
	if err := nilIndex.Delete(query.NewIntConstant(20), record.NewRID(5, 8)); err == nil {
		t.Fatal("nil index Delete() error = nil, want error")
	}
	index, transaction, _, _ := newBTreeIndexInsertFixture(t, 64, [][2]int32{{10, 3}}, 3, nil)
	if err := index.Delete(query.NewStringConstant("wrong type"), record.NewRID(5, 8)); err == nil {
		t.Fatal("Delete() with a varchar key and integer layout returned nil error")
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("invalid key touched pages: pins=%v unpins=%v", transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexDeleteJoinsMutationAndCloseFailuresAndRetainsLeafForRetry(t *testing.T) {
	index, transaction, _, leafBlock := newBTreeIndexInsertFixture(t, 96, [][2]int32{{10, 3}}, 3, [][3]int32{{10, 4, 7}, {20, 5, 8}, {30, 7, 1}})
	deleteCause := errors.New("delete record count failed")
	closeCause := errors.New("leaf unpin failed")
	transaction.setIntFailures[btreeIndexInsertSetInt{block: leafBlock, offset: 4, value: 2}] = deleteCause
	transaction.unpinFailures[leafBlock] = map[int]error{1: closeCause}

	err := index.Delete(query.NewIntConstant(20), record.NewRID(5, 8))
	if !errors.Is(err, deleteCause) || !errors.Is(err, closeCause) {
		t.Fatalf("Delete() error = %v, want mutation and close causes", err)
	}
	if index.leaf == nil || index.leaf.contents == nil || len(transaction.active) != 1 {
		t.Fatalf("failed leaf cleanup lost retry state: leaf=%#v active=%v", index.leaf, transaction.active)
	}
	delete(transaction.setIntFailures, btreeIndexInsertSetInt{block: leafBlock, offset: 4, value: 2})
	delete(transaction.unpinFailures, leafBlock)
	if err := index.Close(); err != nil {
		t.Fatalf("Close() retry after delete error = %v", err)
	}
	if index.leaf != nil || len(transaction.active) != 0 {
		t.Fatalf("successful cleanup retry retained leaf: leaf=%#v active=%v", index.leaf, transaction.active)
	}
}

func TestBTreeIndexDeletePreservesPositioningFailure(t *testing.T) {
	index, transaction, root, _ := newBTreeIndexInsertFixture(t, 64, [][2]int32{{10, 3}}, 3, nil)
	cause := errors.New("root pin failed")
	transaction.pinFailures[root] = map[int]error{1: cause}

	if err := index.Delete(query.NewIntConstant(20), record.NewRID(5, 8)); !errors.Is(err, cause) {
		t.Fatalf("Delete() error = %v, want positioning cause", err)
	}
	if index.leaf != nil || len(transaction.active) != 0 {
		t.Fatalf("position failure retained leaf state: leaf=%#v active=%v", index.leaf, transaction.active)
	}
}
