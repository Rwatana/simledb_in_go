package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// BetterQueryPlanner builds left-deep product plans using block-access estimates.
type BetterQueryPlanner struct {
	basic *BasicQueryPlanner
}

// NewBetterQueryPlanner creates a cost-aware planner using the supplied metadata.
func NewBetterQueryPlanner(metadata BasicPlannerMetadata) *BetterQueryPlanner {
	return &BetterQueryPlanner{basic: NewBasicQueryPlanner(metadata)}
}

// CreatePlan places each next source on the cheaper side of the accumulated product.
func (planner *BetterQueryPlanner) CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (Plan, error) {
	if planner == nil {
		return nil, fmt.Errorf("create better query plan: planner is nil")
	}
	if planner.basic == nil {
		return nil, fmt.Errorf("create better query plan: basic planner is nil")
	}
	sources, err := planner.basic.sourcePlans(data, transaction, make(map[string]bool))
	if err != nil {
		return nil, fmt.Errorf("create better query plan: resolve sources: %w", err)
	}
	root := sources[0]
	for _, source := range sources[1:] {
		keepOrder := NewProductPlan(root, source)
		reverseOrder := NewProductPlan(source, root)
		if reverseOrder.BlocksAccessed() < keepOrder.BlocksAccessed() {
			root = reverseOrder
		} else {
			root = keepOrder
		}
	}
	return finishQueryPlan(root, data)
}
