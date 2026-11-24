package btree

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBTreeIndexCloseReleasesLeafThenPendingDirectoryAndIsIdempotent(t *testing.T) {
	leafBlock := storage.NewBlockID("studentsleaf", 3)
	directoryBlock := storage.NewBlockID("studentsdir", 0)
	transaction := newBTreeIndexSearchTransaction()
	leafPage, err := NewPage(transaction, leafBlock, pageInsertLeafLayout(record.IntegerField))
	if err != nil {
		t.Fatalf("NewPage(leaf) error = %v", err)
	}
	directoryPage, err := NewPage(transaction, directoryBlock, pageInsertDirLayout(record.IntegerField))
	if err != nil {
		t.Fatalf("NewPage(directory) error = %v", err)
	}
	index := &BTreeIndex{
		leaf:             &Leaf{contents: leafPage, searchKey: query.NewIntConstant(20), currentSlot: 0},
		pendingDirectory: directoryPage,
	}

	if err := index.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if index.leaf != nil || index.pendingDirectory != nil {
		t.Fatalf("Close() retained pages: leaf=%#v pending=%#v", index.leaf, index.pendingDirectory)
	}
	if !reflect.DeepEqual(transaction.unpins, []storage.BlockID{leafBlock, directoryBlock}) {
		t.Fatalf("unpin order = %v, want leaf then pending directory", transaction.unpins)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
	if len(transaction.unpins) != 2 {
		t.Fatalf("repeated Close() unpinned again: %v", transaction.unpins)
	}
}

func TestBTreeIndexCloseJoinsFailuresAndRetainsPagesForRetry(t *testing.T) {
	leafBlock := storage.NewBlockID("studentsleaf", 3)
	directoryBlock := storage.NewBlockID("studentsdir", 0)
	leafCause := errors.New("leaf unpin failed")
	directoryCause := errors.New("directory unpin failed")
	transaction := newBTreeIndexSearchTransaction()
	leafPage, err := NewPage(transaction, leafBlock, pageInsertLeafLayout(record.IntegerField))
	if err != nil {
		t.Fatalf("NewPage(leaf) error = %v", err)
	}
	directoryPage, err := NewPage(transaction, directoryBlock, pageInsertDirLayout(record.IntegerField))
	if err != nil {
		t.Fatalf("NewPage(directory) error = %v", err)
	}
	transaction.unpinErrors[leafBlock] = leafCause
	transaction.unpinErrors[directoryBlock] = directoryCause
	index := &BTreeIndex{
		leaf:             &Leaf{contents: leafPage, searchKey: query.NewIntConstant(20), currentSlot: 0},
		pendingDirectory: directoryPage,
	}

	err = index.Close()
	if !errors.Is(err, leafCause) || !errors.Is(err, directoryCause) {
		t.Fatalf("Close() error = %v, want both unpin causes", err)
	}
	if index.leaf == nil || index.leaf.contents != leafPage || index.pendingDirectory != directoryPage {
		t.Fatalf("failed Close() lost retry state: leaf=%#v pending=%#v", index.leaf, index.pendingDirectory)
	}
	if !reflect.DeepEqual(transaction.unpins, []storage.BlockID{leafBlock, directoryBlock}) {
		t.Fatalf("unpin order after failures = %v, want leaf then pending directory", transaction.unpins)
	}

	delete(transaction.unpinErrors, leafBlock)
	delete(transaction.unpinErrors, directoryBlock)
	if err := index.Close(); err != nil {
		t.Fatalf("Close() retry error = %v", err)
	}
	if index.leaf != nil || index.pendingDirectory != nil || len(transaction.active) != 0 {
		t.Fatalf("successful retry retained pages: leaf=%#v pending=%#v active=%v", index.leaf, index.pendingDirectory, transaction.active)
	}
}

func TestBTreeIndexCloseRejectsNilReceiver(t *testing.T) {
	var index *BTreeIndex
	if err := index.Close(); err == nil {
		t.Fatal("nil index Close() error = nil, want error")
	}
}
