package hash

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// HashIndex stores the state required to traverse one static hash-index bucket.
type HashIndex struct {
	transaction record.TableScanTransaction
	indexName   string
	layout      *record.Layout
	searchKey   query.Constant
	scan        *record.TableScan
}

// NewHashIndex initializes a static hash index without opening a bucket scan.
func NewHashIndex(transaction record.TableScanTransaction, indexName string, layout *record.Layout) *HashIndex {
	return &HashIndex{
		transaction: transaction,
		indexName:   indexName,
		layout:      layout,
	}
}

// Close releases the current bucket scan and clears it after successful release.
func (index *HashIndex) Close() error {
	if index.scan == nil {
		return nil
	}
	if err := index.scan.Close(); err != nil {
		return fmt.Errorf("close hash index %q: close bucket scan: %w", index.indexName, err)
	}
	index.scan = nil
	return nil
}
