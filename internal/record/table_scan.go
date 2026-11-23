package record

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// TableScanTransaction provides transaction operations used by a table scan.
type TableScanTransaction interface {
	RecordTransaction
	Size(string) (int, error)
}

// TableScan traverses fixed-layout records across a table file.
type TableScan struct {
	transaction TableScanTransaction
	layout      *Layout
	recordPage  *RecordPage
	fileName    string
	currentSlot int
}

// NewTableScan initializes a scan at the first block of an existing table.
func NewTableScan(transaction TableScanTransaction, tableName string, layout *Layout) (*TableScan, error) {
	fileName := tableName + ".tbl"
	size, err := transaction.Size(fileName)
	if err != nil {
		return nil, fmt.Errorf("initialize table scan for %q: read size of %q: %w", tableName, fileName, err)
	}
	if size <= 0 {
		return nil, fmt.Errorf("initialize table scan for %q: table file %q is empty", tableName, fileName)
	}
	block := storage.NewBlockID(fileName, 0)
	page, err := NewRecordPage(transaction, block, layout)
	if err != nil {
		return nil, fmt.Errorf("initialize table scan for %q at first block: %w", tableName, err)
	}
	return &TableScan{
		transaction: transaction,
		layout:      layout,
		recordPage:  page,
		fileName:    fileName,
		currentSlot: -1,
	}, nil
}
