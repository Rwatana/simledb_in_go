package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSetDataRIDWrite struct {
	block  storage.BlockID
	offset int32
	value  int32
	logged bool
}

type pageSetDataRIDTransaction struct {
	writes   []pageSetDataRIDWrite
	failAt   int
	writeErr error
	reads    int
	pins     int
	unpins   int
}

func (tx *pageSetDataRIDTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageSetDataRIDTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageSetDataRIDTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageSetDataRIDTransaction) GetInt(storage.BlockID, int) (int32, error) {
	tx.reads++
	return 0, nil
}

func (tx *pageSetDataRIDTransaction) GetString(storage.BlockID, int) (string, error) {
	tx.reads++
	return "", nil
}

func (tx *pageSetDataRIDTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	tx.writes = append(tx.writes, pageSetDataRIDWrite{block, offset, value, logged})
	if tx.failAt == len(tx.writes) {
		return tx.writeErr
	}
	return nil
}

func (*pageSetDataRIDTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageSetDataRIDWritesBothIntegerFieldsWithLogging(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 7)
	layout := pageSetDataRIDLayout(record.IntegerField, record.IntegerField)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	positions := []int32{
		int32(8 + 3*layout.SlotSize() + blockOffset),
		int32(8 + 3*layout.SlotSize() + idOffset),
	}

	for _, rid := range []record.RID{record.NewRID(-17, -4), record.NewRID(12, 9)} {
		transaction := &pageSetDataRIDTransaction{}
		page := &Page{transaction: transaction, block: block, layout: layout}
		if err := page.SetDataRID(3, rid); err != nil {
			t.Fatalf("SetDataRID(3, %v) error = %v", rid, err)
		}
		want := []pageSetDataRIDWrite{
			{block: block, offset: positions[0], value: int32(rid.BlockNumber()), logged: true},
			{block: block, offset: positions[1], value: int32(rid.Slot()), logged: true},
		}
		if len(transaction.writes) != len(want) {
			t.Fatalf("writes = %+v, want %+v", transaction.writes, want)
		}
		for i := range want {
			if transaction.writes[i] != want[i] {
				t.Fatalf("write %d = %+v, want %+v", i, transaction.writes[i], want[i])
			}
		}
		if transaction.reads != 0 || transaction.pins != 0 || transaction.unpins != 0 {
			t.Fatal("SetDataRID read page data or changed the page pin lifecycle")
		}
	}
}

func TestPageSetDataRIDRejectsInvalidPageLayoutAndIdentifier(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	transaction := &pageSetDataRIDTransaction{}
	validLayout := pageSetDataRIDLayout(record.IntegerField, record.IntegerField)
	var typedNil *pageSetDataRIDTransaction

	badLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 16)},
		{name: "missing block field", layout: record.NewLayout(pageSetDataRIDSchema(false, record.IntegerField, true, record.IntegerField))},
		{name: "varchar block field", layout: pageSetDataRIDLayout(record.VarcharField, record.IntegerField)},
		{name: "unsupported block field", layout: pageSetDataRIDLayout(record.FieldType(99), record.IntegerField)},
		{name: "missing block offset", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"id": 12}, 16)},
		{name: "negative block offset", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": -1, "id": 12}, 16)},
		{name: "block offset at slot end", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 16, "id": 12}, 16)},
		{name: "missing id field", layout: record.NewLayout(pageSetDataRIDSchema(true, record.IntegerField, false, record.IntegerField))},
		{name: "varchar id field", layout: pageSetDataRIDLayout(record.IntegerField, record.VarcharField)},
		{name: "unsupported id field", layout: pageSetDataRIDLayout(record.IntegerField, record.FieldType(99))},
		{name: "missing id offset", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8}, 16)},
		{name: "negative id offset", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": -1}, 16)},
		{name: "id offset at slot end", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": 16}, 16)},
		{name: "zero slot size", layout: record.NewLayoutFromMetadata(pageSetDataRIDSchema(true, record.IntegerField, true, record.IntegerField), map[string]int{"block": 8, "id": 12}, 0)},
	}
	for _, badLayout := range badLayouts {
		t.Run(badLayout.name, func(t *testing.T) {
			transaction := &pageSetDataRIDTransaction{}
			page := &Page{transaction: transaction, block: block, layout: badLayout.layout}
			if err := page.SetDataRID(0, record.NewRID(1, 2)); err == nil {
				t.Fatal("SetDataRID() error = nil, want layout validation error")
			}
			if len(transaction.writes) != 0 {
				t.Fatalf("invalid layout caused partial writes: %+v", transaction.writes)
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
			if err := test.page.SetDataRID(test.slot, record.NewRID(1, 2)); err == nil {
				t.Fatal("SetDataRID() error = nil, want page validation error")
			}
		})
	}

	overflowPage := &Page{transaction: transaction, block: block, layout: validLayout}
	if err := overflowPage.SetDataRID(math.MaxInt, record.NewRID(1, 2)); err == nil {
		t.Fatal("SetDataRID() with overflowing slot returned nil error")
	}
	if err := overflowPage.SetDataRID(math.MaxInt32/validLayout.SlotSize()+1, record.NewRID(1, 2)); err == nil {
		t.Fatal("SetDataRID() with offset outside int32 range returned nil error")
	}
	for _, rid := range []record.RID{
		record.NewRID(int(int64(math.MaxInt32)+1), 0),
		record.NewRID(0, int(int64(math.MinInt32)-1)),
	} {
		if err := overflowPage.SetDataRID(0, rid); err == nil {
			t.Fatalf("SetDataRID(0, %v) returned nil for an out-of-range value", rid)
		}
	}
	if len(transaction.writes) != 0 {
		t.Fatalf("invalid state caused writes: %+v", transaction.writes)
	}
}

func TestPageSetDataRIDWrapsBothWriteFailures(t *testing.T) {
	layout := pageSetDataRIDLayout(record.IntegerField, record.IntegerField)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	causes := []error{errors.New("data block write failed"), errors.New("data slot write failed")}
	for index, cause := range causes {
		t.Run([]string{"block", "id"}[index], func(t *testing.T) {
			transaction := &pageSetDataRIDTransaction{failAt: index + 1, writeErr: cause}
			page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 3), layout: layout}
			if err := page.SetDataRID(1, record.NewRID(8, 5)); !errors.Is(err, cause) {
				t.Fatalf("SetDataRID(1) error = %v, want wrapped cause", err)
			}
			wantOffsets := []int32{int32(8 + layout.SlotSize() + blockOffset), int32(8 + layout.SlotSize() + idOffset)}[:index+1]
			if len(transaction.writes) != len(wantOffsets) {
				t.Fatalf("write count = %d, want %d", len(transaction.writes), len(wantOffsets))
			}
			for i, offset := range wantOffsets {
				if transaction.writes[i].offset != offset {
					t.Fatalf("write %d offset = %d, want %d", i, transaction.writes[i].offset, offset)
				}
			}
		})
	}
}

func pageSetDataRIDLayout(blockType, idType record.FieldType) *record.Layout {
	return record.NewLayout(pageSetDataRIDSchema(true, blockType, true, idType))
}

func pageSetDataRIDSchema(hasBlock bool, blockType record.FieldType, hasID bool, idType record.FieldType) *record.Schema {
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
