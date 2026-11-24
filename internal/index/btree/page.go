// Package btree implements B-tree index pages.
package btree

import (
	"fmt"
	"reflect"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// PageTransaction provides the record-page operations and pin release used by a B-tree page.
type PageTransaction interface {
	record.RecordTransaction
	Unpin(storage.BlockID) error
}

// Page retains one pinned B-tree block and the layout used to interpret its records.
type Page struct {
	transaction PageTransaction
	block       storage.BlockID
	layout      *record.Layout
}

// NewPage pins block and retains its transaction and record layout.
func NewPage(transaction PageTransaction, block storage.BlockID, layout *record.Layout) (*Page, error) {
	if isNilPageTransaction(transaction) {
		return nil, fmt.Errorf("open B-tree page for block %v: transaction is nil", block)
	}
	if layout == nil {
		return nil, fmt.Errorf("open B-tree page for block %v: layout is nil", block)
	}
	if layout.Schema() == nil {
		return nil, fmt.Errorf("open B-tree page for block %v: layout schema is nil", block)
	}
	if block.FileName() == "" {
		return nil, fmt.Errorf("open B-tree page: block file name is empty")
	}
	if block.Number() < 0 {
		return nil, fmt.Errorf("open B-tree page for file %q: block number %d is negative", block.FileName(), block.Number())
	}
	if err := transaction.Pin(block); err != nil {
		return nil, fmt.Errorf("open B-tree page for block %v: pin: %w", block, err)
	}
	return &Page{transaction: transaction, block: block, layout: layout}, nil
}

func isNilPageTransaction(transaction PageTransaction) bool {
	if transaction == nil {
		return true
	}
	value := reflect.ValueOf(transaction)
	return value.Kind() == reflect.Pointer && value.IsNil()
}
