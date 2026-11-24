package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSetChildNumWrite struct {
	block  storage.BlockID
	offset int32
	value  int32
	logged bool
}

type pageSetChildNumTransaction struct {
	writeError error
	writes     []pageSetChildNumWrite
	reads      int
	pins       int
	unpins     int
}

func (tx *pageSetChildNumTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageSetChildNumTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageSetChildNumTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageSetChildNumTransaction) GetInt(storage.BlockID, int) (int32, error) {
	tx.reads++
	return 0, nil
}

func (*pageSetChildNumTransaction) GetString(storage.BlockID, int) (string, error) { return "", nil }

func (tx *pageSetChildNumTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	tx.writes = append(tx.writes, pageSetChildNumWrite{block, offset, value, logged})
	return tx.writeError
}

func (*pageSetChildNumTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func TestPageSetChildNumWritesSignedBlockFieldWithLogging(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 4)
	layout := pageSetChildNumLayout()
	fieldOffset, _ := layout.Offset("block")
	position := int32(8 + 2*layout.SlotSize() + fieldOffset)

	for _, value := range []int32{-7, 0, 42} {
		transaction := &pageSetChildNumTransaction{}
		page := &Page{transaction: transaction, block: block, layout: layout}
		if err := page.SetChildNum(2, value); err != nil {
			t.Fatalf("SetChildNum(2, %d) error = %v", value, err)
		}
		if len(transaction.writes) != 1 || transaction.writes[0] != (pageSetChildNumWrite{block, position, value, true}) {
			t.Fatalf("SetInt writes = %+v, want one logged write at %d of %d", transaction.writes, position, value)
		}
		if transaction.reads != 0 || transaction.pins != 0 || transaction.unpins != 0 {
			t.Fatal("SetChildNum read page data or changed page pin state")
		}
	}
}

func TestPageSetChildNumRejectsInvalidPageLayoutAndSlot(t *testing.T) {
	block := storage.NewBlockID("directory.idx", 0)
	transaction := &pageSetChildNumTransaction{}
	validLayout := pageSetChildNumLayout()
	var typedNil *pageSetChildNumTransaction

	badLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 12)},
		{name: "missing block field", layout: record.NewLayout(&record.Schema{})},
		{name: "varchar block field", layout: pageSetChildNumLayoutWithType(record.VarcharField)},
		{name: "unsupported block field", layout: pageSetChildNumLayoutWithType(record.FieldType(99))},
		{name: "missing block offset", layout: record.NewLayoutFromMetadata(pageSetChildNumSchema(), nil, 12)},
		{name: "zero slot size", layout: record.NewLayoutFromMetadata(pageSetChildNumSchema(), map[string]int{"block": 8}, 0)},
		{name: "negative block offset", layout: record.NewLayoutFromMetadata(pageSetChildNumSchema(), map[string]int{"block": -1}, 12)},
		{name: "block offset at slot end", layout: record.NewLayoutFromMetadata(pageSetChildNumSchema(), map[string]int{"block": 12}, 12)},
	}
	for _, badLayout := range badLayouts {
		t.Run(badLayout.name, func(t *testing.T) {
			page := &Page{transaction: transaction, block: block, layout: badLayout.layout}
			if err := page.SetChildNum(0, 1); err == nil {
				t.Fatal("SetChildNum() error = nil, want layout validation error")
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
			if err := test.page.SetChildNum(test.slot, 1); err == nil {
				t.Fatal("SetChildNum() error = nil, want page validation error")
			}
		})
	}

	page := &Page{transaction: transaction, block: block, layout: validLayout}
	if err := page.SetChildNum(math.MaxInt, 1); err == nil {
		t.Fatal("SetChildNum() with overflowing slot returned nil error")
	}
	if err := page.SetChildNum(math.MaxInt32/validLayout.SlotSize()+1, 1); err == nil {
		t.Fatal("SetChildNum() with offset outside int32 range returned nil error")
	}
}

func TestPageSetChildNumWrapsWriteFailure(t *testing.T) {
	writeError := errors.New("child block write failed")
	transaction := &pageSetChildNumTransaction{writeError: writeError}
	page := &Page{transaction: transaction, block: storage.NewBlockID("directory.idx", 3), layout: pageSetChildNumLayout()}
	if err := page.SetChildNum(0, 8); !errors.Is(err, writeError) {
		t.Fatalf("SetChildNum(0) error = %v, want wrapped cause", err)
	}
}

func pageSetChildNumLayout() *record.Layout {
	return pageSetChildNumLayoutWithType(record.IntegerField)
}

func pageSetChildNumLayoutWithType(fieldType record.FieldType) *record.Layout {
	return record.NewLayout(pageSetChildNumSchemaWithType(fieldType))
}

func pageSetChildNumSchema() *record.Schema {
	return pageSetChildNumSchemaWithType(record.IntegerField)
}

func pageSetChildNumSchemaWithType(fieldType record.FieldType) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("dataval")
	if fieldType == record.VarcharField {
		schema.AddVarcharField("block", 8)
	} else {
		schema.AddField("block", fieldType, 8)
	}
	return &schema
}
