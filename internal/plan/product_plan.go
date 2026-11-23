package plan

import "github.com/Rwatana/simledb_in_go/internal/record"

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
