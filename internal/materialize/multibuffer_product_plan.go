package materialize

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/multibuffer"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// MultibufferProductPlanTransaction is the transaction surface needed to materialize and scan a product input.
type MultibufferProductPlanTransaction interface {
	plan.MultibufferProductTransaction
	record.TableScanTransaction
}

// MultibufferProductPlan opens a left-major product over a materialized right input.
type MultibufferProductPlan struct {
	transaction MultibufferProductPlanTransaction
	left        plan.Plan
	right       plan.Plan
}

// NewMultibufferProductPlan creates a product plan using left and right query plans.
func NewMultibufferProductPlan(transaction MultibufferProductPlanTransaction, left, right plan.Plan) *MultibufferProductPlan {
	return &MultibufferProductPlan{transaction: transaction, left: left, right: right}
}

// Schema returns the output schema by combining the left fields before the right fields.
func (p *MultibufferProductPlan) Schema() *record.Schema {
	if p == nil || p.left == nil || p.right == nil {
		return nil
	}
	leftSchema := p.left.Schema()
	rightSchema := p.right.Schema()
	if leftSchema == nil || rightSchema == nil {
		return nil
	}
	schema := &record.Schema{}
	schema.AddAllFrom(leftSchema)
	schema.AddAllFrom(rightSchema)
	return schema
}

// RecordsOutput returns the estimated row count of the left-major product.
func (p *MultibufferProductPlan) RecordsOutput() int {
	if p == nil || p.left == nil || p.right == nil {
		return 0
	}
	return int(int32(p.left.RecordsOutput()) * int32(p.right.RecordsOutput()))
}

// BlocksAccessed estimates reading the product result, excluding right-plan materialization.
func (p *MultibufferProductPlan) BlocksAccessed() int {
	if p == nil || isNilSortPlanDependency(p.transaction) || isNilSortPlanDependency(p.left) || isNilSortPlanDependency(p.right) {
		return 0
	}
	rightBlocks := NewMaterializePlan(p.transaction, p.right).BlocksAccessed()
	if rightBlocks <= 0 {
		return 0
	}
	chunkSize := multibuffer.BestFactor(p.transaction.AvailableBuffers(), rightBlocks)
	chunks := rightBlocks / chunkSize
	if rightBlocks%chunkSize != 0 {
		chunks++
	}
	return int(int32(rightBlocks) + int32(p.left.BlocksAccessed())*int32(chunks))
}

// DistinctValues returns the estimate from the child whose schema contains fieldName.
func (p *MultibufferProductPlan) DistinctValues(fieldName string) int {
	if p == nil || isNilSortPlanDependency(p.transaction) || isNilSortPlanDependency(p.left) || isNilSortPlanDependency(p.right) {
		return 0
	}
	leftSchema := p.left.Schema()
	rightSchema := p.right.Schema()
	if leftSchema == nil || rightSchema == nil {
		return 0
	}
	if leftSchema.HasField(fieldName) {
		return p.left.DistinctValues(fieldName)
	}
	if rightSchema.HasField(fieldName) {
		return p.right.DistinctValues(fieldName)
	}
	return 0
}

// Open materializes the right plan and returns a multi-buffer product scan over it.
// The returned scan owns the left input scan opened by this method.
func (p *MultibufferProductPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open multibuffer product plan: plan is nil")
	}
	if p.transaction == nil {
		return nil, fmt.Errorf("open multibuffer product plan: transaction is nil")
	}
	if p.left == nil {
		return nil, fmt.Errorf("open multibuffer product plan: left plan is nil")
	}
	if p.right == nil {
		return nil, fmt.Errorf("open multibuffer product plan: right plan is nil")
	}
	if p.left.Schema() == nil {
		return nil, fmt.Errorf("open multibuffer product plan: left plan schema is nil")
	}
	if p.right.Schema() == nil {
		return nil, fmt.Errorf("open multibuffer product plan: right plan schema is nil")
	}

	leftScan, err := p.left.Open()
	if err != nil {
		return cleanupMultibufferProductOpen(fmt.Errorf("open multibuffer product left input: %w", err), nil, leftScan)
	}
	if leftScan == nil {
		return nil, fmt.Errorf("open multibuffer product plan: left plan returned a nil scan")
	}
	temporaryTable, err := p.copyRecordsFrom(p.right)
	if err != nil {
		return cleanupMultibufferProductOpen(err, nil, leftScan)
	}

	product, err := plan.NewMultibufferProductScan(p.transaction, leftScan, temporaryTable.TableName(), temporaryTable.Layout())
	if err != nil {
		return cleanupMultibufferProductOpen(fmt.Errorf("open multibuffer product scan: %w", err), product, leftScan)
	}
	return &ownedMultibufferProductScan{product: product, left: leftScan}, nil
}

func (p *MultibufferProductPlan) copyRecordsFrom(source plan.Plan) (*TempTable, error) {
	schema := source.Schema()
	if schema == nil {
		return nil, fmt.Errorf("materialize multibuffer product input: source schema is nil")
	}
	temporaryTable, err := NewTempTable(p.transaction, schema)
	if err != nil {
		return nil, fmt.Errorf("create multibuffer product temporary table: %w", err)
	}
	sourceScan, err := source.Open()
	if err != nil {
		return nil, closeMultibufferMaterialization(fmt.Errorf("open multibuffer product source: %w", err), sourceScan, nil)
	}
	if sourceScan == nil {
		return nil, fmt.Errorf("open multibuffer product source: source plan returned a nil scan")
	}
	destinationScan, err := temporaryTable.Open()
	if err != nil {
		return nil, closeMultibufferMaterialization(fmt.Errorf("open multibuffer product destination: %w", err), sourceScan, nil)
	}
	for {
		hasRow, err := sourceScan.Next()
		if err != nil {
			return nil, closeMultibufferMaterialization(fmt.Errorf("read multibuffer product source: %w", err), sourceScan, destinationScan)
		}
		if !hasRow {
			break
		}
		if err := destinationScan.Insert(); err != nil {
			return nil, closeMultibufferMaterialization(fmt.Errorf("insert multibuffer product row: %w", err), sourceScan, destinationScan)
		}
		for _, fieldName := range schema.Fields() {
			value, err := sourceScan.GetVal(fieldName)
			if err != nil {
				return nil, closeMultibufferMaterialization(fmt.Errorf("read multibuffer product field %q: %w", fieldName, err), sourceScan, destinationScan)
			}
			if err := destinationScan.SetVal(fieldName, value); err != nil {
				return nil, closeMultibufferMaterialization(fmt.Errorf("write multibuffer product field %q: %w", fieldName, err), sourceScan, destinationScan)
			}
		}
	}
	if err := sourceScan.Close(); err != nil {
		return nil, closeMultibufferMaterialization(fmt.Errorf("close multibuffer product source: %w", err), nil, destinationScan)
	}
	if err := destinationScan.Close(); err != nil {
		return nil, closeMultibufferMaterialization(fmt.Errorf("close multibuffer product destination: %w", err), nil, destinationScan)
	}
	return temporaryTable, nil
}

func closeMultibufferMaterialization(primary error, source query.Scan, destination query.UpdateScan) error {
	result := primary
	if source != nil {
		if err := source.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close multibuffer product source during cleanup: %w", err))
		}
	}
	if destination != nil {
		if err := destination.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close multibuffer product destination during cleanup: %w", err))
		}
	}
	return result
}

func cleanupMultibufferProductOpen(primary error, product query.Scan, left query.Scan) (query.Scan, error) {
	result := primary
	if product != nil {
		if err := product.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close partial multibuffer product scan: %w", err))
		} else {
			product = nil
		}
	}
	if left != nil {
		if err := left.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close multibuffer product left input: %w", err))
		} else {
			left = nil
		}
	}
	if product != nil || left != nil {
		return &ownedMultibufferProductScan{product: product, left: left}, result
	}
	return nil, result
}

type ownedMultibufferProductScan struct {
	product query.Scan
	left    query.Scan
}

func (scan *ownedMultibufferProductScan) BeforeFirst() error {
	if scan == nil || scan.product == nil {
		return fmt.Errorf("rewind owned multibuffer product scan: product scan is unavailable")
	}
	return scan.product.BeforeFirst()
}

func (scan *ownedMultibufferProductScan) Next() (bool, error) {
	if scan == nil || scan.product == nil {
		return false, fmt.Errorf("advance owned multibuffer product scan: product scan is unavailable")
	}
	return scan.product.Next()
}

func (scan *ownedMultibufferProductScan) GetInt(fieldName string) (int32, error) {
	if scan == nil || scan.product == nil {
		return 0, fmt.Errorf("read integer field %q from owned multibuffer product scan: product scan is unavailable", fieldName)
	}
	return scan.product.GetInt(fieldName)
}

func (scan *ownedMultibufferProductScan) GetString(fieldName string) (string, error) {
	if scan == nil || scan.product == nil {
		return "", fmt.Errorf("read string field %q from owned multibuffer product scan: product scan is unavailable", fieldName)
	}
	return scan.product.GetString(fieldName)
}

func (scan *ownedMultibufferProductScan) GetVal(fieldName string) (query.Constant, error) {
	if scan == nil || scan.product == nil {
		return query.Constant{}, fmt.Errorf("read value field %q from owned multibuffer product scan: product scan is unavailable", fieldName)
	}
	return scan.product.GetVal(fieldName)
}

func (scan *ownedMultibufferProductScan) HasField(fieldName string) bool {
	return scan != nil && scan.product != nil && scan.product.HasField(fieldName)
}

func (scan *ownedMultibufferProductScan) Close() error {
	if scan == nil {
		return nil
	}
	var closeErrors []error
	if scan.product != nil {
		if err := scan.product.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close owned multibuffer product: %w", err))
		} else {
			scan.product = nil
		}
	}
	if scan.left != nil {
		if err := scan.left.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close owned multibuffer product left input: %w", err))
		} else {
			scan.left = nil
		}
	}
	return errors.Join(closeErrors...)
}

var _ query.Scan = (*ownedMultibufferProductScan)(nil)
