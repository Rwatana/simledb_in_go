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
	return scan, nil
}

// Close releases every retained chunk-page pin and keeps failed releases for a retry.
func (scan *ChunkScan) Close() error {
	if scan == nil || scan.transaction == nil {
		return nil
	}

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
