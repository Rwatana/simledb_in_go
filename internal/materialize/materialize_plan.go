package materialize

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// MaterializePlan copies the output of a source plan into a temporary table.
type MaterializePlan struct {
	transaction record.TableScanTransaction
	source      plan.Plan
}

// NewMaterializePlan creates a plan that materializes source with transaction.
func NewMaterializePlan(transaction record.TableScanTransaction, source plan.Plan) *MaterializePlan {
	return &MaterializePlan{transaction: transaction, source: source}
}

// Open copies the source rows into a temporary table and returns its scan before the first row.
func (p *MaterializePlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open materialize plan: plan is nil")
	}
	if p.source == nil {
		return nil, fmt.Errorf("open materialize plan: source plan is nil")
	}
	schema := p.source.Schema()
	temporaryTable, err := NewTempTable(p.transaction, schema)
	if err != nil {
		return nil, fmt.Errorf("create materialize destination: %w", err)
	}
	sourceScan, err := p.source.Open()
	if err != nil {
		return nil, fmt.Errorf("open materialize source: %w", err)
	}
	if sourceScan == nil {
		return nil, fmt.Errorf("open materialize source: source scan is nil")
	}
	destinationScan, err := temporaryTable.Open()
	if err != nil {
		return nil, closeMaterializeScans(fmt.Errorf("open materialize destination: %w", err), sourceScan, nil)
	}
	for {
		hasRow, err := sourceScan.Next()
		if err != nil {
			return nil, closeMaterializeScans(fmt.Errorf("read materialize source: %w", err), sourceScan, destinationScan)
		}
		if !hasRow {
			break
		}
		if err := destinationScan.Insert(); err != nil {
			return nil, closeMaterializeScans(fmt.Errorf("insert materialized row: %w", err), sourceScan, destinationScan)
		}
		for _, fieldName := range schema.Fields() {
			value, err := sourceScan.GetVal(fieldName)
			if err != nil {
				return nil, closeMaterializeScans(fmt.Errorf("read materialize source field %q: %w", fieldName, err), sourceScan, destinationScan)
			}
			if err := destinationScan.SetVal(fieldName, value); err != nil {
				return nil, closeMaterializeScans(fmt.Errorf("write materialized field %q: %w", fieldName, err), sourceScan, destinationScan)
			}
		}
	}
	if err := sourceScan.Close(); err != nil {
		return nil, closeMaterializeScans(fmt.Errorf("close materialize source: %w", err), nil, destinationScan)
	}
	if err := destinationScan.BeforeFirst(); err != nil {
		return nil, closeMaterializeScans(fmt.Errorf("rewind materialized scan: %w", err), nil, destinationScan)
	}
	return destinationScan, nil
}

// Schema returns the source plan's output schema.
func (p *MaterializePlan) Schema() *record.Schema {
	if p == nil || p.source == nil {
		return nil
	}
	return p.source.Schema()
}

// BlocksAccessed estimates reading the completed temporary table, excluding materialization cost.
func (p *MaterializePlan) BlocksAccessed() int {
	if p == nil || p.transaction == nil || p.source == nil {
		return 0
	}
	records := p.source.RecordsOutput()
	if records <= 0 {
		return 0
	}
	schema := p.source.Schema()
	if schema == nil {
		return 0
	}
	layout := record.NewLayout(schema)
	blockSize, err := p.transaction.BlockSize()
	if err != nil || blockSize <= 0 {
		return 0
	}
	recordsPerBlock := blockSize / layout.SlotSize()
	if recordsPerBlock <= 0 {
		return 0
	}
	blocks := records / recordsPerBlock
	if records%recordsPerBlock != 0 {
		blocks++
	}
	return blocks
}

// RecordsOutput returns the source plan's estimated output row count.
func (p *MaterializePlan) RecordsOutput() int {
	if p == nil || p.source == nil {
		return 0
	}
	return p.source.RecordsOutput()
}

// DistinctValues returns the source plan's estimate for fieldName.
func (p *MaterializePlan) DistinctValues(fieldName string) int {
	if p == nil || p.source == nil {
		return 0
	}
	return p.source.DistinctValues(fieldName)
}

func closeMaterializeScans(primary error, source query.Scan, destination query.UpdateScan) error {
	result := primary
	if source != nil {
		if err := source.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close materialize source during cleanup: %w", err))
		}
	}
	if destination != nil {
		if err := destination.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close materialize destination during cleanup: %w", err))
		}
	}
	return result
}

var _ plan.Plan = (*MaterializePlan)(nil)
