package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageDataRIDTransaction struct {
	values     map[int]int32
	readErrors map[int]error
	reads      []int
	pins       int
	unpins     int
}

func (tx *pageDataRIDTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageDataRIDTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageDataRIDTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageDataRIDTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	tx.reads = append(tx.reads, offset)
	if err := tx.readErrors[offset]; err != nil {
		return 0, err
	}
	return tx.values[offset], nil
}

func (*pageDataRIDTransaction) GetString(storage.BlockID, int) (string, error)   { return "", nil }
func (*pageDataRIDTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*pageDataRIDTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageGetDataRIDReadsBlockAndSlotFields(t *testing.T) {
	layout := pageDataRIDLayout(record.IntegerField, record.IntegerField)
	blockFieldOffset, _ := layout.Offset("block")
	idFieldOffset, _ := layout.Offset("id")
	blockPosition := 8 + 2*layout.SlotSize() + blockFieldOffset
	idPosition := 8 + 2*layout.SlotSize() + idFieldOffset
	transaction := &pageDataRIDTransaction{values: map[int]int32{blockPosition: 19, idPosition: 6}}
	page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 4), layout: layout}

	got, err := page.GetDataRID(2)
	if err != nil || got != record.NewRID(19, 6) {
		t.Fatalf("GetDataRID(2) = (%v, %v), want (%v, nil)", got, err, record.NewRID(19, 6))
	}
	if len(transaction.reads) != 2 || transaction.reads[0] != blockPosition || transaction.reads[1] != idPosition {
		t.Fatalf("integer reads = %v, want block at %d followed by id at %d", transaction.reads, blockPosition, idPosition)
	}
	if transaction.pins != 0 || transaction.unpins != 0 {
		t.Fatal("GetDataRID changed the page pin lifecycle")
	}
}

func TestPageGetDataRIDRejectsInvalidPageLayoutAndSlot(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	transaction := &pageDataRIDTransaction{}
	validLayout := pageDataRIDLayout(record.IntegerField, record.IntegerField)
	var typedNil *pageDataRIDTransaction

	invalidLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 16)},
		{name: "missing block field", layout: record.NewLayout(pageDataRIDSchema(false, record.IntegerField, true, record.IntegerField))},
		{name: "varchar block field", layout: pageDataRIDLayout(record.VarcharField, record.IntegerField)},
		{name: "unsupported block field", layout: pageDataRIDLayout(record.FieldType(99), record.IntegerField)},
		{name: "missing block offset", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"id": 12}, 16)},
		{name: "negative block offset", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": -1, "id": 12}, 16)},
		{name: "block offset at slot end", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 16, "id": 12}, 16)},
		{name: "missing id field", layout: record.NewLayout(pageDataRIDSchema(true, record.IntegerField, false, record.IntegerField))},
		{name: "varchar id field", layout: pageDataRIDLayout(record.IntegerField, record.VarcharField)},
		{name: "unsupported id field", layout: pageDataRIDLayout(record.IntegerField, record.FieldType(99))},
		{name: "missing id offset", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8}, 16)},
		{name: "negative id offset", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": -1}, 16)},
		{name: "id offset at slot end", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": 16}, 16)},
		{name: "zero slot size", layout: record.NewLayoutFromMetadata(pageDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": 12}, 0)},
	}
	for _, invalid := range invalidLayouts {
		t.Run(invalid.name, func(t *testing.T) {
			page := &Page{transaction: transaction, block: block, layout: invalid.layout}
			if _, err := page.GetDataRID(0); err == nil {
				t.Fatal("GetDataRID() error = nil, want layout validation error")
			}
		})
	}

	for _, test := range []struct {
		name string
		page *Page
		slot int
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}, layout: validLayout}},
		{name: "nil transaction", page: &Page{block: block, layout: validLayout}},
		{name: "typed-nil transaction", page: &Page{transaction: typedNil, block: block, layout: validLayout}},
		{name: "nil layout", page: &Page{transaction: transaction, block: block}},
		{name: "negative slot", page: &Page{transaction: transaction, block: block, layout: validLayout}, slot: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.GetDataRID(test.slot); err == nil {
				t.Fatal("GetDataRID() error = nil, want page validation error")
			}
		})
	}

	overflowPage := &Page{transaction: transaction, block: block, layout: validLayout}
	if _, err := overflowPage.GetDataRID(math.MaxInt); err == nil {
		t.Fatal("GetDataRID() with overflowing slot error = nil, want error")
	}
}

func TestPageGetDataRIDWrapsBothReadFailuresWithoutPartialResult(t *testing.T) {
	layout := pageDataRIDLayout(record.IntegerField, record.IntegerField)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	blockPosition := 8 + blockOffset
	idPosition := 8 + idOffset
	blockError := errors.New("data block read failed")
	idError := errors.New("data slot read failed")

	for _, test := range []struct {
		name    string
		failure int
		cause   error
	}{
		{name: "block field", failure: blockPosition, cause: blockError},
		{name: "id field", failure: idPosition, cause: idError},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := &pageDataRIDTransaction{
				values:     map[int]int32{blockPosition: 11, idPosition: 3},
				readErrors: map[int]error{test.failure: test.cause},
			}
			page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 1), layout: layout}
			got, err := page.GetDataRID(0)
			if !errors.Is(err, test.cause) {
				t.Fatalf("GetDataRID(0) error = %v, want wrapped cause", err)
			}
			if got != (record.RID{}) {
				t.Fatalf("GetDataRID(0) = %v, want no partial RID", got)
			}
		})
	}
}

func pageDataRIDLayout(blockType, idType record.FieldType) *record.Layout {
	return record.NewLayout(pageDataRIDSchema(true, blockType, true, idType))
}

func pageDataRIDSchema(hasBlock bool, blockType record.FieldType, hasID bool, idType record.FieldType) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	if hasBlock {
		schema.AddField("block", blockType, 8)
	}
	if hasID {
		schema.AddField("id", idType, 8)
	}
	return &schema
}
