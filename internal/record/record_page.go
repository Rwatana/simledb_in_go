package record

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// RecordTransaction provides the transaction operations used by a record page.
type RecordTransaction interface {
	Pin(storage.BlockID) error
	GetInt(storage.BlockID, int) (int32, error)
}

// RecordPage addresses fixed-size record slots within one pinned block.
type RecordPage struct {
	transaction RecordTransaction
	block       storage.BlockID
	layout      *Layout
}

// NewRecordPage pins block and returns a record page using layout.
func NewRecordPage(transaction RecordTransaction, block storage.BlockID, layout *Layout) (*RecordPage, error) {
	if err := transaction.Pin(block); err != nil {
		return nil, fmt.Errorf("initialize record page for block %v: pin: %w", block, err)
	}
	return &RecordPage{transaction: transaction, block: block, layout: layout}, nil
}

// GetInt reads an integer field from slot.
func (p *RecordPage) GetInt(slot int, fieldName string) (int32, error) {
	fieldOffset, ok := p.layout.Offset(fieldName)
	if !ok {
		return 0, fmt.Errorf("read integer from block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
	}
	position := slot*p.layout.SlotSize() + fieldOffset
	value, err := p.transaction.GetInt(p.block, position)
	if err != nil {
		return 0, fmt.Errorf("read integer from block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, position, err)
	}
	return value, nil
}
