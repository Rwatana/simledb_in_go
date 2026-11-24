package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageOpenOverflowPageClosesBeforePinningTarget(t *testing.T) {
	currentBlock := storage.NewBlockID("leaf.idx", 2)
	overflowBlock := storage.NewBlockID("leaf.idx", 9)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	current, err := NewPage(tx, currentBlock, layout)
	if err != nil {
		t.Fatalf("NewPage(current) error = %v", err)
	}

	overflow, err := current.OpenOverflowPage(overflowBlock)
	if err != nil {
		t.Fatalf("OpenOverflowPage() error = %v", err)
	}
	if overflow == nil || overflow.block != overflowBlock || overflow.layout != layout || overflow.transaction != tx {
		t.Fatalf("opened overflow page = %#v, want block %v with retained dependencies", overflow, overflowBlock)
	}
	if current.block.FileName() != "" || current.transaction != tx || current.layout != layout {
		t.Fatalf("current page after overflow = %#v, want closed with dependencies retained", current)
	}
	if len(tx.pins) != 2 || tx.pins[0] != currentBlock || tx.pins[1] != overflowBlock || len(tx.unpins) != 1 || tx.unpins[0] != currentBlock || tx.maxActive != 1 {
		t.Fatalf("pin lifecycle = pins %v, unpins %v, max active %d; want close-before-pin with one active page", tx.pins, tx.unpins, tx.maxActive)
	}
	if err := overflow.Close(); err != nil {
		t.Fatalf("overflow.Close() error = %v", err)
	}
	if len(tx.unpins) != 2 || tx.unpins[1] != overflowBlock {
		t.Fatalf("unpins after caller close = %v, want current then overflow", tx.unpins)
	}
}

func TestPageOpenOverflowPageRejectsInvalidInputsBeforeClosing(t *testing.T) {
	currentBlock := storage.NewBlockID("leaf.idx", 2)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	var typedNil *pageFindLeafTransaction
	nilSchema := record.NewLayoutFromMetadata(nil, nil, 8)
	for _, test := range []struct {
		name     string
		page     *Page
		overflow storage.BlockID
	}{
		{name: "nil page", overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "closed page", page: &Page{transaction: tx, layout: layout}, overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "nil transaction", page: &Page{block: currentBlock, layout: layout}, overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: currentBlock, layout: layout}, overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "nil layout", page: &Page{transaction: tx, block: currentBlock}, overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "nil layout schema", page: &Page{transaction: tx, block: currentBlock, layout: nilSchema}, overflow: storage.NewBlockID("leaf.idx", 3)},
		{name: "empty target file", page: &Page{transaction: tx, block: currentBlock, layout: layout}, overflow: storage.NewBlockID("", 3)},
		{name: "target file differs", page: &Page{transaction: tx, block: currentBlock, layout: layout}, overflow: storage.NewBlockID("other.idx", 3)},
		{name: "negative target number", page: &Page{transaction: tx, block: currentBlock, layout: layout}, overflow: storage.NewBlockID("leaf.idx", -1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if page, err := test.page.OpenOverflowPage(test.overflow); page != nil || err == nil {
				t.Fatalf("OpenOverflowPage() = (%v, %v), want validation error", page, err)
			}
			if test.page != nil && test.name != "closed page" && test.page.block != currentBlock {
				t.Fatalf("validation changed current block to %v", test.page.block)
			}
			if len(tx.unpins) != 0 {
				t.Fatalf("validation unpinned current page: %v", tx.unpins)
			}
		})
	}
}

func TestPageOpenOverflowPageWrapsRetryableUnpinFailure(t *testing.T) {
	currentBlock := storage.NewBlockID("leaf.idx", 4)
	overflowBlock := storage.NewBlockID("leaf.idx", 8)
	cause := errors.New("current leaf unpin failed")
	tx := newPageFindLeafTransaction()
	tx.unpinErrors[currentBlock] = cause
	current, err := NewPage(tx, currentBlock, pageOpenLeafLayout())
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if overflow, err := current.OpenOverflowPage(overflowBlock); overflow != nil || !errors.Is(err, cause) {
		t.Fatalf("OpenOverflowPage() = (%v, %v), want wrapped unpin error", overflow, err)
	}
	if current.block != currentBlock || len(tx.pins) != 1 || len(tx.unpins) != 1 {
		t.Fatalf("unpin failure changed current page: block %v pins %v unpins %v", current.block, tx.pins, tx.unpins)
	}
	tx.unpinErrors = nil
	if err := current.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
}

func TestPageOpenOverflowPageLeavesReceiverClosedWhenTargetPinFails(t *testing.T) {
	currentBlock := storage.NewBlockID("leaf.idx", 1)
	overflowBlock := storage.NewBlockID("leaf.idx", 7)
	cause := errors.New("overflow leaf pin failed")
	tx := newPageFindLeafTransaction()
	tx.pinErrors[overflowBlock] = cause
	current, err := NewPage(tx, currentBlock, pageOpenLeafLayout())
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if overflow, err := current.OpenOverflowPage(overflowBlock); overflow != nil || !errors.Is(err, cause) {
		t.Fatalf("OpenOverflowPage() = (%v, %v), want wrapped pin error", overflow, err)
	}
	if current.block.FileName() != "" || current.transaction != tx {
		t.Fatalf("current receiver after failed target pin = %#v, want closed with transaction retained", current)
	}
	if len(tx.pins) != 2 || tx.pins[0] != currentBlock || tx.pins[1] != overflowBlock || len(tx.unpins) != 1 || tx.unpins[0] != currentBlock || tx.maxActive != 1 {
		t.Fatalf("pin lifecycle = pins %v unpins %v max active %d; want close-before-failed-pin with one active page", tx.pins, tx.unpins, tx.maxActive)
	}
}
