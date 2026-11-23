package record

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// TableScanTransaction provides transaction operations used by a table scan.
type TableScanTransaction interface {
	RecordTransaction
	Size(string) (int, error)
	Append(string) (storage.BlockID, error)
	Unpin(storage.BlockID) error
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
	if size < 0 {
		return nil, fmt.Errorf("initialize table scan for %q: table file %q has negative size %d", tableName, fileName, size)
	}
	block := storage.NewBlockID(fileName, 0)
	format := false
	if size == 0 {
		block, err = transaction.Append(fileName)
		if err != nil {
			return nil, fmt.Errorf("initialize table scan for %q: append first block to %q: %w", tableName, fileName, err)
		}
		format = true
	}
	page, err := NewRecordPage(transaction, block, layout)
	if err != nil {
		return nil, fmt.Errorf("initialize table scan for %q at first block: %w", tableName, err)
	}
	if format {
		if err := page.Format(); err != nil {
			return nil, fmt.Errorf("initialize table scan for %q: format first block %v: %w", tableName, block, err)
		}
	}
	return &TableScan{
		transaction: transaction,
		layout:      layout,
		recordPage:  page,
		fileName:    fileName,
		currentSlot: -1,
	}, nil
}

// Close releases the current record-page pin.
func (s *TableScan) Close() error {
	if s.recordPage == nil {
		return nil
	}
	block := s.recordPage.Block()
	if err := s.transaction.Unpin(block); err != nil {
		return fmt.Errorf("close table scan for %q: unpin block %v: %w", s.fileName, block, err)
	}
	s.recordPage = nil
	return nil
}

// BeforeFirst moves the scan to block zero before its first record.
func (s *TableScan) BeforeFirst() error {
	if err := s.Close(); err != nil {
		return fmt.Errorf("rewind table scan for %q: close current page: %w", s.fileName, err)
	}
	block := storage.NewBlockID(s.fileName, 0)
	page, err := NewRecordPage(s.transaction, block, s.layout)
	if err != nil {
		return fmt.Errorf("rewind table scan for %q: move to block 0: %w", s.fileName, err)
	}
	s.recordPage = page
	s.currentSlot = -1
	return nil
}

// NextInBlock advances to the next occupied record in the current block.
func (s *TableScan) NextInBlock() (bool, error) {
	if s.recordPage == nil {
		return false, fmt.Errorf("advance table scan for %q: no current record page", s.fileName)
	}
	next, err := s.recordPage.NextAfter(s.currentSlot)
	if err != nil {
		return false, fmt.Errorf("advance table scan for %q within block %v after slot %d: %w", s.fileName, s.recordPage.Block(), s.currentSlot, err)
	}
	s.currentSlot = next
	return next >= 0, nil
}
