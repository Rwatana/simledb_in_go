package btree

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageFormatTransaction struct {
	*pageInsertDirTransaction
	blockSize    int
	blockSizeErr error
}

func (tx *pageFormatTransaction) BlockSize() (int, error) {
	return tx.blockSize, tx.blockSizeErr
}

func TestPageFormatWritesFlagCountAndDefaultVarcharSlotsWithoutLogging(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	blockSize := 8 + 2*layout.SlotSize() + layout.SlotSize()/2
	transaction := &pageFormatTransaction{
		pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 9),
		blockSize:                blockSize,
	}
	currentBlock := storage.NewBlockID("current.idx", 1)
	page := &Page{transaction: transaction, block: currentBlock, layout: layout}
	target := storage.NewBlockID("leaf.idx", 7)

	if err := page.Format(target, -3); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	wantWrites := []pageInsertDirWrite{
		{block: target, offset: 0, intValue: -3},
		{block: target, offset: 4, intValue: 0},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 0, "dataval")), textValue: "", isString: true},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 0, "block")), intValue: 0},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 0, "id")), intValue: 0},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 1, "dataval")), textValue: "", isString: true},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 1, "block")), intValue: 0},
		{block: target, offset: int32(pageInsertLeafPosition(layout, 1, "id")), intValue: 0},
	}
	if len(transaction.writes) != len(wantWrites) {
		t.Fatalf("writes = %+v, want %+v", transaction.writes, wantWrites)
	}
	for i := range wantWrites {
		if transaction.writes[i] != wantWrites[i] {
			t.Errorf("write %d = %+v, want %+v", i, transaction.writes[i], wantWrites[i])
		}
		if transaction.writes[i].logged {
			t.Errorf("format write %d requested transaction logging", i)
		}
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 {
		t.Fatal("Format changed the caller-owned pin lifecycle")
	}
	if page.block != currentBlock {
		t.Fatalf("current page block = %v, want unchanged %v", page.block, currentBlock)
	}
}

func TestPageFormatWithNoCompleteSlotsWritesOnlyHeader(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	transaction := &pageFormatTransaction{
		pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 2),
		blockSize:                8,
	}
	page := &Page{transaction: transaction, layout: layout}
	block := storage.NewBlockID("empty.idx", 0)

	if err := page.Format(block, 12); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	want := []pageInsertDirWrite{{block: block, offset: 0, intValue: 12}, {block: block, offset: 4, intValue: 0}}
	if len(transaction.writes) != len(want) {
		t.Fatalf("writes = %+v, want %+v", transaction.writes, want)
	}
	for i := range want {
		if transaction.writes[i] != want[i] {
			t.Errorf("write %d = %+v, want %+v", i, transaction.writes[i], want[i])
		}
	}
}

func TestPageFormatRejectsInvalidStateLayoutAndTargetBeforeMutation(t *testing.T) {
	validLayout := pageInsertLeafLayout(record.IntegerField)
	validBlock := storage.NewBlockID("target.idx", 0)
	var typedNil *pageFormatTransaction
	missingOffset := pageDeleteLayoutMissingFieldOffset()
	zeroSlot := pageDeleteLayoutWithSlotSize(0)
	noFields := pageFormatLayoutWithNoFields()
	tests := []struct {
		name    string
		block   storage.BlockID
		newPage func() (*Page, *pageFormatTransaction)
	}{
		{name: "nil page", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return nil, nil }},
		{name: "nil transaction", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return &Page{layout: validLayout}, nil }},
		{name: "typed nil transaction", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return &Page{transaction: typedNil, layout: validLayout}, nil }},
		{name: "nil layout", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) {
			tx := newPageFormatTransaction(validLayout, 8)
			return &Page{transaction: tx, layout: nil}, tx
		}},
		{name: "empty target file", block: storage.NewBlockID("", 0), newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(validLayout, 8) }},
		{name: "negative target block", block: storage.NewBlockID("target.idx", -1), newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(validLayout, 8) }},
		{name: "missing field offset", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(missingOffset, 8) }},
		{name: "zero slot size", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(zeroSlot, 8) }},
		{name: "no schema fields", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(noFields, 8) }},
		{name: "block smaller than header", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) { return newPageFormatPage(validLayout, 7) }},
		{name: "int32 field offset overflow", block: validBlock, newPage: func() (*Page, *pageFormatTransaction) {
			layout := pageDeleteLayoutWithSlotSize(int(math.MaxInt32) + 20)
			return newPageFormatPage(layout, 8+2*layout.SlotSize())
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, transaction := test.newPage()
			if err := page.Format(test.block, 1); err == nil {
				t.Fatal("Format() error = nil, want validation error")
			}
			if transaction != nil && transaction.pageInsertDirTransaction != nil && len(transaction.writes) != 0 {
				t.Fatalf("writes = %+v, want no mutation", transaction.writes)
			}
		})
	}
}

func newPageFormatTransaction(layout *record.Layout, blockSize int) *pageFormatTransaction {
	return &pageFormatTransaction{pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0), blockSize: blockSize}
}

func newPageFormatPage(layout *record.Layout, blockSize int) (*Page, *pageFormatTransaction) {
	transaction := newPageFormatTransaction(layout, blockSize)
	return &Page{transaction: transaction, layout: layout}, transaction
}

func TestPageFormatWrapsBlockSizeAndWriteFailures(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	block := storage.NewBlockID("target.idx", 1)
	sizeErr := errors.New("block size failed")
	transaction := &pageFormatTransaction{
		pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0),
		blockSizeErr:             sizeErr,
	}
	page := &Page{transaction: transaction, layout: layout}
	if err := page.Format(block, 1); !errors.Is(err, sizeErr) {
		t.Fatalf("block-size error = %v, want wrapped cause", err)
	}
	if len(transaction.writes) != 0 {
		t.Fatalf("writes = %+v, want no mutation after block-size failure", transaction.writes)
	}

	for writeError := 1; writeError <= 8; writeError++ {
		t.Run(fmt.Sprint(writeError), func(t *testing.T) {
			cause := errors.New("format write failed")
			transaction := &pageFormatTransaction{
				pageInsertDirTransaction: pageInsertDirTransactionWithCount(layout, 0),
				blockSize:                8 + 2*layout.SlotSize(),
			}
			transaction.writeErrorAt = writeError
			transaction.writeError = cause
			page := &Page{transaction: transaction, layout: layout}
			if err := page.Format(block, 1); !errors.Is(err, cause) {
				t.Fatalf("Format() error = %v, want wrapped cause", err)
			}
		})
	}
}

func TestPageFormatWrapsVarcharDefaultWriteFailure(t *testing.T) {
	layout := pageInsertLeafLayout(record.VarcharField)
	cause := errors.New("varchar default write failed")
	transaction := newPageFormatTransaction(layout, 8+layout.SlotSize())
	transaction.writeErrorAt = 3
	transaction.writeError = cause
	page := &Page{transaction: transaction, layout: layout}
	block := storage.NewBlockID("target.idx", 1)
	if err := page.Format(block, 1); !errors.Is(err, cause) {
		t.Fatalf("Format() error = %v, want wrapped varchar write cause", err)
	}
}

func pageFormatLayoutWithSlotSize(slotSize int) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	return record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0, "block": 4, "id": 8}, slotSize)
}

func pageFormatLayoutWithNoFields() *record.Layout {
	var schema record.Schema
	return record.NewLayoutFromMetadata(&schema, map[string]int{}, 12)
}
