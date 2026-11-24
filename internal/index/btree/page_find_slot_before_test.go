package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type slotSearchTransaction struct {
	pins         int
	ints         map[int]int32
	strings      map[int]string
	intErrors    map[int]error
	stringErrors map[int]error
	intReads     []int
	stringReads  []int
}

func (tx *slotSearchTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (*slotSearchTransaction) Unpin(storage.BlockID) error { return nil }
func (*slotSearchTransaction) BlockSize() (int, error)     { return 0, nil }

func (tx *slotSearchTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	tx.intReads = append(tx.intReads, offset)
	if err := tx.intErrors[offset]; err != nil {
		return 0, err
	}
	return tx.ints[offset], nil
}

func (tx *slotSearchTransaction) GetString(_ storage.BlockID, offset int) (string, error) {
	tx.stringReads = append(tx.stringReads, offset)
	if err := tx.stringErrors[offset]; err != nil {
		return "", err
	}
	return tx.strings[offset], nil
}

func (*slotSearchTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*slotSearchTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageFindSlotBeforePositionsIntegerKeys(t *testing.T) {
	tx := &slotSearchTransaction{ints: map[int]int32{
		4:  4,
		12: 10,
		20: 20,
		28: 20,
		36: 40,
	}}
	page := newIntegerSearchPage(t, tx, 4)
	for _, test := range []struct {
		key  int32
		want int
	}{
		{key: 1, want: -1},
		{key: 10, want: -1},
		{key: 20, want: 0},
		{key: 21, want: 2},
		{key: 40, want: 2},
		{key: 50, want: 3},
	} {
		got, err := page.FindSlotBefore(query.NewIntConstant(test.key))
		if err != nil || got != test.want {
			t.Errorf("FindSlotBefore(%d) = (%d, %v), want (%d, nil)", test.key, got, err, test.want)
		}
	}
	if tx.pins != 1 {
		t.Fatalf("Pin calls = %d, want one", tx.pins)
	}
}

func TestPageFindSlotBeforePositionsStringKeysAndEmptyPages(t *testing.T) {
	tx := &slotSearchTransaction{ints: map[int]int32{4: 4}, strings: map[int]string{
		12:  "apple",
		84:  "banana",
		156: "banana",
		228: "pear",
	}}
	var schema record.Schema
	schema.AddVarcharField("dataval", 16)
	page, err := NewPage(tx, storage.NewBlockID("names.idx", 0), record.NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	for _, test := range []struct {
		key  string
		want int
	}{
		{key: "aardvark", want: -1},
		{key: "apple", want: -1},
		{key: "banana", want: 0},
		{key: "peach", want: 2},
		{key: "zebra", want: 3},
	} {
		got, err := page.FindSlotBefore(query.NewStringConstant(test.key))
		if err != nil || got != test.want {
			t.Errorf("FindSlotBefore(%q) = (%d, %v), want (%d, nil)", test.key, got, err, test.want)
		}
	}

	emptyTx := &slotSearchTransaction{ints: map[int]int32{4: 0}}
	emptyPage := newIntegerSearchPage(t, emptyTx, 0)
	got, err := emptyPage.FindSlotBefore(query.NewIntConstant(10))
	if err != nil || got != -1 {
		t.Fatalf("empty FindSlotBefore() = (%d, %v), want (-1, nil)", got, err)
	}
	if len(emptyTx.intReads) != 1 || len(emptyTx.stringReads) != 0 {
		t.Fatalf("empty page reads = ints %v strings %v, want header only", emptyTx.intReads, emptyTx.stringReads)
	}
}

func TestPageFindSlotBeforeUsesJavaStringOrdering(t *testing.T) {
	tx := &slotSearchTransaction{
		ints:    map[int]int32{4: 2},
		strings: map[int]string{12: "\U00010000", 24: "\uE000"},
	}
	var schema record.Schema
	schema.AddVarcharField("dataval", 1)
	page := &Page{
		transaction: tx,
		block:       storage.NewBlockID("unicode.idx", 0),
		layout:      record.NewLayout(&schema),
	}
	got, err := page.FindSlotBefore(query.NewStringConstant("\uE000"))
	if err != nil || got != 0 {
		t.Fatalf("FindSlotBefore(U+E000) = (%d, %v), want (0, nil) by Java UTF-16 order", got, err)
	}
}

func TestPageFindSlotBeforeRejectsInvalidState(t *testing.T) {
	transaction := &slotSearchTransaction{ints: map[int]int32{4: 0}}
	validBlock := storage.NewBlockID("values.idx", 0)
	var schema record.Schema
	schema.AddIntegerField("dataval")
	validLayout := record.NewLayout(&schema)
	var nilTransaction *slotSearchTransaction

	tests := []struct {
		name string
		page *Page
		key  query.Constant
	}{
		{name: "nil page", key: query.NewIntConstant(1)},
		{name: "nil transaction", page: &Page{block: validBlock, layout: validLayout}, key: query.NewIntConstant(1)},
		{name: "typed nil transaction", page: &Page{transaction: nilTransaction, block: validBlock, layout: validLayout}, key: query.NewIntConstant(1)},
		{name: "nil layout", page: &Page{transaction: transaction, block: validBlock}, key: query.NewIntConstant(1)},
		{name: "nil schema", page: &Page{transaction: transaction, block: validBlock, layout: record.NewLayoutFromMetadata(nil, nil, 8)}, key: query.NewIntConstant(1)},
		{name: "missing dataval", page: &Page{transaction: transaction, block: validBlock, layout: record.NewLayout(&record.Schema{})}, key: query.NewIntConstant(1)},
		{name: "missing dataval offset", page: &Page{transaction: transaction, block: validBlock, layout: record.NewLayoutFromMetadata(&schema, nil, 8)}, key: query.NewIntConstant(1)},
		{name: "invalid slot size", page: &Page{transaction: transaction, block: validBlock, layout: record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 0}, 0)}, key: query.NewIntConstant(1)},
		{name: "negative field offset", page: &Page{transaction: transaction, block: validBlock, layout: record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": -1}, 8)}, key: query.NewIntConstant(1)},
		{name: "unset key", page: &Page{transaction: transaction, block: validBlock, layout: validLayout}},
		{name: "mismatched key type", page: &Page{transaction: transaction, block: validBlock, layout: validLayout}, key: query.NewStringConstant("1")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.FindSlotBefore(test.key); err == nil {
				t.Fatal("FindSlotBefore() error = nil, want validation error")
			}
		})
	}
	var stringSchema record.Schema
	stringSchema.AddVarcharField("dataval", 8)
	stringPage := &Page{transaction: transaction, block: validBlock, layout: record.NewLayout(&stringSchema)}
	if _, err := stringPage.FindSlotBefore(query.NewIntConstant(1)); err == nil {
		t.Fatal("FindSlotBefore() with integer key for varchar dataval returned no error")
	}

	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("dataval", record.FieldType(99), 1)
	unsupported := &Page{transaction: transaction, block: validBlock, layout: record.NewLayout(&unsupportedSchema)}
	if _, err := unsupported.FindSlotBefore(query.NewIntConstant(1)); err == nil {
		t.Fatal("FindSlotBefore() with unsupported dataval type returned no error")
	}

	negativeCount := &Page{transaction: &slotSearchTransaction{ints: map[int]int32{4: -1}}, block: validBlock, layout: validLayout}
	if _, err := negativeCount.FindSlotBefore(query.NewIntConstant(1)); err == nil {
		t.Fatal("FindSlotBefore() with negative record count returned no error")
	}
}

func TestPageFindSlotBeforeWrapsRecordCountAndValueReadErrors(t *testing.T) {
	countErr := errors.New("record count read failed")
	countPage := &Page{
		transaction: &slotSearchTransaction{intErrors: map[int]error{4: countErr}},
		block:       storage.NewBlockID("values.idx", 0),
		layout:      integerSearchLayout(),
	}
	if _, err := countPage.FindSlotBefore(query.NewIntConstant(1)); !errors.Is(err, countErr) {
		t.Fatalf("record-count error = %v, want wrapped cause", err)
	}

	integerErr := errors.New("integer key read failed")
	integerPage := &Page{
		transaction: &slotSearchTransaction{
			ints:      map[int]int32{4: 1},
			intErrors: map[int]error{12: integerErr},
		},
		block:  storage.NewBlockID("values.idx", 0),
		layout: integerSearchLayout(),
	}
	if _, err := integerPage.FindSlotBefore(query.NewIntConstant(1)); !errors.Is(err, integerErr) {
		t.Fatalf("integer-key error = %v, want wrapped cause", err)
	}

	stringErr := errors.New("string key read failed")
	var stringSchema record.Schema
	stringSchema.AddVarcharField("dataval", 8)
	stringPage := &Page{
		transaction: &slotSearchTransaction{
			ints:         map[int]int32{4: 1},
			stringErrors: map[int]error{12: stringErr},
		},
		block:  storage.NewBlockID("names.idx", 0),
		layout: record.NewLayout(&stringSchema),
	}
	if _, err := stringPage.FindSlotBefore(query.NewStringConstant("a")); !errors.Is(err, stringErr) {
		t.Fatalf("string-key error = %v, want wrapped cause", err)
	}
}

func TestPageSlotFieldPositionRejectsInvalidAndOverflowingOffsets(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, test := range []struct {
		name        string
		slot        int
		slotSize    int
		fieldOffset int
	}{
		{name: "negative slot", slot: -1, slotSize: 8, fieldOffset: 4},
		{name: "nonpositive slot size", slot: 0, slotSize: 0, fieldOffset: 0},
		{name: "negative field offset", slot: 0, slotSize: 8, fieldOffset: -1},
		{name: "offset exceeds int range", slot: 0, slotSize: maxInt, fieldOffset: maxInt - 1},
		{name: "offset beyond slot", slot: 0, slotSize: 8, fieldOffset: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pageSlotFieldPosition(test.slot, test.slotSize, test.fieldOffset); err == nil {
				t.Fatal("pageSlotFieldPosition() error = nil, want invalid-position error")
			}
		})
	}

	tx := &slotSearchTransaction{ints: map[int]int32{4: 2, 12: 1}}
	var schema record.Schema
	schema.AddIntegerField("dataval")
	layout := record.NewLayoutFromMetadata(&schema, map[string]int{"dataval": 4}, maxInt)
	page := &Page{transaction: tx, block: storage.NewBlockID("values.idx", 0), layout: layout}
	if _, err := page.FindSlotBefore(query.NewIntConstant(2)); err == nil {
		t.Fatal("FindSlotBefore() with overflowing slot offset returned no error")
	}
}

func newIntegerSearchPage(t *testing.T, tx *slotSearchTransaction, count int32) *Page {
	t.Helper()
	tx.ints[4] = count
	page, err := NewPage(tx, storage.NewBlockID("values.idx", 0), integerSearchLayout())
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	return page
}

func integerSearchLayout() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	return record.NewLayout(&schema)
}
