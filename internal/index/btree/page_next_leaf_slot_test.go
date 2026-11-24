package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPageNextLeafSlotReturnsMatchingAndMismatchingCandidates(t *testing.T) {
	block := storage.NewBlockID("indexleaf", 4)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 3
	for slot, key := range []int32{7, 7, 9} {
		tx.ints[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, slot, "dataval")}] = key
	}
	page := &Page{transaction: tx, block: block, layout: layout}

	for _, test := range []struct {
		current int
		want    int
		found   bool
	}{{-1, 0, true}, {0, 1, true}, {1, 2, false}, {2, 3, false}, {7, 8, false}} {
		got, found, err := page.NextLeafSlot(query.NewIntConstant(7), test.current)
		if err != nil || got != test.want || found != test.found {
			t.Errorf("NextLeafSlot(7, %d) = (%d, %t, %v), want (%d, %t, nil)", test.current, got, found, err, test.want, test.found)
		}
	}
	if len(tx.pins) != 0 || len(tx.unpins) != 0 {
		t.Fatal("NextLeafSlot changed page pin ownership or page contents")
	}
}

func TestPageNextLeafSlotMatchesVarcharKeys(t *testing.T) {
	block := storage.NewBlockID("namesleaf", 2)
	layout := pageInsertDirLayout(record.VarcharField)
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = 2
	tx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = "apple"
	tx.strings[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 1, "dataval")}] = "banana"
	page := &Page{transaction: tx, block: block, layout: layout}

	if slot, found, err := page.NextLeafSlot(query.NewStringConstant("banana"), 0); err != nil || slot != 1 || !found {
		t.Fatalf("NextLeafSlot(banana, 0) = (%d, %t, %v), want (1, true, nil)", slot, found, err)
	}
}

func TestPageNextLeafSlotHandlesEmptyPagesAndInvalidState(t *testing.T) {
	block := storage.NewBlockID("indexleaf", 1)
	layout := pageOpenLeafLayout()
	tx := newPageFindLeafTransaction()
	page := &Page{transaction: tx, block: block, layout: layout}
	if slot, found, err := page.NextLeafSlot(query.NewIntConstant(1), -1); err != nil || slot != 0 || found {
		t.Fatalf("NextLeafSlot() on an empty page = (%d, %t, %v), want (0, false, nil)", slot, found, err)
	}

	var typedNil *pageFindLeafTransaction
	var emptySchema record.Schema
	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{transaction: tx, layout: layout}},
		{name: "nil transaction", page: &Page{block: block, layout: layout}},
		{name: "typed nil transaction", page: &Page{transaction: typedNil, block: block, layout: layout}},
		{name: "nil layout", page: &Page{transaction: tx, block: block}},
		{name: "nil schema", page: &Page{transaction: tx, block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8)}},
		{name: "missing dataval", page: &Page{transaction: tx, block: block, layout: record.NewLayout(&emptySchema)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if slot, found, err := test.page.NextLeafSlot(query.NewIntConstant(1), -1); err == nil {
				t.Fatalf("NextLeafSlot() = (%d, %t, nil), want validation error", slot, found)
			}
		})
	}
}

func TestPageNextLeafSlotRejectsMalformedDataValueLayouts(t *testing.T) {
	block := storage.NewBlockID("indexleaf", 3)
	validLayout := pageOpenLeafLayout()
	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("dataval", record.FieldType(99), 1)
	for _, test := range []struct {
		name   string
		layout *record.Layout
	}{
		{name: "unsupported dataval type", layout: record.NewLayout(&unsupportedSchema)},
		{name: "missing dataval offset", layout: record.NewLayoutFromMetadata(validLayout.Schema(), map[string]int{}, validLayout.SlotSize())},
		{name: "dataval offset outside slot", layout: record.NewLayoutFromMetadata(validLayout.Schema(), map[string]int{"dataval": validLayout.SlotSize()}, validLayout.SlotSize())},
	} {
		t.Run(test.name, func(t *testing.T) {
			page := &Page{transaction: newPageFindLeafTransaction(), block: block, layout: test.layout}
			if _, _, err := page.NextLeafSlot(query.NewIntConstant(1), -1); err == nil {
				t.Fatal("NextLeafSlot() with malformed dataval layout returned no error")
			}
		})
	}
}

func TestPageNextLeafSlotRejectsInvalidCurrentSlotsAndNegativeCounts(t *testing.T) {
	block := storage.NewBlockID("indexleaf", 6)
	layout := pageOpenLeafLayout()
	for _, currentSlot := range []int{-2, math.MaxInt} {
		tx := newPageFindLeafTransaction()
		page := &Page{transaction: tx, block: block, layout: layout}
		if _, _, err := page.NextLeafSlot(query.NewIntConstant(1), currentSlot); err == nil {
			t.Errorf("NextLeafSlot() with current slot %d returned no error", currentSlot)
		}
		if len(tx.intReads) != 0 || len(tx.pins) != 0 || len(tx.unpins) != 0 {
			t.Errorf("invalid slot %d performed transaction access or changed pin state", currentSlot)
		}
	}
	tx := newPageFindLeafTransaction()
	tx.ints[pageFindLeafOffset{block: block, offset: 4}] = -1
	page := &Page{transaction: tx, block: block, layout: layout}
	if _, _, err := page.NextLeafSlot(query.NewIntConstant(1), -1); err == nil {
		t.Fatal("NextLeafSlot() with a negative page count returned no error")
	}
}

func TestPageNextLeafSlotWrapsCountAndValueReadErrors(t *testing.T) {
	block := storage.NewBlockID("indexleaf", 8)
	layout := pageOpenLeafLayout()
	countCause := errors.New("leaf count read failed")
	countTx := newPageFindLeafTransaction()
	countTx.intErrors[pageFindLeafOffset{block: block, offset: 4}] = countCause
	countPage := &Page{transaction: countTx, block: block, layout: layout}
	if _, _, err := countPage.NextLeafSlot(query.NewIntConstant(4), -1); !errors.Is(err, countCause) {
		t.Fatalf("NextLeafSlot() count error = %v, want wrapped cause", err)
	}

	valueCause := errors.New("leaf key read failed")
	valueTx := newPageFindLeafTransaction()
	valueTx.ints[pageFindLeafOffset{block: block, offset: 4}] = 1
	valueTx.intErrors[pageFindLeafOffset{block: block, offset: pageInsertDirPosition(layout, 0, "dataval")}] = valueCause
	valuePage := &Page{transaction: valueTx, block: block, layout: layout}
	if _, _, err := valuePage.NextLeafSlot(query.NewIntConstant(4), -1); !errors.Is(err, valueCause) {
		t.Fatalf("NextLeafSlot() key error = %v, want wrapped cause", err)
	}
}
