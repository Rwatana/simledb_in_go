package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageFindLeafOffset struct {
	block  storage.BlockID
	offset int
}

type pageFindLeafTransaction struct {
	pins            []storage.BlockID
	unpins          []storage.BlockID
	intReads        []pageFindLeafOffset
	active          map[storage.BlockID]bool
	maxActive       int
	ints            map[pageFindLeafOffset]int32
	strings         map[pageFindLeafOffset]string
	intErrors       map[pageFindLeafOffset]error
	stringErrors    map[pageFindLeafOffset]error
	pinErrors       map[storage.BlockID]error
	unpinErrors     map[storage.BlockID]error
	unpinErrorCalls map[storage.BlockID]int
}

func newPageFindLeafTransaction() *pageFindLeafTransaction {
	return &pageFindLeafTransaction{
		active:          make(map[storage.BlockID]bool),
		ints:            make(map[pageFindLeafOffset]int32),
		strings:         make(map[pageFindLeafOffset]string),
		intErrors:       make(map[pageFindLeafOffset]error),
		stringErrors:    make(map[pageFindLeafOffset]error),
		pinErrors:       make(map[storage.BlockID]error),
		unpinErrors:     make(map[storage.BlockID]error),
		unpinErrorCalls: make(map[storage.BlockID]int),
	}
}

func (tx *pageFindLeafTransaction) Pin(block storage.BlockID) error {
	tx.pins = append(tx.pins, block)
	if err := tx.pinErrors[block]; err != nil {
		return err
	}
	tx.active[block] = true
	active := 0
	for _, pinned := range tx.active {
		if pinned {
			active++
		}
	}
	if active > tx.maxActive {
		tx.maxActive = active
	}
	return nil
}

func (tx *pageFindLeafTransaction) Unpin(block storage.BlockID) error {
	tx.unpins = append(tx.unpins, block)
	if err := tx.unpinErrors[block]; err != nil && tx.unpinErrorCalls[block] == 0 {
		tx.unpinErrorCalls[block]++
		return err
	}
	delete(tx.active, block)
	return nil
}

func (*pageFindLeafTransaction) BlockSize() (int, error) { return 4096, nil }

func (tx *pageFindLeafTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	position := pageFindLeafOffset{block: block, offset: offset}
	tx.intReads = append(tx.intReads, position)
	if err := tx.intErrors[position]; err != nil {
		return 0, err
	}
	return tx.ints[position], nil
}

func (tx *pageFindLeafTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	position := pageFindLeafOffset{block: block, offset: offset}
	if err := tx.stringErrors[position]; err != nil {
		return "", err
	}
	return tx.strings[position], nil
}

func (tx *pageFindLeafTransaction) SetInt(block storage.BlockID, offset, value int32, _ bool) error {
	tx.ints[pageFindLeafOffset{block: block, offset: int(offset)}] = value
	return nil
}

func (tx *pageFindLeafTransaction) SetString(block storage.BlockID, offset int32, value string, _ bool) error {
	tx.strings[pageFindLeafOffset{block: block, offset: int(offset)}] = value
	return nil
}

func TestPageFindLeafBlockDescendsOnePinnedDirectoryAtATime(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	root := storage.NewBlockID("directory.idx", 0)
	levelOne := storage.NewBlockID("directory.idx", 4)
	levelZero := storage.NewBlockID("directory.idx", 9)
	leaf := storage.NewBlockID("directory.idx", 15)
	tx := newPageFindLeafTransaction()
	setPageFindLeafDirectory(tx, layout, root, 2, 10, levelOne.Number())
	setPageFindLeafDirectory(tx, layout, levelOne, 1, 10, levelZero.Number())
	setPageFindLeafDirectory(tx, layout, levelZero, 0, 10, leaf.Number())
	page, err := NewPage(tx, root, layout)
	if err != nil {
		t.Fatalf("NewPage(root) error = %v", err)
	}

	got, err := page.FindLeafBlock(query.NewIntConstant(10))
	if err != nil {
		t.Fatalf("FindLeafBlock() error = %v", err)
	}
	if got != leaf {
		t.Fatalf("FindLeafBlock() = %v, want leaf block %v", got, leaf)
	}
	if page.block != levelZero || page.layout != layout || page.transaction != tx {
		t.Fatalf("page after descent = block %v, layout %p, transaction %p; want final directory %v with original dependencies", page.block, page.layout, page.transaction, levelZero)
	}
	if len(tx.pins) != 3 || tx.pins[0] != root || tx.pins[1] != levelOne || tx.pins[2] != levelZero {
		t.Fatalf("pins = %v, want root and directory path [%v %v %v]", tx.pins, root, levelOne, levelZero)
	}
	if len(tx.unpins) != 2 || tx.unpins[0] != root || tx.unpins[1] != levelOne {
		t.Fatalf("unpins = %v, want ancestors [%v %v]", tx.unpins, root, levelOne)
	}
	if tx.maxActive != 1 {
		t.Fatalf("maximum simultaneous directory pins = %d, want one", tx.maxActive)
	}
	if err := page.Close(); err != nil {
		t.Fatalf("close final directory page: %v", err)
	}
	if len(tx.unpins) != 3 || tx.unpins[2] != levelZero {
		t.Fatalf("unpins after caller closes receiver = %v, want final page %v released", tx.unpins, levelZero)
	}
}

func TestPageFindLeafBlockAtLevelZeroLeavesDirectoryOpen(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	directory := storage.NewBlockID("directory.idx", 7)
	leaf := storage.NewBlockID("directory.idx", 12)
	tx := newPageFindLeafTransaction()
	setPageFindLeafDirectory(tx, layout, directory, 0, 10, leaf.Number())
	page, err := NewPage(tx, directory, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	got, err := page.FindLeafBlock(query.NewIntConstant(10))
	if err != nil || got != leaf {
		t.Fatalf("FindLeafBlock() = (%v, %v), want leaf block %v", got, err, leaf)
	}
	if page.block != directory || len(tx.pins) != 1 || len(tx.unpins) != 0 {
		t.Fatalf("level-zero page/pins after search = %v/%v/%v, want retained directory pin only", page.block, tx.pins, tx.unpins)
	}
}

func TestPageFindLeafBlockRejectsInvalidPagesAndNegativeLevels(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	directory := storage.NewBlockID("directory.idx", 3)
	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{layout: layout}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.FindLeafBlock(query.NewIntConstant(10)); err == nil {
				t.Fatal("FindLeafBlock() error = nil, want invalid page error")
			}
		})
	}
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: directory, offset: 0}] = -1
	page, err := NewPage(tx, directory, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if _, err := page.FindLeafBlock(query.NewIntConstant(10)); err == nil {
		t.Fatal("FindLeafBlock() with negative level returned no error")
	}
	if page.block != directory || len(tx.pins) != 1 || len(tx.unpins) != 0 {
		t.Fatalf("negative-level failure changed page ownership: block %v, pins %v, unpins %v", page.block, tx.pins, tx.unpins)
	}
}

func TestPageFindLeafBlockWrapsFlagAndChildSelectionErrors(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	directory := storage.NewBlockID("directory.idx", 5)
	flagCause := errors.New("directory level read failed")
	tx := newPageFindLeafTransaction()
	tx.intErrors[pageFindLeafOffset{block: directory, offset: 0}] = flagCause
	page := &Page{transaction: tx, block: directory, layout: layout}
	if _, err := page.FindLeafBlock(query.NewIntConstant(10)); !errors.Is(err, flagCause) {
		t.Fatalf("FindLeafBlock() flag error = %v, want wrapped cause", err)
	}
	if page.block != directory {
		t.Fatalf("flag-read failure changed page to %v", page.block)
	}

	selectionTx := newPageFindLeafTransaction()
	setPageFindLeafDirectory(selectionTx, layout, directory, 1, 20, 8)
	selectionPage := &Page{transaction: selectionTx, block: directory, layout: layout}
	if _, err := selectionPage.FindLeafBlock(query.NewIntConstant(10)); err == nil {
		t.Fatal("FindLeafBlock() with a key below the minimum returned no error")
	}
	if selectionPage.block != directory || len(selectionTx.unpins) != 0 {
		t.Fatalf("child-selection failure changed page ownership: block %v, unpins %v", selectionPage.block, selectionTx.unpins)
	}
}

func TestPageFindLeafBlockPreservesPageWhenUnpinFails(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	root := storage.NewBlockID("directory.idx", 1)
	child := storage.NewBlockID("directory.idx", 6)
	cause := errors.New("root unpin failed")
	tx := newPageFindLeafTransaction()
	setPageFindLeafDirectory(tx, layout, root, 1, 10, child.Number())
	tx.unpinErrors[root] = cause
	page, err := NewPage(tx, root, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if got, err := page.FindLeafBlock(query.NewIntConstant(10)); got != (storage.BlockID{}) || !errors.Is(err, cause) {
		t.Fatalf("FindLeafBlock() = (%v, %v), want no block and wrapped unpin error", got, err)
	}
	if page.block != root || len(tx.pins) != 1 || len(tx.unpins) != 1 || tx.unpins[0] != root {
		t.Fatalf("unpin failure changed page state: block %v, pins %v, unpins %v", page.block, tx.pins, tx.unpins)
	}
	if err := page.Close(); err != nil {
		t.Fatalf("retry Close() after unpin failure: %v", err)
	}
	if page.block.FileName() != "" {
		t.Fatalf("page block after successful retry = %v, want closed", page.block)
	}
}

func TestPageFindLeafBlockLeavesReceiverClosedWhenChildPinFails(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	root := storage.NewBlockID("directory.idx", 2)
	child := storage.NewBlockID("directory.idx", 8)
	cause := errors.New("child directory pin failed")
	tx := newPageFindLeafTransaction()
	setPageFindLeafDirectory(tx, layout, root, 1, 10, child.Number())
	tx.pinErrors[child] = cause
	page, err := NewPage(tx, root, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if got, err := page.FindLeafBlock(query.NewIntConstant(10)); got != (storage.BlockID{}) || !errors.Is(err, cause) {
		t.Fatalf("FindLeafBlock() = (%v, %v), want no block and wrapped child pin error", got, err)
	}
	if page.block.FileName() != "" || page.transaction != tx || page.layout != layout {
		t.Fatalf("receiver after failed child pin = block %v, transaction %p, layout %p; want closed with dependencies retained", page.block, page.transaction, page.layout)
	}
	if len(tx.pins) != 2 || tx.pins[0] != root || tx.pins[1] != child || len(tx.unpins) != 1 || tx.unpins[0] != root {
		t.Fatalf("pins/unpins after child pin failure = %v/%v, want parent release before one failed child pin", tx.pins, tx.unpins)
	}
}

func setPageFindLeafDirectory(tx *pageFindLeafTransaction, layout *record.Layout, block storage.BlockID, level int32, key int32, child int) {
	tx.ints[pageFindLeafOffset{block: block, offset: 0}] = level
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = key
	tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "block")}] = int32(child)
}
