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
