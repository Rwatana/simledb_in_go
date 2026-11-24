package multibuffer

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// ChunkTransaction provides the record-page operations used to pin and release a chunk.
type ChunkTransaction interface {
	record.RecordTransaction
	Unpin(storage.BlockID) error
}

// ChunkScan owns the pinned record pages for one inclusive block range.
//
// This resource-lifecycle type does not yet implement record traversal.
type ChunkScan struct {
	transaction ChunkTransaction
	fileName    string
	layout      *record.Layout
	startBlock  int
	endBlock    int
	pages       []*record.RecordPage
	currentPage int
	currentSlot int
	ready       bool
	closed      bool
	exhausted   bool
}

// NewChunkScan pins each block in the inclusive range from startBlock to endBlock.
//
// If pinning fails and cleanup also fails, the returned scan retains the pages
// whose pins could not be released so its caller can retry Close.
func NewChunkScan(transaction ChunkTransaction, fileName string, layout *record.Layout, startBlock, endBlock int) (*ChunkScan, error) {
	if transaction == nil {
		return nil, fmt.Errorf("create chunk scan: transaction is nil")
	}
	if fileName == "" {
		return nil, fmt.Errorf("create chunk scan: filename is empty")
	}
	if layout == nil {
		return nil, fmt.Errorf("create chunk scan for %q: layout is nil", fileName)
	}
	if startBlock < 0 || endBlock < 0 {
		return nil, fmt.Errorf("create chunk scan for %q: range must be non-negative, got %d..%d", fileName, startBlock, endBlock)
	}
	if endBlock < startBlock {
		return nil, fmt.Errorf("create chunk scan for %q: end block %d precedes start block %d", fileName, endBlock, startBlock)
	}

	scan := &ChunkScan{
		transaction: transaction,
		fileName:    fileName,
		layout:      layout,
		startBlock:  startBlock,
		endBlock:    endBlock,
	}
	for blockNumber := startBlock; ; blockNumber++ {
		block := storage.NewBlockID(fileName, blockNumber)
		page, err := record.NewRecordPage(transaction, block, layout)
		if err != nil {
			pinErr := fmt.Errorf("create chunk scan for %q: pin block %d: %w", fileName, blockNumber, err)
			if closeErr := scan.Close(); closeErr != nil {
				return scan, errors.Join(pinErr, fmt.Errorf("create chunk scan for %q: release pages after pin failure: %w", fileName, closeErr))
			}
			return nil, pinErr
		}
		scan.pages = append(scan.pages, page)
		if blockNumber == endBlock {
			break
		}
	}
	scan.currentSlot = -1
	scan.ready = true
	return scan, nil
}

// BeforeFirst positions the chunk scan before the first slot of its active block.
func (scan *ChunkScan) BeforeFirst() error {
	if err := scan.validateCursor("rewind"); err != nil {
		return err
	}
	scan.currentPage = 0
	scan.currentSlot = -1
	scan.exhausted = false
	return nil
}

// Next advances to the next occupied record in the active block.
//
// It returns false at the end of that block; moving to another block is a
// separate operation.
func (scan *ChunkScan) Next() (bool, error) {
	if err := scan.validateCursor("advance"); err != nil {
		return false, err
	}
	if scan.exhausted {
		return false, nil
	}
	for {
		page := scan.pages[scan.currentPage]
		slot, err := page.NextAfter(scan.currentSlot)
		if err != nil {
			return false, fmt.Errorf("advance chunk scan for %q in block %d: %w", scan.fileName, page.Block().Number(), err)
		}
		if slot >= 0 {
			scan.currentSlot = slot
			return true, nil
		}
		if scan.currentPage+1 >= len(scan.pages) {
			scan.exhausted = true
			return false, nil
		}
		scan.currentPage++
		scan.currentSlot = -1
	}
}

// GetInt reads an integer field from the current chunk record.
func (scan *ChunkScan) GetInt(fieldName string) (int32, error) {
	operation := fmt.Sprintf("read integer field %q", fieldName)
	if err := scan.validateCurrentRecord(operation); err != nil {
		return 0, err
	}
	page := scan.pages[scan.currentPage]
	value, err := page.GetInt(scan.currentSlot, fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q from chunk scan block %d: %w", fieldName, page.Block().Number(), err)
	}
	return value, nil
}

// GetString reads a string field from the current chunk record.
func (scan *ChunkScan) GetString(fieldName string) (string, error) {
	operation := fmt.Sprintf("read string field %q", fieldName)
	if err := scan.validateCurrentRecord(operation); err != nil {
		return "", err
	}
	page := scan.pages[scan.currentPage]
	value, err := page.GetString(scan.currentSlot, fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q from chunk scan block %d: %w", fieldName, page.Block().Number(), err)
	}
	return value, nil
}

// Close releases every retained chunk-page pin and keeps failed releases for a retry.
func (scan *ChunkScan) Close() error {
	if scan == nil || scan.transaction == nil {
		return nil
	}

	scan.closed = true
	var remaining []*record.RecordPage
	var failures []error
	for _, page := range scan.pages {
		block := page.Block()
		if err := scan.transaction.Unpin(block); err != nil {
			failures = append(failures, fmt.Errorf("close chunk scan for %q: unpin block %d: %w", scan.fileName, block.Number(), err))
			remaining = append(remaining, page)
		}
	}
	scan.pages = remaining
	return errors.Join(failures...)
}

func (scan *ChunkScan) validateCursor(operation string) error {
	if scan == nil {
		return fmt.Errorf("%s chunk scan: scan is nil", operation)
	}
	if scan.closed {
		return fmt.Errorf("%s chunk scan for %q: scan is closed", operation, scan.fileName)
	}
	if !scan.ready || scan.transaction == nil {
		return fmt.Errorf("%s chunk scan for %q: scan is not initialized", operation, scan.fileName)
	}
	if scan.currentPage < 0 || scan.currentPage >= len(scan.pages) || scan.pages[scan.currentPage] == nil {
		return fmt.Errorf("%s chunk scan for %q: active page index %d is unavailable", operation, scan.fileName, scan.currentPage)
	}
	return nil
}

func (scan *ChunkScan) validateCurrentRecord(operation string) error {
	if err := scan.validateCursor(operation); err != nil {
		return err
	}
	if scan.exhausted || scan.currentSlot < 0 {
		return fmt.Errorf("%s chunk scan for %q: no current record", operation, scan.fileName)
	}
	return nil
}
