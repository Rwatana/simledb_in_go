package hash

import (
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
