package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewLeafPositionsBeforeFirstMatchingIntegerKey(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 4
	for slot, key := range []int32{1, 3, 3, 5} {
		tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}] = key
	}

	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(3))
	if err != nil {
		t.Fatalf("NewLeaf() error = %v", err)
	}
	if leaf == nil || leaf.contents == nil || leaf.contents.block != block || leaf.contents.layout != layout || leaf.contents.transaction != tx {
		t.Fatalf("NewLeaf() = %#v, want retained page %v", leaf, block)
	}
	if leaf.searchKey != query.NewIntConstant(3) || leaf.currentSlot != 0 {
		t.Fatalf("leaf search state = key %s, slot %d; want key 3 immediately before duplicate matches", leaf.searchKey, leaf.currentSlot)
	}
	if len(tx.pins) != 1 || tx.pins[0] != block || len(tx.unpins) != 0 {
		t.Fatalf("pin lifecycle = pins %v unpins %v; want one retained page", tx.pins, tx.unpins)
	}
}

func TestNewLeafPositionsVarcharAndEmptyPages(t *testing.T) {
	block := storage.NewBlockID("names.idx", 1)
	varcharLayout := pageInsertDirLayout(record.VarcharField)
	varcharTx := newPageFindLeafTransaction()
	varcharTx.ints[pageFindLeafOffset{block: block, offset: 4}] = 2
	varcharTx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(varcharLayout, 0, "dataval")}] = "alpha"
	varcharTx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(varcharLayout, 1, "dataval")}] = "beta"
	varcharLeaf, err := NewLeaf(varcharTx, block, varcharLayout, query.NewStringConstant("beta"))
	if err != nil || varcharLeaf == nil || varcharLeaf.currentSlot != 0 {
		t.Fatalf("NewLeaf(varchar) = (%#v, %v), want slot before beta", varcharLeaf, err)
	}

	emptyBlock := storage.NewBlockID("empty.idx", 0)
	emptyTx := newPageFindLeafTransaction()
	emptyLeaf, err := NewLeaf(emptyTx, emptyBlock, pageOpenLeafLayout(), query.NewIntConstant(1))
	if err != nil || emptyLeaf == nil || emptyLeaf.currentSlot != -1 {
		t.Fatalf("NewLeaf(empty) = (%#v, %v), want slot -1", emptyLeaf, err)
	}
}

func TestNewLeafRejectsInvalidInputsBeforePinning(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	var typedNil *pageFindLeafTransaction
	for _, test := range []struct {
		name      string
		trans     PageTransaction
		block     storage.BlockID
		layout    *record.Layout
		searchKey query.Constant
	}{
		{name: "nil transaction", block: block, layout: layout, searchKey: query.NewIntConstant(1)},
		{name: "typed nil transaction", trans: typedNil, block: block, layout: layout, searchKey: query.NewIntConstant(1)},
		{name: "empty block file", trans: newPageFindLeafTransaction(), block: storage.NewBlockID("", 1), layout: layout, searchKey: query.NewIntConstant(1)},
		{name: "negative block number", trans: newPageFindLeafTransaction(), block: storage.NewBlockID("leaf.idx", -1), layout: layout, searchKey: query.NewIntConstant(1)},
		{name: "nil layout", trans: newPageFindLeafTransaction(), block: block, searchKey: query.NewIntConstant(1)},
		{name: "nil schema", trans: newPageFindLeafTransaction(), block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8), searchKey: query.NewIntConstant(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if leaf, err := NewLeaf(test.trans, test.block, test.layout, test.searchKey); leaf != nil || err == nil {
				t.Fatalf("NewLeaf() = (%v, %v), want validation error", leaf, err)
			}
			if tx, ok := test.trans.(*pageFindLeafTransaction); ok && tx != nil && (len(tx.pins) != 0 || len(tx.unpins) != 0) {
				t.Fatalf("validation reached transaction: pins %v unpins %v", tx.pins, tx.unpins)
			}
		})
	}
}

func TestNewLeafCleansUpWhenSearchPositioningFails(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 3)
	layout := pageOpenLeafLayout()
	for _, test := range []struct {
		name    string
		key     query.Constant
		layout  *record.Layout
		errorAt int
	}{
		{name: "key type mismatch", key: query.NewStringConstant("one")},
		{name: "missing dataval field", key: query.NewIntConstant(1), layout: record.NewLayout(&record.Schema{})},
		{name: "record count read", key: query.NewIntConstant(1), errorAt: 4},
		{name: "leaf key read", key: query.NewIntConstant(1), errorAt: pageInsertDirPosition(layout, 0, "dataval")},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
			cause := errors.New("search position failed")
			if test.errorAt != 0 {
				tx.intErrors[pageFindLeafOffset{block: block, offset: test.errorAt}] = cause
			}
			testLayout := test.layout
			if testLayout == nil {
				testLayout = layout
			}
			leaf, err := NewLeaf(tx, block, testLayout, test.key)
			if leaf != nil || err == nil {
				t.Fatalf("NewLeaf() = (%v, %v), want positioning error", leaf, err)
			}
			if test.errorAt != 0 && !errors.Is(err, cause) {
				t.Fatalf("NewLeaf() error = %v, want wrapped read cause", err)
			}
			if len(tx.pins) != 1 || tx.pins[0] != block || len(tx.unpins) != 1 || tx.unpins[0] != block {
				t.Fatalf("failed construction pin lifecycle = pins %v unpins %v", tx.pins, tx.unpins)
			}
		})
	}
}

func TestNewLeafJoinsPositionAndCleanupErrors(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 5)
	layout := pageOpenLeafLayout()
	positionCause := errors.New("leaf count read failed")
	cleanupCause := errors.New("leaf unpin failed")
	tx := newPageFindLeafTransaction()
	tx.intErrors[pageFindLeafOffset{block: block, offset: 4}] = positionCause
	tx.unpinErrors[block] = cleanupCause
	leaf, err := NewLeaf(tx, block, layout, query.NewIntConstant(1))
	if leaf != nil || !errors.Is(err, positionCause) || !errors.Is(err, cleanupCause) {
		t.Fatalf("NewLeaf() = (%v, %v), want joined positioning and cleanup causes", leaf, err)
	}
	if len(tx.pins) != 1 || len(tx.unpins) != 1 || tx.unpins[0] != block {
		t.Fatalf("failed cleanup pin lifecycle = pins %v unpins %v", tx.pins, tx.unpins)
	}
}

func TestNewLeafWrapsPinErrors(t *testing.T) {
	block := storage.NewBlockID("leaf.idx", 7)
	cause := errors.New("leaf pin failed")
	tx := newPageFindLeafTransaction()
	tx.pinErrors[block] = cause
	if leaf, err := NewLeaf(tx, block, pageOpenLeafLayout(), query.NewIntConstant(1)); leaf != nil || !errors.Is(err, cause) {
		t.Fatalf("NewLeaf() = (%v, %v), want wrapped pin error", leaf, err)
	}
	if len(tx.pins) != 1 || len(tx.unpins) != 0 {
		t.Fatalf("pin failure calls = pins %v unpins %v, want one pin attempt and no unpin", tx.pins, tx.unpins)
	}
}
