package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageSplitDirectoryPageReturnsSeparatorAndPreservesIntegerRanges(t *testing.T) {
	page, tx, source, destination, layout := newPageForSplit(t, record.IntegerField, 4)
	tx.ints[pageSplitLocation{block: source, offset: 0}] = 2
	for slot, row := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}, {40, 400}} {
		tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = row.key
		tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = row.child
	}

	entry, err := page.SplitDirectoryPage()
	if err != nil {
		t.Fatalf("SplitDirectoryPage() error = %v", err)
	}
	if key, ok := entry.DataVal().AsInt(); !ok || key != 30 {
		t.Fatalf("separator key = %v, want 30", entry.DataVal())
	}
	if entry.BlockNumber() != destination.Number() {
		t.Fatalf("separator block = %d, want %d", entry.BlockNumber(), destination.Number())
	}
	if got := tx.ints[pageSplitLocation{block: source, offset: 4}]; got != 2 {
		t.Fatalf("source count = %d, want 2", got)
	}
	if got := tx.ints[pageSplitLocation{block: destination, offset: 4}]; got != 2 {
		t.Fatalf("destination count = %d, want 2", got)
	}
	for slot, row := range []struct{ key, child int32 }{{10, 100}, {20, 200}} {
		if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != row.key {
			t.Errorf("source slot %d key = %d, want %d", slot, got, row.key)
		}
		if got := tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}]; got != row.child {
			t.Errorf("source slot %d child = %d, want %d", slot, got, row.child)
		}
	}
	for slot, row := range []struct{ key, child int32 }{{30, 300}, {40, 400}} {
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != row.key {
			t.Errorf("destination slot %d key = %d, want %d", slot, got, row.key)
		}
		if got := tx.ints[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "block")}]; got != row.child {
			t.Errorf("destination slot %d child = %d, want %d", slot, got, row.child)
		}
	}
	if tx.ints[pageSplitLocation{block: source, offset: 0}] != 2 || tx.ints[pageSplitLocation{block: destination, offset: 0}] != 2 {
		t.Fatalf("directory flags source=%d destination=%d, want both 2", tx.ints[pageSplitLocation{block: source, offset: 0}], tx.ints[pageSplitLocation{block: destination, offset: 0}])
	}
	if len(tx.appendCalls) != 1 || len(tx.pinCalls) != 1 || len(tx.unpinCalls) != 1 || tx.pinCalls[0] != destination || tx.unpinCalls[0] != destination {
		t.Fatalf("destination lifecycle append=%v pin=%v unpin=%v", tx.appendCalls, tx.pinCalls, tx.unpinCalls)
	}
	if page.block != source {
		t.Fatalf("source page block = %v, want %v", page.block, source)
	}
}

func TestPageSplitDirectoryPageSupportsVarcharAndOddCounts(t *testing.T) {
	page, tx, source, destination, layout := newPageForSplit(t, record.VarcharField, 3)
	for slot, row := range []struct {
		key   string
		child int32
	}{{"alpha", 10}, {"middle", 20}, {"zulu", 30}} {
		tx.strings[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = row.key
		tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = row.child
	}

	entry, err := page.SplitDirectoryPage()
	if err != nil {
		t.Fatalf("SplitDirectoryPage() error = %v", err)
	}
	if key, ok := entry.DataVal().AsString(); !ok || key != "middle" {
		t.Fatalf("separator key = %v, want middle", entry.DataVal())
	}
	if entry.BlockNumber() != destination.Number() {
		t.Fatalf("separator block = %d, want %d", entry.BlockNumber(), destination.Number())
	}
	if tx.ints[pageSplitLocation{block: source, offset: 4}] != 1 || tx.ints[pageSplitLocation{block: destination, offset: 4}] != 2 {
		t.Fatalf("split counts source=%d destination=%d, want 1 and 2", tx.ints[pageSplitLocation{block: source, offset: 4}], tx.ints[pageSplitLocation{block: destination, offset: 4}])
	}
	for slot, key := range []string{"middle", "zulu"} {
		if got := tx.strings[pageSplitLocation{block: destination, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != key {
			t.Errorf("destination slot %d key = %q, want %q", slot, got, key)
		}
	}
}

func TestPageSplitDirectoryPageRejectsInvalidOrNonfullPagesBeforeAppend(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 0)
	validLayout := pageInsertDirLayout(record.IntegerField)
	var typedNil *pageSplitTransaction
	validPage := &Page{transaction: newPageSplitTransaction(validLayout, 2), block: block, layout: validLayout}
	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{transaction: validPage.transaction, layout: validLayout}},
		{name: "nil transaction", page: &Page{block: block, layout: validLayout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: validLayout}},
		{name: "nil layout", page: &Page{transaction: validPage.transaction, block: block}},
		{name: "nil schema", page: &Page{transaction: validPage.transaction, block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8)}},
		{name: "invalid slot size", page: &Page{transaction: validPage.transaction, block: block, layout: record.NewLayoutFromMetadata(pageInsertDirSchema(record.IntegerField, record.IntegerField), map[string]int{"dataval": 0, "block": 4}, 0)}},
		{name: "leaf layout", page: &Page{transaction: validPage.transaction, block: block, layout: pageInsertLeafLayout(record.IntegerField)}},
		{name: "wrong directory fields", page: &Page{transaction: validPage.transaction, block: block, layout: record.NewLayout(directoryEntrySchemaWithUnexpectedField())}},
		{name: "missing data key", page: &Page{transaction: validPage.transaction, block: block, layout: record.NewLayout(directoryEntrySchemaWithoutDataval())}},
		{name: "varchar child field", page: &Page{transaction: validPage.transaction, block: block, layout: pageInsertDirLayoutWithTypes(record.IntegerField, record.VarcharField)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.SplitDirectoryPage(); err == nil {
				t.Fatal("SplitDirectoryPage() error = nil, want validation error")
			}
		})
	}

	page, tx, _, _, layout := newPageForSplit(t, record.IntegerField, 3)
	tx.blockSize = 8 + 4*layout.SlotSize()
	if _, err := page.SplitDirectoryPage(); err == nil {
		t.Fatal("SplitDirectoryPage() on a nonfull page returned no error")
	}
	if len(tx.appendCalls) != 0 || len(tx.writes) != 0 {
		t.Fatalf("nonfull split mutated storage: append=%v writes=%v", tx.appendCalls, tx.writes)
	}

	page, tx, _, _, _ = newPageForSplit(t, record.IntegerField, 1)
	if _, err := page.SplitDirectoryPage(); err == nil {
		t.Fatal("SplitDirectoryPage() with fewer than two entries returned no error")
	}
	if len(tx.appendCalls) != 0 {
		t.Fatalf("small directory split appended blocks: %v", tx.appendCalls)
	}

	page, tx, _, _, layout = newPageForSplit(t, record.IntegerField, 1)
	tx.blockSize = 8 + 2*layout.SlotSize()
	if _, err := page.SplitDirectoryPage(); err == nil {
		t.Fatal("SplitDirectoryPage() with one record and room for two returned no error")
	}
	if len(tx.appendCalls) != 0 {
		t.Fatalf("one-entry directory split appended blocks: %v", tx.appendCalls)
	}
}

func TestPageSplitDirectoryPageWrapsPreSplitReadErrors(t *testing.T) {
	for _, test := range []struct {
		name         string
		readErrorAt  int
		blockSizeErr bool
	}{
		{name: "flag read", readErrorAt: 1},
		{name: "record count read", readErrorAt: 2},
		{name: "separator key read", readErrorAt: 3},
		{name: "separator child read", readErrorAt: 4},
		{name: "block size read", blockSizeErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, tx, _, _, _ := newPageForSplit(t, record.IntegerField, 4)
			cause := errors.New(test.name + " failed")
			if test.blockSizeErr {
				tx.blockSizeErr = cause
			} else {
				tx.readErrorAt = test.readErrorAt
				tx.readError = cause
			}
			if entry, err := page.SplitDirectoryPage(); !errors.Is(err, cause) || entry != (DirEntry{}) {
				t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and wrapped %v", entry, err, cause)
			}
			if len(tx.appendCalls) != 0 {
				t.Fatalf("pre-split failure appended a block: %v", tx.appendCalls)
			}
		})
	}
}

func TestPageSplitDirectoryPageRejectsInvalidMetadataAndChildValues(t *testing.T) {
	for _, test := range []struct {
		name       string
		level      int32
		count      int32
		blockSize  int
		childBlock int32
	}{
		{name: "negative level", level: -1, count: 4, blockSize: 8 + 4*pageInsertDirLayout(record.IntegerField).SlotSize()},
		{name: "negative count", level: 0, count: -1, blockSize: 8 + 4*pageInsertDirLayout(record.IntegerField).SlotSize()},
		{name: "short header", level: 0, count: 4, blockSize: 7},
		{name: "one physical slot", level: 0, count: 1, blockSize: 8 + pageInsertDirLayout(record.IntegerField).SlotSize()},
		{name: "negative child", level: 0, count: 4, blockSize: 8 + 4*pageInsertDirLayout(record.IntegerField).SlotSize(), childBlock: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, tx, source, _, layout := newPageForSplit(t, record.IntegerField, int(test.count))
			tx.ints[pageSplitLocation{block: source, offset: 0}] = test.level
			tx.ints[pageSplitLocation{block: source, offset: 4}] = test.count
			tx.blockSize = test.blockSize
			if test.childBlock != 0 {
				tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, int(test.count/2), "block")}] = test.childBlock
			}
			if _, err := page.SplitDirectoryPage(); err == nil {
				t.Fatal("SplitDirectoryPage() error = nil, want invalid metadata/child error")
			}
			if len(tx.appendCalls) != 0 {
				t.Fatalf("invalid metadata appended a block: %v", tx.appendCalls)
			}
		})
	}
}

func directoryEntrySchemaWithoutDataval() *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("block")
	schema.AddIntegerField("child")
	return &schema
}

func TestPageSplitDirectoryPageWrapsSplitAndUnrepresentableBlockErrors(t *testing.T) {
	t.Run("append", func(t *testing.T) {
		page, tx, _, _, _ := newPageForSplit(t, record.IntegerField, 4)
		cause := errors.New("directory append failed")
		tx.appendErr = cause
		if entry, err := page.SplitDirectoryPage(); !errors.Is(err, cause) || entry != (DirEntry{}) {
			t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and wrapped %v", entry, err, cause)
		}
	})

	t.Run("destination pin", func(t *testing.T) {
		page, tx, _, _, _ := newPageForSplit(t, record.IntegerField, 4)
		cause := errors.New("destination pin failed")
		tx.pinErr = cause
		if entry, err := page.SplitDirectoryPage(); !errors.Is(err, cause) || entry != (DirEntry{}) {
			t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and wrapped %v", entry, err, cause)
		}
	})

	t.Run("transfer write", func(t *testing.T) {
		page, tx, source, _, layout := newPageForSplit(t, record.IntegerField, 4)
		for slot, row := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}, {40, 400}} {
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = row.key
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = row.child
		}
		cause := errors.New("directory transfer failed")
		tx.writeErrorAt = 11 // Ten unlogged format writes precede the first logged destination field.
		tx.writeError = cause
		if entry, err := page.SplitDirectoryPage(); !errors.Is(err, cause) || entry != (DirEntry{}) {
			t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and wrapped %v", entry, err, cause)
		}
	})

	t.Run("destination unpin", func(t *testing.T) {
		page, tx, source, _, layout := newPageForSplit(t, record.IntegerField, 4)
		for slot, row := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}, {40, 400}} {
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = row.key
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = row.child
		}
		cause := errors.New("destination unpin failed")
		tx.unpinErr = cause
		if entry, err := page.SplitDirectoryPage(); !errors.Is(err, cause) || entry != (DirEntry{}) {
			t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and wrapped %v", entry, err, cause)
		}
	})

	t.Run("block exceeds directory range", func(t *testing.T) {
		if int64(math.MaxInt32)+1 > int64(math.MaxInt) {
			t.Skip("host int cannot represent a block above int32")
		}
		page, tx, source, _, layout := newPageForSplit(t, record.IntegerField, 4)
		for slot, row := range []struct{ key, child int32 }{{10, 100}, {20, 200}, {30, 300}, {40, 400}} {
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "dataval")}] = row.key
			tx.ints[pageSplitLocation{block: source, offset: pageInsertDirPosition(layout, slot, "block")}] = row.child
		}
		tx.appendedBlock = storage.NewBlockID(source.FileName(), int(math.MaxInt32)+1)
		if entry, err := page.SplitDirectoryPage(); err == nil || entry != (DirEntry{}) {
			t.Fatalf("SplitDirectoryPage() = (%v, %v), want zero entry and range error", entry, err)
		}
	})
}
