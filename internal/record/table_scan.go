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

// AtLastBlock reports whether the current page is the table file's final block.
func (s *TableScan) AtLastBlock() (bool, error) {
	if s.recordPage == nil {
		return false, fmt.Errorf("inspect table scan for %q: no current record page", s.fileName)
	}
	block := s.recordPage.Block()
	size, err := s.transaction.Size(s.fileName)
	if err != nil {
		return false, fmt.Errorf("inspect table scan for %q at block %d: read file size: %w", s.fileName, block.Number(), err)
	}
	if size <= 0 {
		return false, fmt.Errorf("inspect table scan for %q at block %d: file size %d must be positive", s.fileName, block.Number(), size)
	}
	return block.Number() == size-1, nil
}

// MoveToNextBlock moves the scan to the next numbered table block.
func (s *TableScan) MoveToNextBlock() error {
	if s.recordPage == nil {
		return fmt.Errorf("move table scan for %q to next block: no current record page", s.fileName)
	}
	nextBlock := storage.NewBlockID(s.fileName, s.recordPage.Block().Number()+1)
	if err := s.Close(); err != nil {
		return fmt.Errorf("move table scan for %q to block %d: close current page: %w", s.fileName, nextBlock.Number(), err)
	}
	page, err := NewRecordPage(s.transaction, nextBlock, s.layout)
	if err != nil {
		return fmt.Errorf("move table scan for %q to block %d: %w", s.fileName, nextBlock.Number(), err)
	}
	s.recordPage = page
	s.currentSlot = -1
	return nil
}

// Next advances to the next occupied record across existing table blocks.
func (s *TableScan) Next() (bool, error) {
	if s.recordPage == nil {
		return false, fmt.Errorf("iterate table scan for %q: no current record page", s.fileName)
	}
	for {
		blockNumber := s.recordPage.Block().Number()
		found, err := s.NextInBlock()
		if err != nil {
			return false, fmt.Errorf("iterate table scan for %q: search block %d: %w", s.fileName, blockNumber, err)
		}
		if found {
			return true, nil
		}
		last, err := s.AtLastBlock()
		if err != nil {
			return false, fmt.Errorf("iterate table scan for %q: continue after block %d: %w", s.fileName, blockNumber, err)
		}
		if last {
			return false, nil
		}
		if err := s.MoveToNextBlock(); err != nil {
			return false, fmt.Errorf("iterate table scan for %q: continue after block %d: %w", s.fileName, blockNumber, err)
		}
	}
}

// GetInt reads an integer field from the current record.
func (s *TableScan) GetInt(fieldName string) (int32, error) {
	if s.recordPage == nil {
		return 0, fmt.Errorf("read integer field %q from table scan for %q: no current record page", fieldName, s.fileName)
	}
	if s.currentSlot < 0 {
		return 0, fmt.Errorf("read integer field %q from table scan for %q: no current record", fieldName, s.fileName)
	}
	block := s.recordPage.Block()
	value, err := s.recordPage.GetInt(s.currentSlot, fieldName)
	if err != nil {
		return 0, fmt.Errorf("read table %q block %d slot %d field %q as integer: %w", s.fileName, block.Number(), s.currentSlot, fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the current record.
func (s *TableScan) GetString(fieldName string) (string, error) {
	if s.recordPage == nil {
		return "", fmt.Errorf("read string field %q from table scan for %q: no current record page", fieldName, s.fileName)
	}
	if s.currentSlot < 0 {
		return "", fmt.Errorf("read string field %q from table scan for %q: no current record", fieldName, s.fileName)
	}
	block := s.recordPage.Block()
	value, err := s.recordPage.GetString(s.currentSlot, fieldName)
	if err != nil {
		return "", fmt.Errorf("read table %q block %d slot %d field %q as string: %w", s.fileName, block.Number(), s.currentSlot, fieldName, err)
	}
	return value, nil
}

// HasField reports whether the table layout declares fieldName.
func (s *TableScan) HasField(fieldName string) bool {
	return s.layout.Schema().HasField(fieldName)
}

// SetInt writes an integer field in the current record.
func (s *TableScan) SetInt(fieldName string, value int32) error {
	if s.recordPage == nil {
		return fmt.Errorf("write integer field %q through table scan for %q: no current record page", fieldName, s.fileName)
	}
	if s.currentSlot < 0 {
		return fmt.Errorf("write integer field %q through table scan for %q: no current record", fieldName, s.fileName)
	}
	block := s.recordPage.Block()
	if err := s.recordPage.SetInt(s.currentSlot, fieldName, value); err != nil {
		return fmt.Errorf("write table %q block %d slot %d field %q as integer: %w", s.fileName, block.Number(), s.currentSlot, fieldName, err)
	}
	return nil
}

// SetString writes a string field in the current record.
func (s *TableScan) SetString(fieldName, value string) error {
	if s.recordPage == nil {
		return fmt.Errorf("write string field %q through table scan for %q: no current record page", fieldName, s.fileName)
	}
	if s.currentSlot < 0 {
		return fmt.Errorf("write string field %q through table scan for %q: no current record", fieldName, s.fileName)
	}
	block := s.recordPage.Block()
	if err := s.recordPage.SetString(s.currentSlot, fieldName, value); err != nil {
		return fmt.Errorf("write table %q block %d slot %d field %q as string: %w", s.fileName, block.Number(), s.currentSlot, fieldName, err)
	}
	return nil
}

// InsertInBlock occupies the next empty record slot in the current block.
func (s *TableScan) InsertInBlock() (bool, error) {
	if s.recordPage == nil {
		return false, fmt.Errorf("insert through table scan for %q: no current record page", s.fileName)
	}
	block := s.recordPage.Block()
	next, err := s.recordPage.InsertAfter(s.currentSlot)
	if err != nil {
		return false, fmt.Errorf("insert through table scan for %q in block %d after slot %d: %w", s.fileName, block.Number(), s.currentSlot, err)
	}
	s.currentSlot = next
	return next >= 0, nil
}

// MoveToNewBlock appends, pins, and formats a new table block.
func (s *TableScan) MoveToNewBlock() error {
	if s.recordPage == nil {
		return fmt.Errorf("move table scan for %q to new block: no current record page", s.fileName)
	}
	if err := s.Close(); err != nil {
		return fmt.Errorf("move table scan for %q to new block: close current page: %w", s.fileName, err)
	}
	block, err := s.transaction.Append(s.fileName)
	if err != nil {
		return fmt.Errorf("move table scan for %q to new block: append: %w", s.fileName, err)
	}
	page, err := NewRecordPage(s.transaction, block, s.layout)
	if err != nil {
		return fmt.Errorf("move table scan for %q to new block %v: %w", s.fileName, block, err)
	}
	s.recordPage = page
	if err := page.Format(); err != nil {
		return fmt.Errorf("move table scan for %q to new block %v: format: %w", s.fileName, block, err)
	}
	s.currentSlot = -1
	return nil
}

// Insert occupies the next available record slot across table blocks.
func (s *TableScan) Insert() error {
	if s.recordPage == nil {
		return fmt.Errorf("insert through table scan for %q: no current record page", s.fileName)
	}
	for {
		blockNumber := s.recordPage.Block().Number()
		inserted, err := s.InsertInBlock()
		if err != nil {
			return fmt.Errorf("insert through table scan for %q: search block %d: %w", s.fileName, blockNumber, err)
		}
		if inserted {
			return nil
		}
		last, err := s.AtLastBlock()
		if err != nil {
			return fmt.Errorf("insert through table scan for %q: continue after block %d: %w", s.fileName, blockNumber, err)
		}
		if !last {
			if err := s.MoveToNextBlock(); err != nil {
				return fmt.Errorf("insert through table scan for %q: continue after block %d: %w", s.fileName, blockNumber, err)
			}
			continue
		}
		if err := s.MoveToNewBlock(); err != nil {
			return fmt.Errorf("insert through table scan for %q: roll over after block %d: %w", s.fileName, blockNumber, err)
		}
		newBlock := s.recordPage.Block()
		inserted, err = s.InsertInBlock()
		if err != nil {
			return fmt.Errorf("insert through table scan for %q: occupy new block %d: %w", s.fileName, newBlock.Number(), err)
		}
		if !inserted {
			return fmt.Errorf("insert through table scan for %q: new block %d has no complete record slot", s.fileName, newBlock.Number())
		}
		return nil
	}
}
