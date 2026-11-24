package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLeafCloseReleasesPageOnceAndInvalidatesCursor(t *testing.T) {
	block := storage.NewBlockID("leaf-close.idx", 3)
	tx := newPageFindLeafTransaction()
	leaf, err := NewLeaf(tx, block, pageOpenLeafLayout(), query.NewIntConstant(4))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}

	if err := leaf.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if leaf.contents != nil || len(tx.unpins) != 1 || tx.unpins[0] != block || tx.active[block] {
		t.Fatalf("close state = page %v, unpins %v, active %t; want cleared cursor and released pin", leaf.contents, tx.unpins, tx.active[block])
	}
	if err := leaf.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v, want nil", err)
	}
	if len(tx.unpins) != 1 {
		t.Fatalf("repeated Close() unpinned again: %v", tx.unpins)
	}
	if _, err := leaf.Next(); err == nil {
		t.Fatal("Next() after Close() error = nil, want closed cursor error")
	}
	if _, err := leaf.GetDataRID(); err == nil {
		t.Fatal("GetDataRID() after Close() error = nil, want closed cursor error")
	}
	if err := leaf.Delete(recordRIDForLeafClose()); err == nil {
		t.Fatal("Delete() after Close() error = nil, want closed cursor error")
	}
	if _, err := leaf.Insert(recordRIDForLeafClose()); err == nil {
		t.Fatal("Insert() after Close() error = nil, want closed cursor error")
	}
}

func TestLeafClosePreservesPageOnUnpinFailureForRetry(t *testing.T) {
	block := storage.NewBlockID("leaf-close.idx", 8)
	tx := newPageFindLeafTransaction()
	cause := errors.New("unpin leaf failed")
	tx.unpinErrors[block] = cause
	leaf, err := NewLeaf(tx, block, pageOpenLeafLayout(), query.NewIntConstant(9))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	page := leaf.contents

	if err := leaf.Close(); !errors.Is(err, cause) {
		t.Fatalf("first Close() error = %v, want wrapped unpin cause", err)
	}
	if leaf.contents != page || page.block != block || !tx.active[block] {
		t.Fatalf("failed close state = page %p, block %v, active %t; want retryable ownership", leaf.contents, page.block, tx.active[block])
	}
	if err := leaf.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	if leaf.contents != nil || page.block.FileName() != "" || tx.active[block] || len(tx.unpins) != 2 {
		t.Fatalf("retry close state = leaf page %v, block %v, active %t, unpins %v; want released pin", leaf.contents, page.block, tx.active[block], tx.unpins)
	}
}

func TestLeafCloseRejectsNilReceiver(t *testing.T) {
	var leaf *Leaf
	if err := leaf.Close(); err == nil {
		t.Fatal("Close() on nil receiver error = nil, want contextual error")
	}
}

func recordRIDForLeafClose() record.RID {
	return record.NewRID(1, 2)
}
