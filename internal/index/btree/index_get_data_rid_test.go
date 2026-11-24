package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBTreeIndexGetDataRIDReturnsCurrentLeafEntry(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("studentsleaf", 3)
	transaction := newBTreeIndexSearchTransaction()
	setBTreeIndexSearchLeaf(transaction, layout, block, [3]int32{20, 5, 8})
	leaf, err := NewLeaf(transaction, block, layout, query.NewIntConstant(20))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	index := &BTreeIndex{leaf: leaf}
	if found, err := index.Next(); err != nil || !found {
		t.Fatalf("Next() = (%v, %v), want one matching entry", found, err)
	}

	pinsBefore, unpinsBefore := len(transaction.pins), len(transaction.unpins)
	got, err := index.GetDataRID()
	if err != nil || !got.Equal(record.NewRID(5, 8)) {
		t.Fatalf("GetDataRID() = (%v, %v), want [5, 8]", got, err)
	}
	if index.leaf != leaf || leaf.currentSlot != 0 || len(transaction.pins) != pinsBefore || len(transaction.unpins) != unpinsBefore {
		t.Fatalf("GetDataRID changed cursor ownership: leaf=%p slot=%d pins=%v unpins=%v", index.leaf, leaf.currentSlot, transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexGetDataRIDRejectsNilOrUnpositionedIndexWithoutPageAccess(t *testing.T) {
	var nilIndex *BTreeIndex
	if got, err := nilIndex.GetDataRID(); got != (record.RID{}) || err == nil {
		t.Fatalf("nil index GetDataRID() = (%v, %v), want empty RID and error", got, err)
	}

	transaction := newBTreeIndexSearchTransaction()
	index := &BTreeIndex{transaction: transaction}
	if got, err := index.GetDataRID(); got != (record.RID{}) || err == nil {
		t.Fatalf("unpositioned GetDataRID() = (%v, %v), want empty RID and error", got, err)
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("unpositioned GetDataRID() touched pages: pins %v/unpins %v", transaction.pins, transaction.unpins)
	}
}

func TestBTreeIndexGetDataRIDPreservesLeafReadErrors(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("studentsleaf", 3)
	cause := errors.New("RID block read failed")
	transaction := newBTreeIndexSearchTransaction()
	transaction.intErrors[pageFindLeafOffset{block: block, offset: pageInsertLeafPosition(layout, 0, "block")}] = cause
	index := &BTreeIndex{
		leaf: &Leaf{
			contents:    &Page{transaction: transaction, block: block, layout: layout},
			searchKey:   query.NewIntConstant(20),
			currentSlot: 0,
		},
	}

	if got, err := index.GetDataRID(); got != (record.RID{}) || !errors.Is(err, cause) {
		t.Fatalf("GetDataRID() = (%v, %v), want empty RID and wrapped page error", got, err)
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 {
		t.Fatalf("failed GetDataRID() changed page ownership: pins %v/unpins %v", transaction.pins, transaction.unpins)
	}
}
