package materialize

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// SortPlan sorts a source plan into temporary runs and merges them on disk.
type SortPlan struct {
	transaction record.TableScanTransaction
	source      plan.Plan
	schema      *record.Schema
	comparator  *RecordComparator
}

var _ plan.Plan = (*SortPlan)(nil)

// NewSortPlan creates a sort plan for source using the requested field order.
func NewSortPlan(transaction record.TableScanTransaction, source plan.Plan, sortFields []string) (*SortPlan, error) {
	if isNilSortPlanDependency(transaction) {
		return nil, fmt.Errorf("create sort plan: transaction is nil")
	}
	if isNilSortPlanDependency(source) {
		return nil, fmt.Errorf("create sort plan: source plan is nil")
	}
	schema := source.Schema()
	if schema == nil {
		return nil, fmt.Errorf("create sort plan: source schema is nil")
	}
	for _, fieldName := range sortFields {
		if !schema.HasField(fieldName) {
			return nil, fmt.Errorf("create sort plan: sort field %q is not in source schema", fieldName)
		}
	}
	return &SortPlan{
		transaction: transaction,
		source:      source,
		schema:      schema,
		comparator:  NewRecordComparator(sortFields),
	}, nil
}

func isNilSortPlanDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

// Open sorts the source rows into temporary runs and returns their merged scan.
func (p *SortPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open sort plan: plan is nil")
	}
	if isNilSortPlanDependency(p.transaction) || isNilSortPlanDependency(p.source) || p.schema == nil || p.comparator == nil {
		return nil, fmt.Errorf("open sort plan: plan is incomplete")
	}
	source, err := p.source.Open()
	if err != nil {
		openErr := fmt.Errorf("open sort source: %w", err)
		if !isNilSortPlanDependency(source) {
			if closeErr := source.Close(); closeErr != nil {
				openErr = errors.Join(openErr, fmt.Errorf("close sort source after open failure: %w", closeErr))
			}
		}
		return nil, openErr
	}
	if isNilSortPlanDependency(source) {
		return nil, fmt.Errorf("open sort source: source scan is nil")
	}
	runs, splitErr := p.splitIntoRuns(source)
	if closeErr := source.Close(); closeErr != nil {
		splitErr = errors.Join(splitErr, fmt.Errorf("close sort source: %w", closeErr))
	}
	if splitErr != nil {
		return nil, splitErr
	}
	if len(runs) == 0 {
		emptyRun := newTempTable(p.transaction, p.schema)
		runs = []*TempTable{emptyRun}
	}
	for len(runs) > 2 {
		merged := make([]*TempTable, 0, (len(runs)+1)/2)
		for index := 0; index < len(runs); index += 2 {
			if index+1 == len(runs) {
				merged = append(merged, runs[index])
				continue
			}
			run, err := p.mergeRuns(runs[index], runs[index+1])
			if err != nil {
				return nil, fmt.Errorf("merge sort runs %d and %d: %w", index+1, index+2, err)
			}
			merged = append(merged, run)
		}
		runs = merged
	}
	scan, err := NewSortScan(runs, p.comparator)
	if err != nil {
		return nil, fmt.Errorf("open final sorted runs: %w", err)
	}
	return scan, nil
}

// BlocksAccessed estimates reading the sorted result without its one-time sort cost.
func (p *SortPlan) BlocksAccessed() int {
	if p == nil || isNilSortPlanDependency(p.transaction) || isNilSortPlanDependency(p.source) || p.schema == nil {
		return 0
	}
	return NewMaterializePlan(p.transaction, p.source).BlocksAccessed()
}

// RecordsOutput returns the source plan's estimated record count.
func (p *SortPlan) RecordsOutput() int {
	if p == nil || isNilSortPlanDependency(p.source) {
		return 0
	}
	return p.source.RecordsOutput()
}

// DistinctValues returns the source plan's estimate for fieldName.
func (p *SortPlan) DistinctValues(fieldName string) int {
	if p == nil || isNilSortPlanDependency(p.source) {
		return 0
	}
	return p.source.DistinctValues(fieldName)
}

// Schema returns the source plan's output schema.
func (p *SortPlan) Schema() *record.Schema {
	if p == nil {
		return nil
	}
	return p.schema
}

func (p *SortPlan) splitIntoRuns(source query.Scan) (runs []*TempTable, resultErr error) {
	if err := source.BeforeFirst(); err != nil {
		return nil, fmt.Errorf("rewind sort source: %w", err)
	}
	hasRecord, err := source.Next()
	if err != nil {
		return nil, fmt.Errorf("advance sort source: %w", err)
	}
	if !hasRecord {
		return nil, nil
	}

	var destination query.UpdateScan
	closeDestination := func() error {
		scan := destination
		destination = nil
		if err := scan.Close(); err != nil {
			return fmt.Errorf("close sorted run: %w", err)
		}
		return nil
	}
	defer func() {
		if destination != nil {
			if err := closeDestination(); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()

	for hasRecord {
		if destination == nil {
			run := newTempTable(p.transaction, p.schema)
			var err error
			destination, err = run.Open()
			if err != nil {
				return nil, fmt.Errorf("open sorted run: %w", err)
			}
			runs = append(runs, run)
		}
		if err := copySortRecord(source, destination, p.schema); err != nil {
			return nil, fmt.Errorf("copy sort source record: %w", err)
		}
		hasRecord, err = source.Next()
		if err != nil {
			return nil, fmt.Errorf("advance sort source: %w", err)
		}
		if hasRecord {
			order, err := p.comparator.Compare(source, destination)
			if err != nil {
				return nil, fmt.Errorf("compare sort source with current run: %w", err)
			}
			if order < 0 {
				if err := closeDestination(); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := closeDestination(); err != nil {
		return nil, err
	}
	return runs, nil
}

func (p *SortPlan) mergeRuns(first, second *TempTable) (*TempTable, error) {
	source, err := NewSortScan([]*TempTable{first, second}, p.comparator)
	if err != nil {
		return nil, fmt.Errorf("open merge inputs: %w", err)
	}
	destinationTable := newTempTable(p.transaction, p.schema)
	destination, err := destinationTable.Open()
	if err != nil {
		return nil, closeSortPlanScans(fmt.Errorf("open merge destination: %w", err), source, nil)
	}
	for {
		hasRecord, err := source.Next()
		if err != nil {
			return nil, closeSortPlanScans(fmt.Errorf("advance merge inputs: %w", err), source, destination)
		}
		if !hasRecord {
			break
		}
		if err := copySortRecord(source, destination, p.schema); err != nil {
			return nil, closeSortPlanScans(fmt.Errorf("copy merged record: %w", err), source, destination)
		}
	}
	if err := closeSortPlanScans(nil, source, destination); err != nil {
		return nil, err
	}
	return destinationTable, nil
}

func copySortRecord(source query.ValueScan, destination query.UpdateScan, schema *record.Schema) error {
	if err := destination.Insert(); err != nil {
		return fmt.Errorf("insert destination record: %w", err)
	}
	for _, fieldName := range schema.Fields() {
		value, err := source.GetVal(fieldName)
		if err != nil {
			return fmt.Errorf("read field %q: %w", fieldName, err)
		}
		if err := destination.SetVal(fieldName, value); err != nil {
			return fmt.Errorf("write field %q: %w", fieldName, err)
		}
	}
	return nil
}

func closeSortPlanScans(primary error, source query.Scan, destination query.UpdateScan) error {
	result := primary
	if source != nil {
		if err := source.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close sort merge input: %w", err))
		}
	}
	if destination != nil {
		if err := destination.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close sort merge destination: %w", err))
		}
	}
	return result
}
