package metadata

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

const staticHashBucketCount = 100

// IndexInfo describes one index and the physical layout of its records.
type IndexInfo struct {
	indexName   string
	fieldName   string
	tableSchema *record.Schema
	transaction record.TableScanTransaction
	stats       *StatInfo
	indexLayout *record.Layout
}

// NewIndexInfo initializes index metadata for one table field.
func NewIndexInfo(indexName, fieldName string, tableSchema *record.Schema, transaction record.TableScanTransaction, stats *StatInfo) (*IndexInfo, error) {
	if tableSchema == nil {
		return nil, fmt.Errorf("initialize index %q: table schema is nil", indexName)
	}
	if transaction == nil {
		return nil, fmt.Errorf("initialize index %q: transaction is nil", indexName)
	}
	if stats == nil {
		return nil, fmt.Errorf("initialize index %q: table statistics are nil", indexName)
	}
	var indexSchema record.Schema
	indexSchema.AddIntegerField("block")
	indexSchema.AddIntegerField("id")
	definition, ok := tableSchema.Field(fieldName)
	if !ok {
		return nil, fmt.Errorf("initialize index %q: table field %q does not exist", indexName, fieldName)
	}
	indexSchema.AddField("dataval", definition.Type, definition.Length)
	return &IndexInfo{
		indexName:   indexName,
		fieldName:   fieldName,
		tableSchema: tableSchema,
		transaction: transaction,
		stats:       stats,
		indexLayout: record.NewLayout(&indexSchema),
	}, nil
}

// BlocksAccessed estimates the blocks visited while traversing one static-hash bucket.
func (i *IndexInfo) BlocksAccessed() (int, error) {
	blockSize, err := i.transaction.BlockSize()
	if err != nil {
		return 0, fmt.Errorf("estimate block accesses for index %q: read block size: %w", i.indexName, err)
	}
	recordsPerBlock := blockSize / i.indexLayout.SlotSize()
	if recordsPerBlock < 1 {
		return 0, fmt.Errorf("estimate block accesses for index %q: index record size %d does not fit block size %d", i.indexName, i.indexLayout.SlotSize(), blockSize)
	}
	indexBlocks := i.stats.RecordsOutput() / recordsPerBlock
	return indexBlocks / staticHashBucketCount, nil
}

// RecordsOutput estimates the table records matching one indexed search key.
func (i *IndexInfo) RecordsOutput() int {
	return i.stats.RecordsOutput() / i.stats.DistinctValues(i.fieldName)
}
