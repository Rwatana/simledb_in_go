package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBTreeIndexNextDelegatesMatchingEntryAndExhaustion(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("studentsleaf", 3)
	transaction := newBTreeIndexSearchTransaction()
	setBTreeIndexSearchLeaf(transaction, layout, block, [3]int32{20, 5, 8})
	leaf, err := NewLeaf(transaction, block, layout, query.NewIntConstant(20))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	index := &BTreeIndex{leaf: leaf}

	found, err := index.Next()
	if err != nil || !found {
		t.Fatalf("Next() = (%v, %v), want one matching entry", found, err)
	}
	found, err = index.Next()
	if err != nil || found {
		t.Fatalf("Next() at end = (%v, %v), want clean exhaustion", found, err)
	}
	if err := index.leaf.Close(); err != nil {
		t.Fatalf("close leaf: %v", err)
	}
}

func TestBTreeIndexNextRejectsNilOrUnpositionedIndexWithoutPageAccess(t *testing.T) {
	var nilIndex *BTreeIndex
	if found, err := nilIndex.Next(); found || err == nil {
		t.Fatalf("nil index Next() = (%v, %v), want error and false", found, err)
	}
	transaction := newBTreeIndexSearchTransaction()
	index := &BTreeIndex{transaction: transaction}
	if found, err := index.Next(); found || err == nil {
		t.Fatalf("unpositioned Next() = (%v, %v), want error and false", found, err)
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("unpositioned Next() touched pages: pins %v/unpins %v", transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexNextPreservesLeafAdvanceErrors(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("studentsleaf", 3)
	cause := errors.New("leaf count read failed")
	transaction := newBTreeIndexSearchTransaction()
	transaction.intErrors[pageFindLeafOffset{block: block, offset: 4}] = cause
	index := &BTreeIndex{
		leaf: &Leaf{
			contents:    &Page{transaction: transaction, block: block, layout: layout},
			searchKey:   query.NewIntConstant(20),
			currentSlot: -1,
		},
	}

	if found, err := index.Next(); found || !errors.Is(err, cause) {
		t.Fatalf("Next() = (%v, %v), want false and wrapped leaf error", found, err)
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("Next() changed page ownership: pins %v/unpins %v", transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexNextRejectsClosedLeafCursor(t *testing.T) {
	index := &BTreeIndex{leaf: &Leaf{searchKey: query.NewIntConstant(20)}}
	if found, err := index.Next(); found || err == nil {
		t.Fatalf("Next() with closed leaf = (%v, %v), want error and false", found, err)
	}
}
