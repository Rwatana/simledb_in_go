package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// ProjectPlan exposes a selected, ordered subset of its child's fields.
type ProjectPlan struct {
	child  Plan
	schema record.Schema
}

// NewProjectPlan creates a projection of child over fieldNames in the requested order.
func NewProjectPlan(child Plan, fieldNames []string) (*ProjectPlan, error) {
	if child == nil {
		return nil, fmt.Errorf("create project plan: child plan is nil")
	}
	childSchema := child.Schema()
	if childSchema == nil {
		return nil, fmt.Errorf("create project plan: child schema is nil")
	}
	project := &ProjectPlan{child: child}
	for _, fieldName := range fieldNames {
		if !project.schema.AddFrom(fieldName, childSchema) {
			return nil, fmt.Errorf("create project plan: field %q is not in child schema", fieldName)
		}
	}
	return project, nil
}

// Schema returns the schema containing only the projected fields.
func (p *ProjectPlan) Schema() *record.Schema {
	if p == nil {
		return nil
	}
	return &p.schema
}

// BlocksAccessed returns the child's estimated block accesses.
func (p *ProjectPlan) BlocksAccessed() int {
	if p == nil || p.child == nil {
		return 0
	}
	return p.child.BlocksAccessed()
}

// Open opens the child and wraps its scan with the requested projection.
func (p *ProjectPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open project plan: nil plan")
	}
	if p.child == nil {
		return nil, fmt.Errorf("open project plan: child plan is nil")
	}
	childScan, err := p.child.Open()
	if err != nil {
		return nil, fmt.Errorf("open project plan child: %w", err)
	}
	projectScan, err := query.NewProjectScan(childScan, p.schema.Fields())
	if err != nil {
		return nil, fmt.Errorf("open project plan scan: %w", err)
	}
	return projectScan, nil
}
