package btree

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageOpenLeafPageClosesDirectoryBeforePinningLeaf(t *testing.T) {
	directoryBlock := storage.NewBlockID("indexdir", 3)
	leafBlock := storage.NewBlockID("indexleaf", 11)
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	leafLayout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: directoryBlock, offset: 0}] = 0
	directory, err := NewPage(tx, directoryBlock, directoryLayout)
	if err != nil {
		t.Fatalf("NewPage(directory) error = %v", err)
	}

	leaf, err := directory.OpenLeafPage(leafBlock, leafLayout)
	if err != nil {
		t.Fatalf("OpenLeafPage() error = %v", err)
	}
	if leaf == nil || leaf.block != leafBlock || leaf.layout != leafLayout || leaf.transaction != tx {
		t.Fatalf("opened leaf page = %#v, want block %v with supplied layout and directory transaction", leaf, leafBlock)
	}
	if directory.block.FileName() != "" {
		t.Fatalf("directory receiver block = %v, want closed", directory.block)
	}
	if len(tx.pins) != 2 || tx.pins[0] != directoryBlock || tx.pins[1] != leafBlock || len(tx.unpins) != 1 || tx.unpins[0] != directoryBlock {
		t.Fatalf("pin lifecycle = pins %v, unpins %v; want directory close before leaf pin", tx.pins, tx.unpins)
	}
	if tx.maxActive != 1 {
		t.Fatalf("maximum simultaneous pins = %d, want one", tx.maxActive)
	}
	if err := leaf.Close(); err != nil {
		t.Fatalf("leaf.Close() error = %v", err)
	}
	if len(tx.unpins) != 2 || tx.unpins[1] != leafBlock {
		t.Fatalf("unpins after caller closes leaf = %v, want directory then leaf", tx.unpins)
	}
}

func TestPageOpenLeafPageRejectsInvalidDirectoryAndLeafInputs(t *testing.T) {
	directoryBlock := storage.NewBlockID("indexdir", 3)
	validDirectoryLayout := pageInsertDirLayout(record.IntegerField)
	validLeafLayout := pageOpenLeafLayout()
	nilSchemaLayout := record.NewLayoutFromMetadata(nil, nil, 8)
	var typedNil *pageFindLeafTransaction

	for _, test := range []struct {
		name       string
		page       *Page
		leafBlock  storage.BlockID
		leafLayout *record.Layout
	}{
		{name: "nil page", leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "closed page", page: &Page{transaction: newPageFindLeafTransaction(), layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "nil transaction", page: &Page{block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "nil directory layout", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "nil directory schema", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock, layout: nilSchemaLayout}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: validLeafLayout},
		{name: "empty leaf file", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("", 1), leafLayout: validLeafLayout},
		{name: "negative leaf number", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", -1), leafLayout: validLeafLayout},
		{name: "nil leaf layout", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", 1)},
		{name: "nil leaf schema", page: &Page{transaction: newPageFindLeafTransaction(), block: directoryBlock, layout: validDirectoryLayout}, leafBlock: storage.NewBlockID("indexleaf", 1), leafLayout: nilSchemaLayout},
	} {
		t.Run(test.name, func(t *testing.T) {
			if leaf, err := test.page.OpenLeafPage(test.leafBlock, test.leafLayout); leaf != nil || err == nil {
				t.Fatalf("OpenLeafPage() = (%v, %v), want validation error", leaf, err)
			}
			if test.page != nil {
				if test.name != "closed page" && test.page.block != directoryBlock {
					t.Fatalf("validation changed directory block to %v", test.page.block)
				}
				if tx, ok := test.page.transaction.(*pageFindLeafTransaction); ok && tx != nil && len(tx.unpins) != 0 {
					t.Fatalf("validation unpinned directory block: %v", tx.unpins)
				}
			}
		})
	}
}

func TestPageOpenLeafPageRequiresLevelZero(t *testing.T) {
	directoryBlock := storage.NewBlockID("indexdir", 5)
	leafBlock := storage.NewBlockID("indexleaf", 8)
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	for _, level := range []int32{-1, 1, 2} {
		t.Run("level "+strconv.FormatInt(int64(level), 10), func(t *testing.T) {
			tx := newPageFindLeafTransaction()
			tx.ints[pageFindLeafOffset{block: directoryBlock, offset: 0}] = level
			page, err := NewPage(tx, directoryBlock, directoryLayout)
			if err != nil {
				t.Fatalf("NewPage() error = %v", err)
			}
			if leaf, err := page.OpenLeafPage(leafBlock, pageOpenLeafLayout()); leaf != nil || err == nil {
				t.Fatalf("OpenLeafPage() at level %d = (%v, %v), want level error", level, leaf, err)
			}
			if page.block != directoryBlock || len(tx.pins) != 1 || len(tx.unpins) != 0 {
				t.Fatalf("level %d changed directory ownership: block %v pins %v unpins %v", level, page.block, tx.pins, tx.unpins)
			}
		})
	}
}

func TestPageOpenLeafPageWrapsLevelAndDirectoryUnpinFailures(t *testing.T) {
	directoryBlock := storage.NewBlockID("indexdir", 1)
	leafBlock := storage.NewBlockID("indexleaf", 7)
	directoryLayout := pageInsertDirLayout(record.IntegerField)
	flagCause := errors.New("directory level read failed")
	flagTx := newPageFindLeafTransaction()
	flagTx.intErrors[pageFindLeafOffset{block: directoryBlock, offset: 0}] = flagCause
	flagPage := &Page{transaction: flagTx, block: directoryBlock, layout: directoryLayout}
	if leaf, err := flagPage.OpenLeafPage(leafBlock, pageOpenLeafLayout()); leaf != nil || !errors.Is(err, flagCause) {
		t.Fatalf("OpenLeafPage() level error = (%v, %v), want wrapped cause", leaf, err)
	}
	if flagPage.block != directoryBlock || len(flagTx.pins) != 0 || len(flagTx.unpins) != 0 {
		t.Fatalf("level-read failure changed directory ownership: block %v, pins %v, unpins %v", flagPage.block, flagTx.pins, flagTx.unpins)
	}

	unpinCause := errors.New("directory unpin failed")
	unpinTx := newPageFindLeafTransaction()
	unpinTx.ints[pageFindLeafOffset{block: directoryBlock, offset: 0}] = 0
	unpinTx.unpinErrors[directoryBlock] = unpinCause
	unpinPage, err := NewPage(unpinTx, directoryBlock, directoryLayout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if leaf, err := unpinPage.OpenLeafPage(leafBlock, pageOpenLeafLayout()); leaf != nil || !errors.Is(err, unpinCause) {
		t.Fatalf("OpenLeafPage() unpin error = (%v, %v), want wrapped cause", leaf, err)
	}
	if unpinPage.block != directoryBlock || len(unpinTx.pins) != 1 || len(unpinTx.unpins) != 1 {
		t.Fatalf("unpin failure changed directory ownership: block %v pins %v unpins %v", unpinPage.block, unpinTx.pins, unpinTx.unpins)
	}
	if err := unpinPage.Close(); err != nil {
		t.Fatalf("retry Close() after unpin failure: %v", err)
	}
}

func TestPageOpenLeafPageLeavesDirectoryClosedWhenLeafPinFails(t *testing.T) {
	directoryBlock := storage.NewBlockID("indexdir", 2)
	leafBlock := storage.NewBlockID("indexleaf", 10)
	cause := errors.New("leaf pin failed")
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: directoryBlock, offset: 0}] = 0
	tx.pinErrors[leafBlock] = cause
	directory, err := NewPage(tx, directoryBlock, pageInsertDirLayout(record.IntegerField))
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if leaf, err := directory.OpenLeafPage(leafBlock, pageOpenLeafLayout()); leaf != nil || !errors.Is(err, cause) {
		t.Fatalf("OpenLeafPage() pin error = (%v, %v), want wrapped leaf pin cause", leaf, err)
	}
	if directory.block.FileName() != "" || directory.transaction != tx {
		t.Fatalf("directory receiver after leaf pin failure = %#v, want closed with transaction retained", directory)
	}
	if len(tx.pins) != 2 || tx.pins[0] != directoryBlock || tx.pins[1] != leafBlock || len(tx.unpins) != 1 || tx.unpins[0] != directoryBlock || tx.maxActive != 1 {
		t.Fatalf("pins/unpins/max active = %v/%v/%d, want close-before-failed-pin with one active page", tx.pins, tx.unpins, tx.maxActive)
	}
}

func pageOpenLeafLayout() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}
