package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// SelectPlan applies a query predicate to the scan opened by a child plan.
type SelectPlan struct {
	child     Plan
	predicate query.Predicate
}

// NewSelectPlan creates a selection plan over child using predicate.
func NewSelectPlan(child Plan, predicate query.Predicate) *SelectPlan {
	return &SelectPlan{child: child, predicate: predicate}
}

// Open opens the child plan and filters its scan with the stored predicate.
func (p *SelectPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open select plan: nil plan")
	}
	if p.child == nil {
		return nil, fmt.Errorf("open select plan: child plan is nil")
	}
	childScan, err := p.child.Open()
	if err != nil {
		return nil, fmt.Errorf("open select plan child: %w", err)
	}
	scan, err := query.NewSelectScan(childScan, p.predicate)
	if err != nil {
		return nil, fmt.Errorf("open select plan scan: %w", err)
	}
	return scan, nil
}

// Schema returns the output schema exposed by the child plan.
func (p *SelectPlan) Schema() *record.Schema {
	if p == nil || p.child == nil {
		return nil
	}
	return p.child.Schema()
}

// BlocksAccessed returns the block-access estimate of the child plan.
func (p *SelectPlan) BlocksAccessed() int {
	if p == nil || p.child == nil {
		return 0
	}
	return p.child.BlocksAccessed()
}

// RecordsOutput estimates the rows that survive the predicate filter.
func (p *SelectPlan) RecordsOutput() int {
	if p == nil || p.child == nil {
		return 0
	}
	factor := p.predicate.ReductionFactor(p.child)
	if factor == 0 {
		return 0
	}
	return p.child.RecordsOutput() / factor
}

// DistinctValues estimates distinct values for fieldName after applying the predicate.
func (p *SelectPlan) DistinctValues(fieldName string) int {
	if p == nil || p.child == nil {
		return 0
	}
	if _, ok := p.predicate.EquatesWithConstant(fieldName); ok {
		return 1
	}
	if otherField, ok := p.predicate.EquatesWithField(fieldName); ok {
		fieldEstimate := p.child.DistinctValues(fieldName)
		otherEstimate := p.child.DistinctValues(otherField)
		if fieldEstimate < otherEstimate {
			return fieldEstimate
		}
		return otherEstimate
	}
	return p.child.DistinctValues(fieldName)
}
