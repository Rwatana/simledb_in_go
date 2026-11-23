package hash

import (
	"fmt"
	"strconv"

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

// BeforeFirst closes the previous bucket scan and opens the bucket for searchKey.
func (index *HashIndex) BeforeFirst(searchKey query.Constant) error {
	if index == nil {
		return fmt.Errorf("position nil hash index before first match")
	}
	if index.transaction == nil {
		return fmt.Errorf("position hash index %q: transaction is nil", index.indexName)
	}
	if index.layout == nil {
		return fmt.Errorf("position hash index %q: layout is nil", index.indexName)
	}
	if err := index.Close(); err != nil {
		return fmt.Errorf("position hash index %q: close previous bucket scan: %w", index.indexName, err)
	}
	index.searchKey = query.Constant{}
	bucket, err := BucketNumber(searchKey)
	if err != nil {
		return fmt.Errorf("position hash index %q: select bucket: %w", index.indexName, err)
	}
	tableName := index.indexName + strconv.Itoa(bucket)
	scan, err := record.NewTableScan(index.transaction, tableName, index.layout)
	if err != nil {
		return fmt.Errorf("position hash index %q at bucket %d table %q: %w", index.indexName, bucket, tableName, err)
	}
	index.searchKey = searchKey
	index.scan = scan
	return nil
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
