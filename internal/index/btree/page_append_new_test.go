package btree

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageAppendNewTransaction struct {
	*pageFormatTransaction
	appendedBlock storage.BlockID
	appendErr     error
	appendCalls   []string
}

func (tx *pageAppendNewTransaction) Append(fileName string) (storage.BlockID, error) {
	tx.appendCalls = append(tx.appendCalls, fileName)
	return tx.appendedBlock, tx.appendErr
}

func TestPageAppendNewAppendsCurrentFileThenFormatsWithoutPinning(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	transaction := &pageAppendNewTransaction{
		pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
		appendedBlock:         storage.NewBlockID("index.idx", 5),
	}
	page := &Page{transaction: transaction, block: storage.NewBlockID("index.idx", 2), layout: layout}

	block, err := page.AppendNew(-3)
	if err != nil {
		t.Fatalf("AppendNew() error = %v", err)
	}
	if block != transaction.appendedBlock {
		t.Fatalf("AppendNew() block = %v, want %v", block, transaction.appendedBlock)
	}
	if !reflect.DeepEqual(transaction.appendCalls, []string{"index.idx"}) {
		t.Fatalf("Append calls = %v, want [index.idx]", transaction.appendCalls)
	}
	wantWrites := []pageInsertDirWrite{
		{block: block, offset: 0, intValue: -3},
		{block: block, offset: 4, intValue: 0},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 0, "dataval")), intValue: 0},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 0, "block")), intValue: 0},
		{block: block, offset: int32(pageInsertLeafPosition(layout, 0, "id")), intValue: 0},
	}
	if !reflect.DeepEqual(transaction.writes, wantWrites) {
		t.Fatalf("format writes = %+v, want %+v", transaction.writes, wantWrites)
	}
	for i, write := range transaction.writes {
		if write.logged {
			t.Errorf("format write %d requested transaction logging", i)
		}
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 {
		t.Fatal("AppendNew changed the caller-owned pin lifecycle")
	}
}

func TestPageAppendNewPreservesAppendFailure(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	cause := errors.New("append failed")
	transaction := &pageAppendNewTransaction{
		pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
		appendErr:             cause,
	}
	page := &Page{transaction: transaction, block: storage.NewBlockID("index.idx", 2), layout: layout}

	block, err := page.AppendNew(1)
	if !errors.Is(err, cause) {
		t.Fatalf("AppendNew() error = %v, want wrapped append failure", err)
	}
	if block != (storage.BlockID{}) {
		t.Fatalf("AppendNew() block = %v, want zero block", block)
	}
	if !reflect.DeepEqual(transaction.appendCalls, []string{"index.idx"}) {
		t.Fatalf("Append calls = %v, want [index.idx]", transaction.appendCalls)
	}
	if len(transaction.writes) != 0 {
		t.Fatalf("format writes = %+v, want none after append failure", transaction.writes)
	}
}

func TestPageAppendNewDoesNotPublishBlockWhenFormattingFails(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	cause := errors.New("format count write failed")
	transaction := &pageAppendNewTransaction{
		pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
		appendedBlock:         storage.NewBlockID("index.idx", 5),
	}
	transaction.writeErrorAt = 2
	transaction.writeError = cause
	page := &Page{transaction: transaction, block: storage.NewBlockID("index.idx", 2), layout: layout}

	block, err := page.AppendNew(1)
	if !errors.Is(err, cause) {
		t.Fatalf("AppendNew() error = %v, want wrapped format failure", err)
	}
	if block != (storage.BlockID{}) {
		t.Fatalf("AppendNew() block = %v, want zero block after partial format", block)
	}
	if !reflect.DeepEqual(transaction.appendCalls, []string{"index.idx"}) {
		t.Fatalf("Append calls = %v, want [index.idx]", transaction.appendCalls)
	}
	if len(transaction.writes) != 2 {
		t.Fatalf("format writes = %+v, want one successful write followed by one failed write", transaction.writes)
	}
	if transaction.pinCount != 0 || transaction.unpinCount != 0 {
		t.Fatal("AppendNew changed the caller-owned pin lifecycle")
	}
}

func TestPageAppendNewRejectsInvalidDependenciesBeforeAppending(t *testing.T) {
	layout := pageInsertLeafLayout(record.IntegerField)
	validBlock := storage.NewBlockID("index.idx", 2)
	var typedNil *pageAppendNewTransaction
	validTransaction := func() *pageAppendNewTransaction {
		return &pageAppendNewTransaction{
			pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
			appendedBlock:         storage.NewBlockID("index.idx", 5),
		}
	}
	tests := []struct {
		name    string
		newPage func() (*Page, *pageAppendNewTransaction)
	}{
		{name: "nil page"},
		{name: "closed page", newPage: func() (*Page, *pageAppendNewTransaction) {
			tx := validTransaction()
			return &Page{transaction: tx, block: storage.BlockID{}, layout: layout}, tx
		}},
		{name: "negative current block", newPage: func() (*Page, *pageAppendNewTransaction) {
			tx := validTransaction()
			return &Page{transaction: tx, block: storage.NewBlockID("index.idx", -1), layout: layout}, tx
		}},
		{name: "nil transaction", newPage: func() (*Page, *pageAppendNewTransaction) {
			return &Page{block: validBlock, layout: layout}, nil
		}},
		{name: "typed nil transaction", newPage: func() (*Page, *pageAppendNewTransaction) {
			return &Page{transaction: typedNil, block: validBlock, layout: layout}, nil
		}},
		{name: "nil layout", newPage: func() (*Page, *pageAppendNewTransaction) {
			tx := validTransaction()
			return &Page{transaction: tx, block: validBlock}, tx
		}},
		{name: "malformed layout", newPage: func() (*Page, *pageAppendNewTransaction) {
			tx := validTransaction()
			return &Page{transaction: tx, block: validBlock, layout: pageDeleteLayoutMissingFieldOffset()}, tx
		}},
		{name: "layout has no fields", newPage: func() (*Page, *pageAppendNewTransaction) {
			tx := validTransaction()
			return &Page{transaction: tx, block: validBlock, layout: pageFormatLayoutWithNoFields()}, tx
		}},
		{name: "transaction lacks append", newPage: func() (*Page, *pageAppendNewTransaction) {
			return &Page{transaction: newPageFormatTransaction(layout, 8+layout.SlotSize()), block: validBlock, layout: layout}, nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var page *Page
			var transaction *pageAppendNewTransaction
			if test.newPage != nil {
				page, transaction = test.newPage()
			}
			if block, err := page.AppendNew(1); err == nil || block != (storage.BlockID{}) {
				t.Fatalf("AppendNew() = (%v, %v), want zero block and error", block, err)
			}
			if transaction != nil {
				if len(transaction.appendCalls) != 0 {
					t.Fatalf("Append calls = %v, want none before validation", transaction.appendCalls)
				}
				if len(transaction.writes) != 0 {
					t.Fatalf("format writes = %+v, want none before validation", transaction.writes)
				}
			}
		})
	}
}

func TestPageAppendNewRejectsInvalidBlockReturnedByAppend(t *testing.T) {
	for _, block := range []storage.BlockID{storage.NewBlockID("", 0), storage.NewBlockID("index.idx", -1)} {
		t.Run(block.String(), func(t *testing.T) {
			layout := pageInsertLeafLayout(record.IntegerField)
			transaction := &pageAppendNewTransaction{
				pageFormatTransaction: newPageFormatTransaction(layout, 8+layout.SlotSize()),
				appendedBlock:         block,
			}
			page := &Page{transaction: transaction, block: storage.NewBlockID("index.idx", 2), layout: layout}

			result, err := page.AppendNew(1)
			if err == nil || result != (storage.BlockID{}) {
				t.Fatalf("AppendNew() = (%v, %v), want zero block and validation error", result, err)
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("format writes = %+v, want none for invalid appended block", transaction.writes)
			}
		})
	}
}
