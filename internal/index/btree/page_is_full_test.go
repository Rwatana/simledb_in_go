package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageCapacityTransaction struct {
	blockSize      int
	blockSizeError error
	blockSizeReads int
	integerValues  map[int]int32
	integerErrors  map[int]error
}

func (*pageCapacityTransaction) Pin(storage.BlockID) error { return nil }
func (*pageCapacityTransaction) Unpin(storage.BlockID) error {
	return nil
}

func (tx *pageCapacityTransaction) BlockSize() (int, error) {
	tx.blockSizeReads++
	return tx.blockSize, tx.blockSizeError
}

func (tx *pageCapacityTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	if err := tx.integerErrors[offset]; err != nil {
		return 0, err
	}
	return tx.integerValues[offset], nil
}

func (*pageCapacityTransaction) GetString(storage.BlockID, int) (string, error) {
	return "", nil
}
func (*pageCapacityTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*pageCapacityTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageIsFullChecksTheNextRecordBoundary(t *testing.T) {
	for _, test := range []struct {
		name      string
		records   int32
		extraSize int
		want      bool
	}{
		{name: "next slot fits", records: 0, extraSize: 1, want: false},
		{name: "next slot starts at block boundary", records: 0, want: true},
		{name: "next slot begins at boundary", records: 2, want: true},
		{name: "records already exceed block capacity", records: 3, extraSize: -1, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := pageCapacityLayout()
			nextSlotStart := 2*4 + (int(test.records)+1)*layout.SlotSize()
			tx := &pageCapacityTransaction{
				blockSize:     nextSlotStart + test.extraSize,
				integerValues: map[int]int32{4: test.records},
			}
			page := newPageForCapacity(t, tx, layout)

			got, err := page.IsFull()
			if err != nil || got != test.want {
				t.Fatalf("IsFull() = (%t, %v), want (%t, nil)", got, err, test.want)
			}
			if tx.blockSizeReads != 1 {
				t.Fatalf("BlockSize calls = %d, want one", tx.blockSizeReads)
			}
		})
	}
}

func TestPageIsFullRejectsInvalidPageAndLayoutState(t *testing.T) {
	validBlock := storage.NewBlockID("students.idx", 0)
	var nilTransaction *pageCapacityTransaction

	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: validBlock, layout: pageCapacityLayout()}},
		{name: "typed-nil transaction", page: &Page{transaction: nilTransaction, block: validBlock, layout: pageCapacityLayout()}},
		{name: "nil layout", page: &Page{transaction: &pageCapacityTransaction{}, block: validBlock}},
		{name: "nil schema", page: &Page{transaction: &pageCapacityTransaction{}, block: validBlock, layout: record.NewLayoutFromMetadata(nil, nil, 8)}},
		{name: "invalid slot size", page: &Page{transaction: &pageCapacityTransaction{}, block: validBlock, layout: record.NewLayoutFromMetadata(pageCapacitySchema(), map[string]int{"dataval": 0}, 0)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.IsFull(); err == nil {
				t.Fatal("IsFull() error = nil, want validation error")
			}
		})
	}
}

func TestPageIsFullWrapsRecordCountAndBlockSizeErrors(t *testing.T) {
	countReadError := errors.New("record count read failed")
	countTx := &pageCapacityTransaction{
		integerErrors: map[int]error{4: countReadError},
		integerValues: map[int]int32{},
		blockSize:     128,
	}
	countPage := newPageForCapacity(t, countTx, pageCapacityLayout())
	if _, err := countPage.IsFull(); !errors.Is(err, countReadError) {
		t.Fatalf("record count error = %v, want wrapped cause", err)
	}
	if countTx.blockSizeReads != 0 {
		t.Fatalf("BlockSize calls after failed count read = %d, want zero", countTx.blockSizeReads)
	}

	blockSizeError := errors.New("block size read failed")
	blockTx := &pageCapacityTransaction{
		integerValues:  map[int]int32{4: 0},
		blockSizeError: blockSizeError,
		blockSize:      128,
	}
	blockPage := newPageForCapacity(t, blockTx, pageCapacityLayout())
	if _, err := blockPage.IsFull(); !errors.Is(err, blockSizeError) {
		t.Fatalf("block size error = %v, want wrapped cause", err)
	}
	if blockTx.blockSizeReads != 1 {
		t.Fatalf("BlockSize calls = %d, want one", blockTx.blockSizeReads)
	}
}

func TestPageIsFullRejectsMalformedCountsSizesAndOverflow(t *testing.T) {
	negativeCount := newPageForCapacity(t, &pageCapacityTransaction{
		integerValues: map[int]int32{4: -1},
		blockSize:     128,
	}, pageCapacityLayout())
	if _, err := negativeCount.IsFull(); err == nil {
		t.Fatal("IsFull() with negative record count error = nil, want error")
	}

	for _, blockSize := range []int{0, -1} {
		page := newPageForCapacity(t, &pageCapacityTransaction{
			integerValues: map[int]int32{4: 0},
			blockSize:     blockSize,
		}, pageCapacityLayout())
		if _, err := page.IsFull(); err == nil {
			t.Fatalf("IsFull() with block size %d error = nil, want error", blockSize)
		}
	}

	maxInt := int(^uint(0) >> 1)
	largeLayout := record.NewLayoutFromMetadata(pageCapacitySchema(), map[string]int{"dataval": 0}, maxInt)
	overflowPage := newPageForCapacity(t, &pageCapacityTransaction{
		integerValues: map[int]int32{4: 1},
		blockSize:     maxInt,
	}, largeLayout)
	if _, err := overflowPage.IsFull(); err == nil {
		t.Fatal("IsFull() with overflowing next-slot position error = nil, want error")
	}
}

func newPageForCapacity(t *testing.T, transaction *pageCapacityTransaction, layout *record.Layout) *Page {
	t.Helper()
	page, err := NewPage(transaction, storage.NewBlockID("students.idx", 0), layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	return page
}

func pageCapacityLayout() *record.Layout {
	return record.NewLayout(pageCapacitySchema())
}

func pageCapacitySchema() *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	return &schema
}
