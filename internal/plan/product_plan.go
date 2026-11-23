package plan

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// ProductPlan represents the Cartesian product of two child plans.
type ProductPlan struct {
	left   Plan
	right  Plan
	schema record.Schema
}

// NewProductPlan creates a product plan whose schema combines the child schemas from left to right.
func NewProductPlan(left, right Plan) *ProductPlan {
	product := &ProductPlan{left: left, right: right}
	if left != nil {
		if schema := left.Schema(); schema != nil {
			product.schema.AddAllFrom(schema)
		}
	}
	if right != nil {
		if schema := right.Schema(); schema != nil {
			product.schema.AddAllFrom(schema)
		}
	}
	return product
}

// Schema returns the product plan's combined output schema.
func (p *ProductPlan) Schema() *record.Schema {
	if p == nil {
		return nil
	}
	return &p.schema
}

// Open opens both child plans and returns their initialized Cartesian-product scan.
func (p *ProductPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open product plan: nil plan")
	}
	if p.left == nil {
		return nil, fmt.Errorf("open product plan: left child plan is nil")
	}
	if p.right == nil {
		return nil, fmt.Errorf("open product plan: right child plan is nil")
	}

	leftScan, err := p.left.Open()
	if err != nil {
		return nil, fmt.Errorf("open product plan left child: %w", err)
	}
	if leftScan == nil {
		return nil, fmt.Errorf("open product plan: left child scan is nil")
	}

	rightScan, err := p.right.Open()
	if err != nil {
		primary := fmt.Errorf("open product plan right child: %w", err)
		return nil, errors.Join(append([]error{primary}, closeProductPlanScans(productPlanScanSource{name: "left", scan: leftScan})...)...)
	}

	productScan, err := query.NewProductScan(leftScan, rightScan)
	if err != nil {
		primary := fmt.Errorf("open product plan scan: %w", err)
		return nil, errors.Join(append([]error{primary}, closeProductPlanScans(
			productPlanScanSource{name: "left", scan: leftScan},
			productPlanScanSource{name: "right", scan: rightScan},
		)...)...)
	}
	if err := productScan.BeforeFirst(); err != nil {
		primary := fmt.Errorf("initialize product scan: %w", err)
		return nil, errors.Join(append([]error{primary}, closeProductPlanScans(
			productPlanScanSource{name: "left", scan: leftScan},
			productPlanScanSource{name: "right", scan: rightScan},
		)...)...)
	}
	return productScan, nil
}

// BlocksAccessed estimates the nested-loop block reads of the product scan.
func (p *ProductPlan) BlocksAccessed() int {
	if p == nil || p.left == nil || p.right == nil {
		return 0
	}
	leftBlocks := int32(p.left.BlocksAccessed())
	leftRecords := int32(p.left.RecordsOutput())
	rightBlocks := int32(p.right.BlocksAccessed())
	return int(leftBlocks + leftRecords*rightBlocks)
}

// RecordsOutput estimates the number of rows in the product scan.
func (p *ProductPlan) RecordsOutput() int {
	if p == nil || p.left == nil || p.right == nil {
		return 0
	}
	return int(int32(p.left.RecordsOutput()) * int32(p.right.RecordsOutput()))
}

type productPlanScanSource struct {
	name string
	scan query.Scan
}

func closeProductPlanScans(sources ...productPlanScanSource) []error {
	var closeErrors []error
	for _, source := range sources {
		if source.scan == nil {
			continue
		}
		if err := source.scan.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close %s scan after product plan failure: %w", source.name, err))
		}
	}
	return closeErrors
}
