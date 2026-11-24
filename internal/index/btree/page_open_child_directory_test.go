package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageOpenChildTransaction struct {
	*pageInsertDirTransaction
	pins            []storage.BlockID
	unpins          []storage.BlockID
	pinFailureBlock storage.BlockID
	pinError        error
}

func (tx *pageOpenChildTransaction) Pin(block storage.BlockID) error {
	tx.pins = append(tx.pins, block)
	if block == tx.pinFailureBlock {
		return tx.pinError
	}
	return nil
}

func (tx *pageOpenChildTransaction) Unpin(block storage.BlockID) error {
	tx.unpins = append(tx.unpins, block)
	return nil
}

func TestPageOpenChildDirectoryPinsSelectedPageAndLeavesParentOpen(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	parentBlock := storage.NewBlockID("directory.idx", 2)
	childBlock := storage.NewBlockID("directory.idx", 8)
	base := pageInsertDirTransactionWithCount(layout, 1)
	base.ints[0] = 1
	base.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
	base.ints[pageInsertDirPosition(layout, 0, "block")] = int32(childBlock.Number())
	transaction := &pageOpenChildTransaction{pageInsertDirTransaction: base}
	parent, err := NewPage(transaction, parentBlock, layout)
	if err != nil {
		t.Fatalf("NewPage(parent) error = %v", err)
	}

	child, err := parent.OpenChildDirectory(query.NewIntConstant(10))
	if err != nil {
		t.Fatalf("OpenChildDirectory() error = %v", err)
	}
	if child == nil || child.block != childBlock || child.layout != layout || child.transaction != transaction {
		t.Fatalf("opened child = %#v, want same-file block %v with parent layout and transaction", child, childBlock)
	}
	if parent.block != parentBlock || len(transaction.pins) != 2 || transaction.pins[0] != parentBlock || transaction.pins[1] != childBlock || len(transaction.unpins) != 0 {
		t.Fatalf("parent/pin state after open = block %v, pins %v, unpins %v", parent.block, transaction.pins, transaction.unpins)
	}

	if err := child.Close(); err != nil {
		t.Fatalf("child.Close() error = %v", err)
	}
	if parent.block != parentBlock || len(transaction.unpins) != 1 || transaction.unpins[0] != childBlock {
		t.Fatalf("child close changed parent ownership: parent %v, unpins %v", parent.block, transaction.unpins)
	}
	if err := parent.Close(); err != nil {
		t.Fatalf("parent.Close() error = %v", err)
	}
}

func TestPageOpenChildDirectoryRejectsInvalidPageAndLeafLevels(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	block := storage.NewBlockID("directory.idx", 1)
	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{layout: layout}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.OpenChildDirectory(query.NewIntConstant(10)); err == nil {
				t.Fatal("OpenChildDirectory() error = nil, want invalid-page error")
			}
		})
	}
	for _, level := range []int32{0, -1} {
		t.Run("level "+query.NewIntConstant(level).String(), func(t *testing.T) {
			base := pageInsertDirTransactionWithCount(layout, 0)
			base.ints[0] = level
			transaction := &pageOpenChildTransaction{pageInsertDirTransaction: base}
			parent, err := NewPage(transaction, block, layout)
			if err != nil {
				t.Fatalf("NewPage() error = %v", err)
			}
			if _, err := parent.OpenChildDirectory(query.NewIntConstant(10)); err == nil {
				t.Fatalf("OpenChildDirectory() at level %d returned no error", level)
			}
			if len(transaction.pins) != 1 || len(transaction.unpins) != 0 {
				t.Fatalf("pins/unpins = %v/%v, want only the parent pin", transaction.pins, transaction.unpins)
			}
		})
	}
}

func TestPageOpenChildDirectoryWrapsFlagAndSelectionErrors(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	block := storage.NewBlockID("directory.idx", 1)
	flagCause := errors.New("directory flag read failed")
	flagBase := pageInsertDirTransactionWithCount(layout, 1)
	flagBase.readErrors[0] = flagCause
	flagTransaction := &pageOpenChildTransaction{pageInsertDirTransaction: flagBase}
	flagPage := &Page{transaction: flagTransaction, block: block, layout: layout}
	if _, err := flagPage.OpenChildDirectory(query.NewIntConstant(10)); !errors.Is(err, flagCause) {
		t.Fatalf("OpenChildDirectory() flag error = %v, want wrapped cause", err)
	}

	selectionBase := pageInsertDirTransactionWithCount(layout, 1)
	selectionBase.ints[0] = 1
	selectionBase.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
	selectionTransaction := &pageOpenChildTransaction{pageInsertDirTransaction: selectionBase}
	selectionPage := &Page{transaction: selectionTransaction, block: block, layout: layout}
	if _, err := selectionPage.OpenChildDirectory(query.NewIntConstant(9)); err == nil {
		t.Fatal("OpenChildDirectory() below minimum key returned no error")
	}
	if len(selectionTransaction.pins) != 0 {
		t.Fatalf("selection failure pinned blocks %v, want none", selectionTransaction.pins)
	}
}

func TestPageOpenChildDirectoryWrapsChildPinFailure(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	parentBlock := storage.NewBlockID("directory.idx", 1)
	childBlock := storage.NewBlockID("directory.idx", 5)
	cause := errors.New("child page pin failed")
	base := pageInsertDirTransactionWithCount(layout, 1)
	base.ints[0] = 1
	base.ints[pageInsertDirPosition(layout, 0, "dataval")] = 10
	base.ints[pageInsertDirPosition(layout, 0, "block")] = int32(childBlock.Number())
	transaction := &pageOpenChildTransaction{
		pageInsertDirTransaction: base,
		pinFailureBlock:          childBlock,
		pinError:                 cause,
	}
	parent := &Page{transaction: transaction, block: parentBlock, layout: layout}
	if child, err := parent.OpenChildDirectory(query.NewIntConstant(10)); child != nil || !errors.Is(err, cause) {
		t.Fatalf("OpenChildDirectory() = (%v, %v), want nil child and wrapped pin cause", child, err)
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != childBlock || len(transaction.unpins) != 0 || parent.block != parentBlock {
		t.Fatalf("pins/unpins/parent = %v/%v/%v, want failed child pin only and unchanged parent", transaction.pins, transaction.unpins, parent.block)
	}
}

func TestPageOpenChildDirectoryRejectsInvalidDirectorySchema(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	layout := record.NewLayout(&schema)
	transaction := &pageOpenChildTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0)}
	transaction.ints[0] = 1
	page := &Page{transaction: transaction, block: storage.NewBlockID("directory.idx", 1), layout: layout}
	if _, err := page.OpenChildDirectory(query.NewIntConstant(10)); err == nil {
		t.Fatal("OpenChildDirectory() with missing child metadata returned no error")
	}
	if len(transaction.pins) != 0 {
		t.Fatalf("invalid directory selection pinned blocks %v", transaction.pins)
	}
}
