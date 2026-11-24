package btree

import (
	"errors"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSetDataValIntegerWrite struct {
	block  storage.BlockID
	offset int32
	value  int32
	logged bool
}

type pageSetDataValStringWrite struct {
	block  storage.BlockID
	offset int32
	value  string
	logged bool
}

type pageSetDataValTransaction struct {
	integerError error
	stringError  error
	integerReads int
	stringReads  int
	integerWrite []pageSetDataValIntegerWrite
	stringWrite  []pageSetDataValStringWrite
	pins         int
	unpins       int
}

func (tx *pageSetDataValTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageSetDataValTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageSetDataValTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageSetDataValTransaction) GetInt(storage.BlockID, int) (int32, error) {
	tx.integerReads++
	return 0, nil
}

func (tx *pageSetDataValTransaction) GetString(storage.BlockID, int) (string, error) {
	tx.stringReads++
	return "", nil
}

func (tx *pageSetDataValTransaction) SetInt(block storage.BlockID, offset, value int32, logged bool) error {
	tx.integerWrite = append(tx.integerWrite, pageSetDataValIntegerWrite{block, offset, value, logged})
	return tx.integerError
}

func (tx *pageSetDataValTransaction) SetString(block storage.BlockID, offset int32, value string, logged bool) error {
	tx.stringWrite = append(tx.stringWrite, pageSetDataValStringWrite{block, offset, value, logged})
	return tx.stringError
}

func TestPageSetDataValWritesTypedFieldsWithLogging(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 5)
	for _, test := range []struct {
		name     string
		field    record.FieldType
		value    query.Constant
		position int32
		integer  int32
		text     string
	}{
		{name: "integer", field: record.IntegerField, value: query.NewIntConstant(-42), position: 28, integer: -42},
		{name: "varchar", field: record.VarcharField, value: query.NewStringConstant("東京"), position: 60, text: "東京"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := &pageSetDataValTransaction{}
			page := &Page{transaction: transaction, block: block, layout: pageSetDataValLayout(test.field)}
			if err := page.SetDataVal(1, test.value); err != nil {
				t.Fatalf("SetDataVal(1) error = %v", err)
			}
			if test.field == record.IntegerField {
				if len(transaction.integerWrite) != 1 || transaction.integerWrite[0] != (pageSetDataValIntegerWrite{block, test.position, test.integer, true}) {
					t.Fatalf("integer writes = %+v, want one logged write at %d", transaction.integerWrite, test.position)
				}
				if len(transaction.stringWrite) != 0 {
					t.Fatalf("string writes = %+v, want none", transaction.stringWrite)
				}
			} else {
				if len(transaction.stringWrite) != 1 || transaction.stringWrite[0] != (pageSetDataValStringWrite{block, test.position, test.text, true}) {
					t.Fatalf("string writes = %+v, want one logged write at %d", transaction.stringWrite, test.position)
				}
				if len(transaction.integerWrite) != 0 {
					t.Fatalf("integer writes = %+v, want none", transaction.integerWrite)
				}
			}
			if transaction.integerReads != 0 || transaction.stringReads != 0 || transaction.pins != 0 || transaction.unpins != 0 {
				t.Fatal("SetDataVal read page data or changed the page pin lifecycle")
			}
		})
	}
}

func TestPageSetDataValRejectsInvalidPageLayoutAndValue(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	transaction := &pageSetDataValTransaction{}
	validIntLayout := pageSetDataValLayout(record.IntegerField)
	validStringLayout := pageSetDataValLayout(record.VarcharField)
	var typedNil *pageSetDataValTransaction

	badLayouts := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 12)},
		{name: "missing dataval", layout: record.NewLayout(&record.Schema{})},
		{name: "missing dataval offset", layout: record.NewLayoutFromMetadata(pageSetDataValSchema(record.IntegerField), nil, 12)},
		{name: "unsupported dataval", layout: record.NewLayout(pageSetDataValSchema(record.FieldType(99)))},
		{name: "zero slot size", layout: record.NewLayoutFromMetadata(pageSetDataValSchema(record.IntegerField), map[string]int{"prefix": 4, "dataval": 8}, 0)},
		{name: "negative dataval offset", layout: record.NewLayoutFromMetadata(pageSetDataValSchema(record.IntegerField), map[string]int{"dataval": -1}, 12)},
		{name: "dataval offset at slot end", layout: record.NewLayoutFromMetadata(pageSetDataValSchema(record.IntegerField), map[string]int{"dataval": 12}, 12)},
	}
	for _, badLayout := range badLayouts {
		t.Run(badLayout.name, func(t *testing.T) {
			page := &Page{transaction: transaction, block: block, layout: badLayout.layout}
			if err := page.SetDataVal(0, query.NewIntConstant(1)); err == nil {
				t.Fatal("SetDataVal() error = nil, want layout validation error")
			}
		})
	}

	for _, test := range []struct {
		name  string
		page  *Page
		slot  int
		value query.Constant
	}{
		{name: "nil page", value: query.NewIntConstant(1)},
		{name: "closed page", page: &Page{block: storage.BlockID{}, layout: validIntLayout}, value: query.NewIntConstant(1)},
		{name: "nil transaction", page: &Page{block: block, layout: validIntLayout}, value: query.NewIntConstant(1)},
		{name: "typed-nil transaction", page: &Page{transaction: typedNil, block: block, layout: validIntLayout}, value: query.NewIntConstant(1)},
		{name: "nil layout", page: &Page{transaction: transaction, block: block}, value: query.NewIntConstant(1)},
		{name: "negative slot", page: &Page{transaction: transaction, block: block, layout: validIntLayout}, slot: -1, value: query.NewIntConstant(1)},
		{name: "unset integer value", page: &Page{transaction: transaction, block: block, layout: validIntLayout}},
		{name: "string value for integer field", page: &Page{transaction: transaction, block: block, layout: validIntLayout}, value: query.NewStringConstant("wrong")},
		{name: "integer value for varchar field", page: &Page{transaction: transaction, block: block, layout: validStringLayout}, value: query.NewIntConstant(1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.page.SetDataVal(test.slot, test.value); err == nil {
				t.Fatal("SetDataVal() error = nil, want page/value validation error")
			}
		})
	}

	if err := (&Page{transaction: transaction, block: block, layout: validIntLayout}).SetDataVal(math.MaxInt32/validIntLayout.SlotSize()+1, query.NewIntConstant(1)); err == nil {
		t.Fatal("SetDataVal() with offset outside int32 range returned nil error")
	}
}

func TestPageSetDataValWrapsTypedWriteFailures(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 1)
	integerError := errors.New("integer dataval write failed")
	integerTransaction := &pageSetDataValTransaction{integerError: integerError}
	integerPage := &Page{transaction: integerTransaction, block: block, layout: pageSetDataValLayout(record.IntegerField)}
	if err := integerPage.SetDataVal(0, query.NewIntConstant(3)); !errors.Is(err, integerError) {
		t.Fatalf("integer SetDataVal() error = %v, want wrapped cause", err)
	}

	stringError := errors.New("string dataval write failed")
	stringTransaction := &pageSetDataValTransaction{stringError: stringError}
	stringPage := &Page{transaction: stringTransaction, block: block, layout: pageSetDataValLayout(record.VarcharField)}
	if err := stringPage.SetDataVal(0, query.NewStringConstant("x")); !errors.Is(err, stringError) {
		t.Fatalf("string SetDataVal() error = %v, want wrapped cause", err)
	}
}

func pageSetDataValLayout(fieldType record.FieldType) *record.Layout {
	return record.NewLayout(pageSetDataValSchema(fieldType))
}

func pageSetDataValSchema(fieldType record.FieldType) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField("prefix")
	if fieldType == record.VarcharField {
		schema.AddVarcharField("dataval", 8)
	} else {
		schema.AddField("dataval", fieldType, 8)
	}
	return &schema
}
