package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
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
