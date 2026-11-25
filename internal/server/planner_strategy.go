package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/plan"
)

// QueryPlannerStrategy selects the query-planner implementation for a manager set.
type QueryPlannerStrategy uint8

const (
	// QueryPlannerStrategyBasic selects the basic query planner.
	QueryPlannerStrategyBasic QueryPlannerStrategy = iota
	// QueryPlannerStrategyBetter selects the cost-aware query planner.
	QueryPlannerStrategyBetter
	// QueryPlannerStrategyHeuristic selects the lowest-output join and product planner.
	QueryPlannerStrategyHeuristic
)

// SetQueryPlannerStrategy installs a query planner backed by the manager set's metadata.
func (m *Managers) SetQueryPlannerStrategy(strategy QueryPlannerStrategy) error {
	if m == nil || m.Metadata == nil {
		return fmt.Errorf("select server query planner: manager set or metadata is nil")
	}

	var selected plan.QueryPlanner
	switch strategy {
	case QueryPlannerStrategyBasic:
		selected = plan.NewBasicQueryPlanner(m.Metadata)
	case QueryPlannerStrategyBetter:
		selected = plan.NewBetterQueryPlanner(m.Metadata)
	case QueryPlannerStrategyHeuristic:
		selected = &heuristicQueryPlanner{metadata: m.Metadata}
	default:
		return fmt.Errorf("select server query planner: unknown strategy %d", strategy)
	}

	m.QueryPlanner = selected
	return nil
}
