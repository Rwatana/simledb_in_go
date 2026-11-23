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

// Next advances the current bucket scan to the next entry matching the search key.
func (index *HashIndex) Next() (bool, error) {
	if index == nil {
		return false, fmt.Errorf("advance nil hash index")
	}
	if index.scan == nil {
		return false, fmt.Errorf("advance hash index %q: bucket scan is not positioned", index.indexName)
	}
	if _, ok := index.searchKey.HashCode(); !ok {
		return false, fmt.Errorf("advance hash index %q: search key is unset", index.indexName)
	}
	if index.layout == nil {
		return false, fmt.Errorf("advance hash index %q: record layout is nil", index.indexName)
	}
	schema := index.layout.Schema()
	if schema == nil {
		return false, fmt.Errorf("advance hash index %q: record layout schema is nil", index.indexName)
	}
	fieldType, ok := schema.TypeOf("dataval")
	if !ok {
		return false, fmt.Errorf("advance hash index %q: record layout has no dataval field", index.indexName)
	}
	if fieldType != record.IntegerField && fieldType != record.VarcharField {
		return false, fmt.Errorf("advance hash index %q: dataval field has unsupported type %d", index.indexName, fieldType)
	}

	for {
		found, err := index.scan.Next()
		if err != nil {
			return false, fmt.Errorf("advance hash index %q bucket scan: %w", index.indexName, err)
		}
		if !found {
			return false, nil
		}

		var value query.Constant
		switch fieldType {
		case record.IntegerField:
			integer, err := index.scan.GetInt("dataval")
			if err != nil {
				return false, fmt.Errorf("advance hash index %q: read integer dataval: %w", index.indexName, err)
			}
			value = query.NewIntConstant(integer)
		case record.VarcharField:
			text, err := index.scan.GetString("dataval")
			if err != nil {
				return false, fmt.Errorf("advance hash index %q: read varchar dataval: %w", index.indexName, err)
			}
			value = query.NewStringConstant(text)
		}
		if value.Equal(index.searchKey) {
			return true, nil
		}
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
