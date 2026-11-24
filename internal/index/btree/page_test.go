package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageTransactionProbe struct {
	pins   *[]storage.BlockID
	pinErr error
}

func (p pageTransactionProbe) Pin(block storage.BlockID) error {
	*p.pins = append(*p.pins, block)
	return p.pinErr
}

func (pageTransactionProbe) Unpin(storage.BlockID) error { return nil }
func (pageTransactionProbe) BlockSize() (int, error)     { return 0, nil }
func (pageTransactionProbe) GetInt(storage.BlockID, int) (int32, error) {
	return 0, nil
}
func (pageTransactionProbe) GetString(storage.BlockID, int) (string, error) {
	return "", nil
}
func (pageTransactionProbe) SetInt(storage.BlockID, int32, int32, bool) error {
	return nil
}
func (pageTransactionProbe) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestNewPagePinsAndRetainsBlockState(t *testing.T) {
	var pins []storage.BlockID
	transaction := pageTransactionProbe{pins: &pins}
	block := storage.NewBlockID("students.idx", 4)
	var schema record.Schema
	schema.AddIntegerField("dataval")
	layout := record.NewLayout(&schema)

	page, err := NewPage(transaction, block, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if page == nil {
		t.Fatal("NewPage() returned a nil page")
	}
	if page.transaction != transaction || page.block != block || page.layout != layout {
		t.Fatalf("NewPage() state = (%v, %v, %p), want (%v, %v, %p)", page.transaction, page.block, page.layout, transaction, block, layout)
	}
	if len(pins) != 1 || pins[0] != block {
		t.Fatalf("Pin calls = %v, want [%v]", pins, block)
	}
}

func TestNewPageRejectsInvalidDependenciesBeforePinning(t *testing.T) {
	var pins []storage.BlockID
	transaction := pageTransactionProbe{pins: &pins}
	var typedNil *pageTransactionProbe
	var schema record.Schema
	schema.AddIntegerField("dataval")
	validLayout := record.NewLayout(&schema)
	missingSchemaLayout := record.NewLayoutFromMetadata(nil, nil, 8)
	validBlock := storage.NewBlockID("students.idx", 0)

	tests := []struct {
		name        string
		transaction PageTransaction
		block       storage.BlockID
		layout      *record.Layout
	}{
		{name: "nil transaction", block: validBlock, layout: validLayout},
		{name: "typed nil transaction", transaction: typedNil, block: validBlock, layout: validLayout},
		{name: "nil layout", transaction: transaction, block: validBlock},
		{name: "nil layout schema", transaction: transaction, block: validBlock, layout: missingSchemaLayout},
		{name: "empty block file", transaction: transaction, block: storage.NewBlockID("", 0), layout: validLayout},
		{name: "negative block number", transaction: transaction, block: storage.NewBlockID("students.idx", -1), layout: validLayout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, err := NewPage(test.transaction, test.block, test.layout)
			if err == nil || page != nil {
				t.Fatalf("NewPage() = (%v, %v), want nil page and validation error", page, err)
			}
			if len(pins) != 0 {
				t.Fatalf("Pin calls = %v, want none", pins)
			}
		})
	}
}

func TestNewPageWrapsPinFailure(t *testing.T) {
	pinErr := errors.New("pin failed")
	var pins []storage.BlockID
	transaction := pageTransactionProbe{pins: &pins, pinErr: pinErr}
	block := storage.NewBlockID("students.idx", 2)
	var schema record.Schema
	schema.AddIntegerField("dataval")

	page, err := NewPage(transaction, block, record.NewLayout(&schema))
	if page != nil || !errors.Is(err, pinErr) {
		t.Fatalf("NewPage() = (%v, %v), want nil page and wrapped pin error", page, err)
	}
	if len(pins) != 1 || pins[0] != block {
		t.Fatalf("Pin calls = %v, want [%v]", pins, block)
	}
}
