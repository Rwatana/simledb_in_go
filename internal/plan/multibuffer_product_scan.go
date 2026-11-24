package plan

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/multibuffer"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// MultibufferProductTransaction is the transaction surface needed to initialize a multi-buffer product scan.
type MultibufferProductTransaction interface {
	multibuffer.ChunkTransaction
	Size(fileName string) (int, error)
	AvailableBuffers() int
}

// MultibufferProductScan retains the inputs and current right-hand chunk for a multi-buffer product.
// It currently owns only the right chunk acquired during initialization; the left scan remains caller-owned.
type MultibufferProductScan struct {
	transaction    MultibufferProductTransaction
	left           query.Scan
	right          *ChunkValueScan
	fileName       string
	layout         *record.Layout
	fileSize       int
	chunkSize      int
	nextBlock      int
	product        *query.ProductScan
	newProductScan func(query.Scan, query.Scan) (*query.ProductScan, error)
}

// NewMultibufferProductScan sizes the right-hand chunks and initializes the first chunk when one exists.
// The right table name is extended with .tbl before asking the transaction for its block count.
func NewMultibufferProductScan(transaction MultibufferProductTransaction, left query.Scan, rightTable string, layout *record.Layout) (*MultibufferProductScan, error) {
	if transaction == nil {
		return nil, fmt.Errorf("initialize multibuffer product scan: transaction is nil")
	}
	if left == nil {
		return nil, fmt.Errorf("initialize multibuffer product scan: left scan is nil")
	}
	if rightTable == "" {
		return nil, fmt.Errorf("initialize multibuffer product scan: right table name is empty")
	}
	if layout == nil {
		return nil, fmt.Errorf("initialize multibuffer product scan for %q: layout is nil", rightTable)
	}
	schema := layout.Schema()
	if schema == nil {
		return nil, fmt.Errorf("initialize multibuffer product scan for %q: layout schema is nil", rightTable)
	}

	fileName := rightTable + ".tbl"
	fileSize, err := transaction.Size(fileName)
	if err != nil {
		return nil, fmt.Errorf("initialize multibuffer product scan for %q: read file size: %w", fileName, err)
	}
	if fileSize < 0 {
		return nil, fmt.Errorf("initialize multibuffer product scan for %q: file size is negative: %d", fileName, fileSize)
	}

	scan := &MultibufferProductScan{
		transaction:    transaction,
		left:           left,
		fileName:       fileName,
		layout:         layout,
		fileSize:       fileSize,
		nextBlock:      0,
		newProductScan: query.NewProductScan,
	}
	if fileSize == 0 {
		return scan, nil
	}

	available := transaction.AvailableBuffers()
	if available < 0 {
		return nil, fmt.Errorf("initialize multibuffer product scan for %q: negative buffer count: %d", fileName, available)
	}
	scan.chunkSize = multibuffer.BestFactor(available, fileSize)
	endBlock := scan.chunkSize - 1
	chunk, err := multibuffer.NewChunkScan(transaction, fileName, layout, scan.nextBlock, endBlock)
	if err != nil {
		if chunk == nil {
			return nil, fmt.Errorf("initialize multibuffer product scan for %q: pin first chunk: %w", fileName, err)
		}
		scan.right = &ChunkValueScan{ChunkScan: chunk, schema: schema}
		return scan, fmt.Errorf("initialize multibuffer product scan for %q: retain partially pinned first chunk: %w", fileName, err)
	}
	scan.right = &ChunkValueScan{ChunkScan: chunk, schema: schema}
	if err := left.BeforeFirst(); err != nil {
		initializationErr := fmt.Errorf("initialize multibuffer product scan for %q: reset left scan before first product row: %w", fileName, err)
		if closeErr := scan.Close(); closeErr != nil {
			return scan, errors.Join(initializationErr, fmt.Errorf("initialize multibuffer product scan for %q: release first chunk after left reset failure: %w", fileName, closeErr))
		}
		return nil, initializationErr
	}
	scan.nextBlock = endBlock + 1
	return scan, nil
}

// BeforeFirst resets the left and right scans within the current chunk.
func (scan *MultibufferProductScan) BeforeFirst() error {
	if scan == nil || scan.transaction == nil || scan.left == nil {
		return fmt.Errorf("rewind multibuffer product scan: scan is not initialized")
	}
	if scan.fileSize == 0 {
		return nil
	}
	if scan.right == nil {
		return fmt.Errorf("rewind multibuffer product scan for %q: current chunk is unavailable", scan.fileName)
	}
	if err := scan.ensureProduct(); err != nil {
		return err
	}
	if err := scan.product.BeforeFirst(); err != nil {
		return fmt.Errorf("rewind multibuffer product scan for %q: %w", scan.fileName, err)
	}
	return nil
}

// Next advances to the next left-major pair within the currently pinned right-hand chunk.
func (scan *MultibufferProductScan) Next() (bool, error) {
	if scan == nil || scan.transaction == nil || scan.left == nil {
		return false, fmt.Errorf("advance multibuffer product scan: scan is not initialized")
	}
	if scan.fileSize == 0 {
		return false, nil
	}
	if scan.right == nil {
		return false, fmt.Errorf("advance multibuffer product scan for %q: current chunk is unavailable", scan.fileName)
	}
	if err := scan.ensureProduct(); err != nil {
		return false, err
	}
	hasRow, err := scan.product.Next()
	if err != nil {
		return false, fmt.Errorf("advance multibuffer product scan for %q: %w", scan.fileName, err)
	}
	return hasRow, nil
}

func (scan *MultibufferProductScan) ensureProduct() error {
	if scan.product != nil {
		return nil
	}
	if scan.newProductScan == nil {
		return fmt.Errorf("initialize multibuffer product scan for %q: product scan factory is nil", scan.fileName)
	}
	product, err := scan.newProductScan(scan.left, scan.right)
	if err != nil {
		return fmt.Errorf("initialize multibuffer product scan for %q: create current-chunk product: %w", scan.fileName, err)
	}
	if product == nil {
		return fmt.Errorf("initialize multibuffer product scan for %q: product scan factory returned nil", scan.fileName)
	}
	scan.product = product
	return nil
}

// Close releases the right-hand chunk retained during initialization.
// Failed page releases remain available for a later Close retry. The caller retains ownership of the left scan.
func (scan *MultibufferProductScan) Close() error {
	if scan == nil || scan.right == nil {
		return nil
	}
	if err := scan.right.Close(); err != nil {
		return err
	}
	scan.right = nil
	scan.product = nil
	return nil
}
