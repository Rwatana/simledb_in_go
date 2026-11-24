package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageMakeNewRootKeepsRootAtBlockZeroAndRaisesItsLevel(t *testing.T) {
	page, tx, root, layout := newRootGrowthPage(t, record.IntegerField, []query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(25)}, []int32{100, 200, 250}, 0, 4)
	left := storage.NewBlockID(root.FileName(), 7)
	right := storage.NewBlockID(root.FileName(), 8)
	tx.appendBlocks = []storage.BlockID{left}

	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), int(right.Number()))); err != nil {
		t.Fatalf("MakeNewRoot() error = %v", err)
	}
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 30}, []int32{7, 8})
	assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, left, layout, []int32{10, 20, 25}, []int32{100, 200, 250})
	if got := tx.ints[pageSplitLocation{block: root, offset: 0}]; got != 1 {
		t.Fatalf("root level = %d, want 1", got)
	}
	if got := tx.ints[pageSplitLocation{block: left, offset: 0}]; got != 0 {
		t.Fatalf("moved directory level = %d, want 0", got)
	}
	if len(tx.appendCalls) != 1 || tx.appendCalls[0] != root.FileName() {
		t.Fatalf("root growth appends = %v, want one append to %q", tx.appendCalls, root.FileName())
	}
}

func TestPageMakeNewRootSupportsVarcharAndRaisedExistingLevel(t *testing.T) {
	t.Run("varchar", func(t *testing.T) {
		page, tx, root, layout := newRootGrowthPage(t, record.VarcharField,
			[]query.Constant{query.NewStringConstant("alpha"), query.NewStringConstant("middle")}, []int32{100, 200}, 0, 4)
		left := storage.NewBlockID(root.FileName(), 7)
		tx.appendBlocks = []storage.BlockID{left}
		if err := page.MakeNewRoot(NewDirEntry(query.NewStringConstant("zulu"), 8)); err != nil {
			t.Fatalf("MakeNewRoot(varchar) error = %v", err)
		}
		assertRecursiveDirectoryStringRows(t, tx.pageSplitTransaction, root, layout, []string{"alpha", "zulu"}, []int32{7, 8})
		assertRecursiveDirectoryStringRows(t, tx.pageSplitTransaction, left, layout, []string{"alpha", "middle"}, []int32{100, 200})
	})

	t.Run("nonzero old level", func(t *testing.T) {
		page, tx, root, layout := newRootGrowthPage(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 2, 4)
		left := storage.NewBlockID(root.FileName(), 7)
		tx.appendBlocks = []storage.BlockID{left}
		if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 8)); err != nil {
			t.Fatalf("MakeNewRoot(level 2) error = %v", err)
		}
		if got := tx.ints[pageSplitLocation{block: root, offset: 0}]; got != 3 {
			t.Fatalf("root level = %d, want 3", got)
		}
		if got := tx.ints[pageSplitLocation{block: left, offset: 0}]; got != 2 {
			t.Fatalf("moved child level = %d, want 2", got)
		}
		assertRecursiveDirectoryRows(t, tx.pageSplitTransaction, root, layout, []int32{10, 30}, []int32{7, 8})
	})
}

func TestPageMakeNewRootRejectsInvalidPagesAndLayoutsBeforeAppending(t *testing.T) {
	entry := NewDirEntry(query.NewIntConstant(30), 8)
	if err := (*Page)(nil).MakeNewRoot(entry); err == nil {
		t.Fatal("nil page MakeNewRoot() returned no error")
	}
	if err := (&Page{}).MakeNewRoot(entry); err == nil {
		t.Fatal("closed page MakeNewRoot() returned no error")
	}
	if err := (&Page{block: storage.NewBlockID("root.idx", 0)}).MakeNewRoot(entry); err == nil {
		t.Fatal("page without transaction MakeNewRoot() returned no error")
	}

	layout := pageInsertDirLayout(record.IntegerField)
	base := newPageSplitTransaction(layout, 4)
	transaction := &recursiveDirectoryTransaction{pageSplitTransaction: base}
	for _, blockNumber := range []int{1, 2} {
		page, err := NewPage(transaction, storage.NewBlockID("root.idx", blockNumber), layout)
		if err != nil {
			t.Fatalf("NewPage(%d) error = %v", blockNumber, err)
		}
		if blockNumber == 1 {
			if err := page.MakeNewRoot(entry); err == nil {
				t.Fatal("non-root page MakeNewRoot() returned no error")
			}
			continue
		}
		page.layout = nil
		if err := page.MakeNewRoot(entry); err == nil {
			t.Fatal("page with nil layout MakeNewRoot() returned no error")
		}
	}

	for name, invalidLayout := range map[string]*record.Layout{
		"missing block field":    newDirectoryOnlyLayout(record.IntegerField),
		"noninteger block field": newDirectoryLayoutWithBlockType(record.VarcharField),
		"missing dataval field":  newDirectoryLayoutWithoutDataVal(),
		"invalid metadata":       record.NewLayoutFromMetadata(nil, nil, 8),
	} {
		t.Run(name, func(t *testing.T) {
			root := storage.NewBlockID("root-growth.idx", 0)
			tx := &recursiveDirectoryTransaction{pageSplitTransaction: newPageSplitTransaction(invalidLayout, 4)}
			page := &Page{transaction: tx, block: root, layout: invalidLayout}
			if err := page.MakeNewRoot(entry); err == nil {
				t.Fatal("MakeNewRoot() returned no error for invalid layout")
			}
			assertNoRootGrowthMutation(t, tx)
		})
	}
}

func TestPageMakeNewRootRejectsInvalidLevelsAndCountsBeforeAppending(t *testing.T) {
	tests := []struct {
		name  string
		level int32
		keys  []query.Constant
	}{
		{name: "negative level", level: -1, keys: []query.Constant{query.NewIntConstant(10)}},
		{name: "maximum level", level: math.MaxInt32, keys: []query.Constant{query.NewIntConstant(10)}},
		{name: "empty root", level: 0},
		{name: "negative record count", level: 0, keys: []query.Constant{query.NewIntConstant(10)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, tx, _, _ := newRootGrowthPage(t, record.IntegerField, test.keys, make([]int32, len(test.keys)), test.level, 4)
			if test.name == "negative record count" {
				tx.ints[pageSplitLocation{block: page.block, offset: 4}] = -1
			}
			if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 8)); err == nil {
				t.Fatal("MakeNewRoot() returned no error for invalid root metadata")
			}
			assertNoRootGrowthMutation(t, tx)
		})
	}
}

func TestPageMakeNewRootWrapsMetadataAndFirstKeyReadErrors(t *testing.T) {
	for _, test := range []struct {
		name        string
		readErrorAt int
	}{
		{name: "level", readErrorAt: 1},
		{name: "count", readErrorAt: 2},
		{name: "first key", readErrorAt: 3},
		{name: "last key", readErrorAt: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, tx, _, _ := newRootGrowthPage(t, record.IntegerField,
				[]query.Constant{query.NewIntConstant(10)}, []int32{100}, 0, 4)
			cause := errors.New("root metadata read failed")
			tx.readErrorAt = test.readErrorAt
			tx.readError = cause
			if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(20), 200)); !errors.Is(err, cause) {
				t.Fatalf("MakeNewRoot() error = %v, want wrapped %v", err, cause)
			}
			assertNoRootGrowthMutation(t, tx)
		})
	}
}

func TestPageMakeNewRootRejectsInsufficientPhysicalCapacityBeforeAppending(t *testing.T) {
	page, tx, _, _ := newRootGrowthPage(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
	tx.blockSizeErr = errors.New("block size read failed")
	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); !errors.Is(err, tx.blockSizeErr) {
		t.Fatalf("MakeNewRoot() block-size error = %v, want wrapped cause", err)
	}
	assertNoRootGrowthMutation(t, tx)

	page, tx, _, _ = newRootGrowthPage(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
	tx.blockSize = 7
	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); err == nil {
		t.Fatal("MakeNewRoot() with short page header returned no error")
	}
	assertNoRootGrowthMutation(t, tx)

	page, tx, _, _ = newRootGrowthPage(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
	tx.blockSize = 8 + page.layout.SlotSize()
	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); err == nil {
		t.Fatal("MakeNewRoot() with capacity one returned no error")
	}
	assertNoRootGrowthMutation(t, tx)

	page, tx, _, _ = newRootGrowthPage(t, record.IntegerField,
		[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20), query.NewIntConstant(30), query.NewIntConstant(40), query.NewIntConstant(50)}, []int32{1, 2, 3, 4, 5}, 0, 4)
	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(60), 6)); err == nil {
		t.Fatal("MakeNewRoot() with over-capacity count returned no error")
	}
	assertNoRootGrowthMutation(t, tx)
}

func TestPageMakeNewRootRejectsInvalidSeparatorsBeforeAppending(t *testing.T) {
	for name, separator := range map[string]DirEntry{
		"wrong type":                NewDirEntry(query.NewStringConstant("wrong"), 200),
		"before first key":          NewDirEntry(query.NewIntConstant(9), 200),
		"inside existing key range": NewDirEntry(query.NewIntConstant(15), 200),
		"negative child":            NewDirEntry(query.NewIntConstant(20), -1),
		"child out of range":        NewDirEntry(query.NewIntConstant(20), int(math.MaxInt32)+1),
	} {
		t.Run(name, func(t *testing.T) {
			page, tx, _, _ := newRootGrowthPage(t, record.IntegerField,
				[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
			if err := page.MakeNewRoot(separator); err == nil {
				t.Fatal("MakeNewRoot() returned no error for invalid separator")
			}
			assertNoRootGrowthMutation(t, tx)
		})
	}

	page, tx, _, _ := newRootGrowthPage(t, record.VarcharField,
		[]query.Constant{query.NewStringConstant("alpha"), query.NewStringConstant("middle")}, []int32{100, 200}, 0, 4)
	if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); err == nil {
		t.Fatal("MakeNewRoot() with integer separator for varchar page returned no error")
	}
	assertNoRootGrowthMutation(t, tx)
}

func TestPageMakeNewRootWrapsSplitFailures(t *testing.T) {
	t.Run("append", func(t *testing.T) {
		page, tx, _, _ := newRootGrowthPage(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
		cause := errors.New("root child append failed")
		tx.appendErrorAt = 1
		tx.appendError = cause
		if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); !errors.Is(err, cause) {
			t.Fatalf("MakeNewRoot() error = %v, want wrapped %v", err, cause)
		}
	})

	t.Run("destination pin", func(t *testing.T) {
		page, tx, root, _ := newRootGrowthPage(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
		left := storage.NewBlockID(root.FileName(), 7)
		tx.appendBlocks = []storage.BlockID{left}
		cause := errors.New("new root child pin failed")
		tx.pinErrorBlock = left
		tx.pinError = cause
		if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); !errors.Is(err, cause) {
			t.Fatalf("MakeNewRoot() error = %v, want wrapped %v", err, cause)
		}
	})

	t.Run("destination close", func(t *testing.T) {
		page, tx, root, _ := newRootGrowthPage(t, record.IntegerField,
			[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
		left := storage.NewBlockID(root.FileName(), 7)
		tx.appendBlocks = []storage.BlockID{left}
		cause := errors.New("new root child unpin failed")
		tx.unpinErrorBlock = left
		tx.unpinError = cause
		if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); !errors.Is(err, cause) {
			t.Fatalf("MakeNewRoot() error = %v, want wrapped %v", err, cause)
		}
	})
}

func TestPageMakeNewRootWrapsMovedAndSplitEntryWriteErrors(t *testing.T) {
	for _, test := range []struct {
		name             string
		writesAfterSplit int
	}{
		{name: "moved child entry", writesAfterSplit: 1},
		{name: "split child entry", writesAfterSplit: 4},
		{name: "root level", writesAfterSplit: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe, probeTx, _, _ := newRootGrowthPage(t, record.IntegerField,
				[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
			probeTx.appendBlocks = []storage.BlockID{storage.NewBlockID(probe.block.FileName(), 7)}
			if _, err := probe.Split(0, 0); err != nil {
				t.Fatalf("probe Split() error = %v", err)
			}
			splitWriteCount := len(probeTx.writes)

			page, tx, _, _ := newRootGrowthPage(t, record.IntegerField,
				[]query.Constant{query.NewIntConstant(10), query.NewIntConstant(20)}, []int32{100, 200}, 0, 4)
			cause := errors.New("root update write failed")
			tx.appendBlocks = []storage.BlockID{storage.NewBlockID(page.block.FileName(), 7)}
			tx.writeErrorAt = splitWriteCount + test.writesAfterSplit
			tx.writeError = cause
			if err := page.MakeNewRoot(NewDirEntry(query.NewIntConstant(30), 300)); !errors.Is(err, cause) {
				t.Fatalf("MakeNewRoot() error = %v, want wrapped %v", err, cause)
			}
		})
	}
}

func newRootGrowthPage(t *testing.T, keyType record.FieldType, keys []query.Constant, children []int32, level int32, capacity int) (*Page, *recursiveDirectoryTransaction, storage.BlockID, *record.Layout) {
	t.Helper()
	return newRootGrowthPageWithLayout(t, pageInsertDirLayout(keyType), keys, children, level, capacity)
}

func newRootGrowthPageWithLayout(t *testing.T, layout *record.Layout, keys []query.Constant, children []int32, level int32, capacity int) (*Page, *recursiveDirectoryTransaction, storage.BlockID, *record.Layout) {
	t.Helper()
	root := storage.NewBlockID("root-growth.idx", 0)
	base := newPageSplitTransaction(layout, capacity)
	tx := &recursiveDirectoryTransaction{pageSplitTransaction: base}
	tx.ints[pageSplitLocation{block: root, offset: 0}] = level
	tx.ints[pageSplitLocation{block: root, offset: 4}] = int32(len(keys))
	for slot, key := range keys {
		if value, ok := key.AsInt(); ok {
			tx.ints[pageSplitLocation{block: root, offset: pageInsertDirPosition(layout, slot, "dataval")}] = value
		} else if value, ok := key.AsString(); ok {
			tx.strings[pageSplitLocation{block: root, offset: pageInsertDirPosition(layout, slot, "dataval")}] = value
		}
		tx.ints[pageSplitLocation{block: root, offset: pageInsertDirPosition(layout, slot, "block")}] = children[slot]
	}
	page, err := NewPage(tx, root, layout)
	if err != nil {
		t.Fatalf("NewPage(root) error = %v", err)
	}
	return page, tx, root, layout
}

func assertRecursiveDirectoryStringRows(t *testing.T, tx *pageSplitTransaction, block storage.BlockID, layout *record.Layout, keys []string, children []int32) {
	t.Helper()
	if got := tx.ints[pageSplitLocation{block: block, offset: 4}]; got != int32(len(keys)) {
		t.Fatalf("block %v record count = %d, want %d", block, got, len(keys))
	}
	for slot, key := range keys {
		if got := tx.strings[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}]; got != key {
			t.Errorf("block %v slot %d key = %q, want %q", block, slot, got, key)
		}
		if got := tx.ints[pageSplitLocation{block: block, offset: pageInsertDirPosition(layout, slot, "block")}]; got != children[slot] {
			t.Errorf("block %v slot %d child = %d, want %d", block, slot, got, children[slot])
		}
	}
}

func assertNoRootGrowthMutation(t *testing.T, tx *recursiveDirectoryTransaction) {
	t.Helper()
	if len(tx.appendCalls) != 0 || len(tx.writes) != 0 {
		t.Fatalf("invalid root growth mutated storage: appends=%v writes=%v", tx.appendCalls, tx.writes)
	}
}

func newDirectoryOnlyLayout(keyType record.FieldType) *record.Layout {
	var schema record.Schema
	if keyType == record.IntegerField {
		schema.AddIntegerField("dataval")
	} else {
		schema.AddVarcharField("dataval", 8)
	}
	return record.NewLayout(&schema)
}

func newDirectoryLayoutWithBlockType(blockType record.FieldType) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	if blockType == record.IntegerField {
		schema.AddIntegerField("block")
	} else {
		schema.AddVarcharField("block", 8)
	}
	return record.NewLayout(&schema)
}

func newDirectoryLayoutWithoutDataVal() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("child")
	schema.AddIntegerField("block")
	return record.NewLayout(&schema)
}
