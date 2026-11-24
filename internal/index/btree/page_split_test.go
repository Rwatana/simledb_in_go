package btree

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSplitLocation struct {
	block  storage.BlockID
	offset int
}

type pageSplitWrite struct {
	location  pageSplitLocation
	intValue  int32
	textValue string
	isString  bool
	logged    bool
}

type pageSplitTransaction struct {
	blockSize     int
	blockSizeErr  error
	ints          map[pageSplitLocation]int32
	strings       map[pageSplitLocation]string
	readCalls     int
	readErrorAt   int
	readError     error
	writes        []pageSplitWrite
	writeErrorAt  int
	writeError    error
	appendedBlock storage.BlockID
	appendCalls   []string
	appendErr     error
	pinCalls      []storage.BlockID
	pinErr        error
	unpinCalls    []storage.BlockID
	unpinErr      error
}

func (tx *pageSplitTransaction) Append(fileName string) (storage.BlockID, error) {
	tx.appendCalls = append(tx.appendCalls, fileName)
	return tx.appendedBlock, tx.appendErr
}

func (tx *pageSplitTransaction) Pin(block storage.BlockID) error {
	tx.pinCalls = append(tx.pinCalls, block)
	return tx.pinErr
}

func (tx *pageSplitTransaction) Unpin(block storage.BlockID) error {
	tx.unpinCalls = append(tx.unpinCalls, block)
	return tx.unpinErr
}

func (tx *pageSplitTransaction) BlockSize() (int, error) {
	return tx.blockSize, tx.blockSizeErr
}

func (tx *pageSplitTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	tx.readCalls++
	if tx.readCalls == tx.readErrorAt {
		return 0, tx.readError
	}
	return tx.ints[pageSplitLocation{block: block, offset: offset}], nil
}

func (tx *pageSplitTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	tx.readCalls++
	if tx.readCalls == tx.readErrorAt {
		return "", tx.readError
	}
	return tx.strings[pageSplitLocation{block: block, offset: offset}], nil
}

func (tx *pageSplitTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	write := pageSplitWrite{location: pageSplitLocation{block: block, offset: int(offset)}, intValue: value, logged: logged}
	tx.writes = append(tx.writes, write)
	if len(tx.writes) == tx.writeErrorAt {
		return tx.writeError
	}
	tx.ints[write.location] = value
	return nil
}

func (tx *pageSplitTransaction) SetString(block storage.BlockID, offset int32, value string, logged bool) error {
	write := pageSplitWrite{location: pageSplitLocation{block: block, offset: int(offset)}, textValue: value, isString: true, logged: logged}
	tx.writes = append(tx.writes, write)
	if len(tx.writes) == tx.writeErrorAt {
		return tx.writeError
	}
	tx.strings[write.location] = value
	return nil
}

func TestPageSplitMovesSuffixAndPreservesSourcePrefixAndFlag(t *testing.T) {
	page, transaction, source, destination, layout := newPageForSplit(t, record.IntegerField, 4)
	transaction.ints[pageSplitLocation{block: source, offset: 0}] = -2
	for slot, item := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}, {40, 400}} {
		transaction.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = item.key
		transaction.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = item.child
	}

	block, err := page.Split(2, 7)
	if err != nil {
		t.Fatalf("Split(2) error = %v", err)
	}
	if block != destination {
		t.Fatalf("Split(2) block = %v, want %v", block, destination)
	}
	if got := transaction.ints[pageSplitLocation{block: source, offset: 4}]; got != 2 {
		t.Fatalf("source count = %d, want 2", got)
	}
	if got := transaction.ints[pageSplitLocation{block: source, offset: 0}]; got != -2 {
		t.Fatalf("source flag = %d, want unchanged -2", got)
	}
	for slot, want := range []struct{ key, child int32 }{{10, 100}, {20, 200}} {
		if got := transaction.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != want.key {
			t.Errorf("source slot %d key = %d, want %d", slot, got, want.key)
		}
		if got := transaction.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}]; got != want.child {
			t.Errorf("source slot %d child = %d, want %d", slot, got, want.child)
		}
	}
	if got := transaction.ints[pageSplitLocation{block: destination, offset: 4}]; got != 2 {
		t.Fatalf("destination count = %d, want 2", got)
	}
	if got := transaction.ints[pageSplitLocation{block: destination, offset: 0}]; got != 7 {
		t.Fatalf("destination flag = %d, want 7", got)
	}
	for slot, want := range []struct{ key, child int32 }{{30, 300}, {40, 400}} {
		if got := transaction.ints[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != want.key {
			t.Errorf("destination slot %d key = %d, want %d", slot, got, want.key)
		}
		if got := transaction.ints[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "block")}]; got != want.child {
			t.Errorf("destination slot %d child = %d, want %d", slot, got, want.child)
		}
	}
	if len(transaction.appendCalls) != 1 || transaction.appendCalls[0] != source.FileName() {
		t.Fatalf("append calls = %v, want one append to %q", transaction.appendCalls, source.FileName())
	}
	if len(transaction.pinCalls) != 1 || transaction.pinCalls[0] != destination {
		t.Fatalf("pin calls = %v, want destination only", transaction.pinCalls)
	}
	if len(transaction.unpinCalls) != 1 || transaction.unpinCalls[0] != destination {
		t.Fatalf("unpin calls = %v, want destination only", transaction.unpinCalls)
	}
	if page.block != source {
		t.Fatalf("source page block = %v, want unchanged %v", page.block, source)
	}
	for i, write := range transaction.writes {
		if i < 10 && write.logged {
			t.Errorf("format write %d logged unexpectedly: %+v", i, write)
		}
		if i >= 10 && !write.logged {
			t.Errorf("record transfer write %d was not logged: %+v", i, write)
		}
	}
}

func TestPageSplitAcceptsZeroAndCountBoundaries(t *testing.T) {
	for _, splitSlot := range []int{0, 1} {
		t.Run(fmt.Sprint(splitSlot), func(t *testing.T) {
			page, transaction, source, destination, layout := newPageForSplit(t, record.VarcharField, 1)
			transaction.ints[pageSplitLocation{block: source, offset: 4}] = 1
			transaction.strings[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "alpha"
			transaction.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, 0, "block")}] = 31

			block, err := page.Split(splitSlot, -4)
			if err != nil {
				t.Fatalf("Split(%d) error = %v", splitSlot, err)
			}
			if block != destination {
				t.Fatalf("Split(%d) block = %v, want %v", splitSlot, block, destination)
			}
			sourceCount := transaction.ints[pageSplitLocation{block: source, offset: 4}]
			destinationCount := transaction.ints[pageSplitLocation{block: destination, offset: 4}]
			if int(sourceCount) != splitSlot || int(destinationCount) != 1-splitSlot {
				t.Fatalf("source/destination counts = %d/%d, want %d/%d", sourceCount, destinationCount, splitSlot, 1-splitSlot)
			}
			if transaction.ints[pageSplitLocation{block: destination, offset: 0}] != -4 {
				t.Fatal("destination flag was not initialized")
			}
			if destinationCount == 1 {
				if got := transaction.strings[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, 0, "dataval")}]; got != "alpha" {
					t.Fatalf("destination key = %q, want alpha", got)
				}
			}
		})
	}
}

func TestPageSplitWrapsAppendPinReadWriteAndCloseFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		configure  func(*pageSplitTransaction, storage.BlockID)
		wantAppend int
		wantPin    int
		wantUnpin  int
	}{
		{name: "append", configure: func(tx *pageSplitTransaction, _ storage.BlockID) { tx.appendErr = errors.New("append failed") }, wantAppend: 1},
		{name: "destination pin", configure: func(tx *pageSplitTransaction, _ storage.BlockID) { tx.pinErr = errors.New("pin failed") }, wantAppend: 1, wantPin: 1},
		{name: "record read", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.readErrorAt = 2
			tx.readError = errors.New("record read failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "record read and destination unpin", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.readErrorAt = 2
			tx.readError = errors.New("record read failed")
			tx.unpinErr = errors.New("destination unpin failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "destination record write", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.writeErrorAt = 5
			tx.writeError = errors.New("destination write failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "destination count write", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.writeErrorAt = 7
			tx.writeError = errors.New("destination count failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "source deletion count write", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.writeErrorAt = 8
			tx.writeError = errors.New("source count failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "destination flag write", configure: func(tx *pageSplitTransaction, _ storage.BlockID) {
			tx.writeErrorAt = 9
			tx.writeError = errors.New("destination flag failed")
		}, wantAppend: 1, wantPin: 1, wantUnpin: 1},
		{name: "destination unpin", configure: func(tx *pageSplitTransaction, _ storage.BlockID) { tx.unpinErr = errors.New("unpin failed") }, wantAppend: 1, wantPin: 1, wantUnpin: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, transaction, _, _, _ := newPageForSplit(t, record.IntegerField, 1)
			test.configure(transaction, storage.BlockID{})
			cause := transactionErrorCause(transaction)
			block, err := page.Split(0, 6)
			if !errors.Is(err, cause) {
				t.Fatalf("Split() error = %v, want wrapped cause %v", err, cause)
			}
			if block != (storage.BlockID{}) {
				t.Fatalf("Split() block = %v, want zero block on failure", block)
			}
			if len(transaction.appendCalls) != test.wantAppend || len(transaction.pinCalls) != test.wantPin || len(transaction.unpinCalls) != test.wantUnpin {
				t.Fatalf("append/pin/unpin calls = %d/%d/%d, want %d/%d/%d", len(transaction.appendCalls), len(transaction.pinCalls), len(transaction.unpinCalls), test.wantAppend, test.wantPin, test.wantUnpin)
			}
		})
	}
}

func TestPageSplitWrapsVarcharTransferFailures(t *testing.T) {
	for _, test := range []struct {
		name         string
		readErrorAt  int
		writeErrorAt int
	}{
		{name: "read", readErrorAt: 2},
		{name: "write", writeErrorAt: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, transaction, source, destination, layout := newPageForSplit(t, record.VarcharField, 1)
			transaction.ints[pageSplitLocation{block: source, offset: 4}] = 1
			transaction.strings[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "alpha"
			cause := errors.New("varchar transfer failed")
			transaction.readErrorAt = test.readErrorAt
			transaction.readError = cause
			transaction.writeErrorAt = test.writeErrorAt
			transaction.writeError = cause

			block, err := page.Split(0, 1)
			if !errors.Is(err, cause) {
				t.Fatalf("Split() error = %v, want wrapped varchar failure", err)
			}
			if block != (storage.BlockID{}) {
				t.Fatalf("Split() block = %v, want zero block on failure", block)
			}
			if len(transaction.unpinCalls) != 1 || transaction.unpinCalls[0] != destination {
				t.Fatalf("unpin calls = %v, want destination after failure", transaction.unpinCalls)
			}
		})
	}
}

func TestPageSplitRejectsInvalidStateAndRangeBeforeAppending(t *testing.T) {
	layout := pageInsertDirLayout(record.IntegerField)
	validBlock := storage.NewBlockID("directory.idx", 2)
	var typedNil *pageSplitTransaction
	makeValid := func() (*Page, *pageSplitTransaction) {
		page, tx, _, _, _ := newPageForSplit(t, record.IntegerField, 1)
		return page, tx
	}
	for _, test := range []struct {
		name    string
		newPage func() (*Page, *pageSplitTransaction)
	}{
		{name: "nil page"},
		{name: "closed page", newPage: func() (*Page, *pageSplitTransaction) {
			tx := newPageSplitTransaction(layout, 1)
			return &Page{transaction: tx, layout: layout}, tx
		}},
		{name: "negative current block", newPage: func() (*Page, *pageSplitTransaction) {
			tx := newPageSplitTransaction(layout, 1)
			return &Page{transaction: tx, block: storage.NewBlockID("directory.idx", -1), layout: layout}, tx
		}},
		{name: "nil transaction", newPage: func() (*Page, *pageSplitTransaction) {
			return &Page{block: validBlock, layout: layout}, nil
		}},
		{name: "typed nil transaction", newPage: func() (*Page, *pageSplitTransaction) {
			return &Page{transaction: typedNil, block: validBlock, layout: layout}, nil
		}},
		{name: "nil layout", newPage: func() (*Page, *pageSplitTransaction) {
			tx := newPageSplitTransaction(layout, 1)
			return &Page{transaction: tx, block: validBlock}, tx
		}},
		{name: "invalid schema", newPage: func() (*Page, *pageSplitTransaction) {
			tx := newPageSplitTransaction(layout, 1)
			return &Page{transaction: tx, block: validBlock, layout: pageFormatLayoutWithNoFields()}, tx
		}},
		{name: "malformed layout", newPage: func() (*Page, *pageSplitTransaction) {
			tx := newPageSplitTransaction(layout, 1)
			return &Page{transaction: tx, block: validBlock, layout: pageDeleteLayoutMissingFieldOffset()}, tx
		}},
		{name: "negative count", newPage: func() (*Page, *pageSplitTransaction) {
			page, tx := makeValid()
			tx.ints[pageSplitLocation{block: page.block, offset: 4}] = -1
			return page, tx
		}},
		{name: "count read failure", newPage: func() (*Page, *pageSplitTransaction) {
			page, tx := makeValid()
			tx.readErrorAt = 1
			tx.readError = errors.New("count read failed")
			return page, tx
		}},
		{name: "block size failure", newPage: func() (*Page, *pageSplitTransaction) {
			page, tx := makeValid()
			tx.blockSizeErr = errors.New("block size failed")
			return page, tx
		}},
		{name: "block smaller than header", newPage: func() (*Page, *pageSplitTransaction) {
			page, tx := makeValid()
			tx.blockSize = 7
			return page, tx
		}},
		{name: "negative split slot", newPage: makeValid},
		{name: "split slot beyond count", newPage: makeValid},
		{name: "suffix exceeds capacity", newPage: func() (*Page, *pageSplitTransaction) {
			page, tx := makeValid()
			tx.ints[pageSplitLocation{block: page.block, offset: 4}] = 2
			tx.blockSize = 8 + layout.SlotSize()
			return page, tx
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var page *Page
			var tx *pageSplitTransaction
			if test.newPage != nil {
				page, tx = test.newPage()
			}
			if page == nil {
				if block, err := page.Split(0, 1); err == nil || block != (storage.BlockID{}) {
					t.Fatalf("Split() = (%v, %v), want zero block and error", block, err)
				}
				return
			}
			splitSlot := 0
			if test.name == "negative split slot" {
				splitSlot = -1
			}
			if test.name == "split slot beyond count" {
				splitSlot = 2
			}
			if block, err := page.Split(splitSlot, 1); err == nil || block != (storage.BlockID{}) {
				t.Fatalf("Split(%d) = (%v, %v), want zero block and error", splitSlot, block, err)
			}
			if tx != nil && len(tx.appendCalls) != 0 {
				t.Fatalf("Append calls = %v, want no append before validation", tx.appendCalls)
			}
		})
	}
}

func TestPageSplitChecksOffsetRangeBeforeAppending(t *testing.T) {
	slotSize := int(math.MaxInt32) + 64
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	layout := record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0, "block": 4}, slotSize)
	block := storage.NewBlockID("directory.idx", 0)
	tx := newPageSplitTransaction(layout, 2)
	tx.ints[pageSplitLocation{block: block, offset: 4}] = 2
	tx.blockSize = math.MaxInt
	page := &Page{transaction: tx, block: block, layout: layout}
	if got, err := page.Split(0, 1); err == nil || got != (storage.BlockID{}) {
		t.Fatalf("Split() = (%v, %v), want offset validation error", got, err)
	}
	if len(tx.appendCalls) != 0 {
		t.Fatalf("Append calls = %v, want no append before offset validation", tx.appendCalls)
	}

	overflowLayout := pageDeleteLayoutWithSlotSize(math.MaxInt)
	overflowBlock := storage.NewBlockID("directory.idx", 0)
	overflowTransaction := newPageSplitTransaction(overflowLayout, 2)
	overflowTransaction.ints[pageSplitLocation{block: overflowBlock, offset: 4}] = 2
	overflowTransaction.blockSize = math.MaxInt
	overflowPage := &Page{transaction: overflowTransaction, block: overflowBlock, layout: overflowLayout}
	if got, err := overflowPage.Split(2, 1); err == nil || got != (storage.BlockID{}) {
		t.Fatalf("Split() = (%v, %v), want machine-int offset validation error", got, err)
	}
	if len(overflowTransaction.appendCalls) != 0 {
		t.Fatalf("Append calls = %v, want none after machine-int overflow", overflowTransaction.appendCalls)
	}
}

func newPageForSplit(t *testing.T, keyType record.FieldType, count int) (*Page, *pageSplitTransaction, storage.BlockID, storage.BlockID, *record.Layout) {
	t.Helper()
	layout := pageInsertDirLayout(keyType)
	source := storage.NewBlockID("directory.idx", 2)
	destination := storage.NewBlockID("directory.idx", 7)
	tx := newPageSplitTransaction(layout, count)
	tx.appendedBlock = destination
	tx.ints[pageSplitLocation{block: source, offset: 4}] = int32(count)
	return &Page{transaction: tx, block: source, layout: layout}, tx, source, destination, layout
}

func newPageSplitTransaction(layout *record.Layout, slots int) *pageSplitTransaction {
	return &pageSplitTransaction{
		blockSize:     8 + slots*layout.SlotSize(),
		ints:          make(map[pageSplitLocation]int32),
		strings:       make(map[pageSplitLocation]string),
		appendedBlock: storage.NewBlockID("directory.idx", 7),
	}
}

func transactionErrorCause(tx *pageSplitTransaction) error {
	switch {
	case tx.appendErr != nil:
		return tx.appendErr
	case tx.pinErr != nil:
		return tx.pinErr
	case tx.readError != nil:
		return tx.readError
	case tx.writeError != nil:
		return tx.writeError
	case tx.unpinErr != nil:
		return tx.unpinErr
	default:
		return errors.New("no configured transaction error")
	}
}
