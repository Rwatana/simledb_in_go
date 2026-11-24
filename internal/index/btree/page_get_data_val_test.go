package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageDataValueTransaction struct {
	pins        []storage.BlockID
	unpins      []storage.BlockID
	integerData map[int]int32
	stringData  map[int]string
	integerErr  error
	stringErr   error
	intOffsets  []int
	strOffsets  []int
}

func (tx *pageDataValueTransaction) Pin(block storage.BlockID) error {
	tx.pins = append(tx.pins, block)
	return nil
}

func (tx *pageDataValueTransaction) Unpin(block storage.BlockID) error {
	tx.unpins = append(tx.unpins, block)
	return nil
}

func (*pageDataValueTransaction) BlockSize() (int, error) { return 128, nil }

func (tx *pageDataValueTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	tx.intOffsets = append(tx.intOffsets, offset)
	if tx.integerErr != nil {
		return 0, tx.integerErr
	}
	return tx.integerData[offset], nil
}

func (tx *pageDataValueTransaction) GetString(_ storage.BlockID, offset int) (string, error) {
	tx.strOffsets = append(tx.strOffsets, offset)
	if tx.stringErr != nil {
		return "", tx.stringErr
	}
	return tx.stringData[offset], nil
}

func (*pageDataValueTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*pageDataValueTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageGetDataValReadsTypedIntegerAndStringValues(t *testing.T) {
	integerLayout := pageDataValueLayout(record.IntegerField)
	integerSlot, _ := integerLayout.Offset("dataval")
	integerPosition := 8 + 2*integerLayout.SlotSize() + integerSlot
	integerTransaction := &pageDataValueTransaction{integerData: map[int]int32{integerPosition: 42}}
	integerPage := newPageForDataValue(t, integerTransaction, integerLayout)
	integerValue, err := integerPage.GetDataVal(2)
	if err != nil {
		t.Fatalf("GetDataVal(2) error = %v", err)
	}
	if value, ok := integerValue.AsInt(); !ok || value != 42 {
		t.Fatalf("integer constant = (%d, %t), want (42, true)", value, ok)
	}
	if len(integerTransaction.intOffsets) != 1 || integerTransaction.intOffsets[0] != integerPosition || len(integerTransaction.strOffsets) != 0 {
		t.Fatalf("integer reads = %v, string reads = %v, want integer read at %d", integerTransaction.intOffsets, integerTransaction.strOffsets, integerPosition)
	}

	stringLayout := pageDataValueLayout(record.VarcharField)
	stringSlot, _ := stringLayout.Offset("dataval")
	stringPosition := 8 + stringLayout.SlotSize() + stringSlot
	stringTransaction := &pageDataValueTransaction{stringData: map[int]string{stringPosition: "B-tree"}}
	stringPage := newPageForDataValue(t, stringTransaction, stringLayout)
	stringValue, err := stringPage.GetDataVal(1)
	if err != nil {
		t.Fatalf("GetDataVal(1) error = %v", err)
	}
	if value, ok := stringValue.AsString(); !ok || value != "B-tree" {
		t.Fatalf("string constant = (%q, %t), want (%q, true)", value, ok, "B-tree")
	}
	if len(stringTransaction.strOffsets) != 1 || stringTransaction.strOffsets[0] != stringPosition || len(stringTransaction.intOffsets) != 0 {
		t.Fatalf("string reads = %v, integer reads = %v, want string read at %d", stringTransaction.strOffsets, stringTransaction.intOffsets, stringPosition)
	}
	if len(integerTransaction.pins) != 1 || len(integerTransaction.unpins) != 0 || len(stringTransaction.pins) != 1 || len(stringTransaction.unpins) != 0 {
		t.Fatal("GetDataVal changed the page pin lifecycle")
	}
}

func TestPageGetDataValRejectsInvalidPageLayoutAndSlot(t *testing.T) {
	block := storage.NewBlockID("students.idx", 0)
	validTransaction := &pageDataValueTransaction{}
	validLayout := pageDataValueLayout(record.IntegerField)
	var typedNil *pageDataValueTransaction

	for _, test := range []struct {
		name string
		page *Page
		slot int
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: block, layout: validLayout}},
		{name: "typed-nil transaction", page: &Page{transaction: typedNil, block: block, layout: validLayout}},
		{name: "nil layout", page: &Page{transaction: validTransaction, block: block}},
		{name: "nil schema", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayoutFromMetadata(nil, nil, 8)}},
		{name: "missing dataval", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayout(&record.Schema{})}},
		{name: "missing dataval offset", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayoutFromMetadata(pageDataValueSchema(record.IntegerField), nil, 8)}},
		{name: "zero slot size", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayoutFromMetadata(pageDataValueSchema(record.IntegerField), map[string]int{"dataval": 0}, 0)}},
		{name: "negative slot", page: &Page{transaction: validTransaction, block: block, layout: validLayout}, slot: -1},
		{name: "negative field offset", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayoutFromMetadata(pageDataValueSchema(record.IntegerField), map[string]int{"dataval": -1}, 8)}},
		{name: "field offset at slot end", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayoutFromMetadata(pageDataValueSchema(record.IntegerField), map[string]int{"dataval": 8}, 8)}},
		{name: "unsupported field type", page: &Page{transaction: validTransaction, block: block, layout: record.NewLayout(pageDataValueSchema(record.FieldType(99)))}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.GetDataVal(test.slot); err == nil {
				t.Fatal("GetDataVal() error = nil, want validation error")
			}
		})
	}

	overflowLayout := record.NewLayoutFromMetadata(pageDataValueSchema(record.IntegerField), map[string]int{"dataval": 0}, 8)
	overflowPage := &Page{transaction: validTransaction, block: block, layout: overflowLayout}
	if _, err := overflowPage.GetDataVal(math.MaxInt); err == nil {
		t.Fatal("GetDataVal() with overflowing slot position error = nil, want error")
	}
}

func TestPageGetDataValWrapsIntegerAndStringReadErrors(t *testing.T) {
	integerError := errors.New("integer value read failed")
	integerTransaction := &pageDataValueTransaction{integerErr: integerError}
	integerPage := newPageForDataValue(t, integerTransaction, pageDataValueLayout(record.IntegerField))
	if _, err := integerPage.GetDataVal(0); !errors.Is(err, integerError) {
		t.Fatalf("integer error = %v, want wrapped cause", err)
	}

	stringError := errors.New("string value read failed")
	stringTransaction := &pageDataValueTransaction{stringErr: stringError}
	stringPage := newPageForDataValue(t, stringTransaction, pageDataValueLayout(record.VarcharField))
	if _, err := stringPage.GetDataVal(0); !errors.Is(err, stringError) {
		t.Fatalf("string error = %v, want wrapped cause", err)
	}
}

func newPageForDataValue(t *testing.T, transaction *pageDataValueTransaction, layout *record.Layout) *Page {
	t.Helper()
	page, err := NewPage(transaction, storage.NewBlockID("students.idx", 0), layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	return page
}

func pageDataValueLayout(fieldType record.FieldType) *record.Layout {
	return record.NewLayout(pageDataValueSchema(fieldType))
}

func pageDataValueSchema(fieldType record.FieldType) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("prefix")
	switch fieldType {
	case record.IntegerField:
		schema.AddIntegerField("dataval")
	case record.VarcharField:
		schema.AddVarcharField("dataval", 8)
	default:
		schema.AddField("dataval", fieldType, 1)
	}
	return &schema
}
