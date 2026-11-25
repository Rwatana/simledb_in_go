package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// HeuristicQueryPlanner creates a left-deep query plan from table metadata and output estimates.
type HeuristicQueryPlanner struct {
	metadata       TablePlannerMetadata
	productFactory ProductPlanFactory
}

// NewHeuristicQueryPlanner creates a heuristic planner, defaulting to ordinary product plans when factory is nil.
func NewHeuristicQueryPlanner(metadata TablePlannerMetadata, factory ProductPlanFactory) *HeuristicQueryPlanner {
	if factory == nil {
		factory = func(left, right Plan) Plan { return NewProductPlan(left, right) }
	}
	return &HeuristicQueryPlanner{metadata: metadata, productFactory: factory}
}

// CreatePlan builds the lowest-output left-deep join order, uses product plans for disconnected sources, and projects fields.
func (planner *HeuristicQueryPlanner) CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (Plan, error) {
	if planner == nil {
		return nil, fmt.Errorf("create heuristic query plan: planner is nil")
	}
	if isNilTablePlannerDependency(planner.metadata) {
		return nil, fmt.Errorf("create heuristic query plan: metadata is nil")
	}
	if isNilTablePlannerDependency(transaction) {
		return nil, fmt.Errorf("create heuristic query plan: transaction is nil")
	}

	tableNames := data.Tables()
	if len(tableNames) == 0 {
		return nil, fmt.Errorf("create heuristic query plan: query has no tables")
	}
	planners := make([]*TablePlanner, 0, len(tableNames))
	for _, tableName := range tableNames {
		tablePlanner, err := NewTablePlanner(tableName, data.Predicate(), transaction, planner.metadata)
		if err != nil {
			return nil, fmt.Errorf("create heuristic query plan: initialize table %q: %w", tableName, err)
		}
		planners = append(planners, tablePlanner)
	}

	current, remaining := chooseLowestSelectPlanFromValid(planners)
	for len(remaining) > 0 {
		next, nextRemaining := chooseLowestJoinPlanFromValid(remaining, current)
		if next == nil {
			product, productRemaining, err := chooseLowestProductPlan(remaining, current, planner.productFactory)
			if err != nil {
				return nil, fmt.Errorf("create heuristic query plan: choose product fallback: %w", err)
			}
			next, nextRemaining = product, productRemaining
		}
		current = next
		remaining = nextRemaining
	}

	projected, err := NewProjectPlan(current, data.Fields())
	if err != nil {
		return nil, fmt.Errorf("create heuristic query plan projection: %w", err)
	}
	return projected, nil
}

func chooseLowestSelectPlan(planners []*TablePlanner) (Plan, []*TablePlanner, error) {
	if len(planners) == 0 {
		return nil, nil, fmt.Errorf("choose initial query plan: no table planners")
	}
	for index, planner := range planners {
		if planner == nil {
			return nil, nil, fmt.Errorf("choose initial query plan: table planner at index %d is nil", index)
		}
		if planner.tablePlan == nil {
			return nil, nil, fmt.Errorf("choose initial query plan: table plan for %q is nil", planner.tableName)
		}
	}
	bestPlan, remaining := chooseLowestSelectPlanFromValid(planners)
	return bestPlan, remaining, nil
}

func chooseLowestSelectPlanFromValid(planners []*TablePlanner) (Plan, []*TablePlanner) {
	bestIndex := 0
	bestPlan := planners[0].MakeSelectPlan()
	bestRecords := bestPlan.RecordsOutput()
	for index := 1; index < len(planners); index++ {
		candidate := planners[index].MakeSelectPlan()
		candidateRecords := candidate.RecordsOutput()
		if candidateRecords < bestRecords {
			bestIndex = index
			bestPlan = candidate
			bestRecords = candidateRecords
		}
	}
	remaining := make([]*TablePlanner, 0, len(planners)-1)
	remaining = append(remaining, planners[:bestIndex]...)
	remaining = append(remaining, planners[bestIndex+1:]...)
	return bestPlan, remaining
}

func chooseLowestJoinPlan(planners []*TablePlanner, current Plan) (Plan, []*TablePlanner, error) {
	if len(planners) == 0 {
		return nil, nil, fmt.Errorf("choose next query join: no table planners")
	}
	if isNilTablePlannerDependency(current) {
		return nil, nil, fmt.Errorf("choose next query join: current plan is nil")
	}

	for index, planner := range planners {
		if planner == nil {
			return nil, nil, fmt.Errorf("choose next query join: table planner at index %d is nil", index)
		}
		if planner.tablePlan == nil {
			return nil, nil, fmt.Errorf("choose next query join: table plan for %q is nil", planner.tableName)
		}
	}
	bestPlan, remaining := chooseLowestJoinPlanFromValid(planners, current)
	return bestPlan, remaining, nil
}

func chooseLowestJoinPlanFromValid(planners []*TablePlanner, current Plan) (Plan, []*TablePlanner) {
	bestIndex := -1
	var bestPlan Plan
	bestRecords := 0
	for index, planner := range planners {
		candidate := planner.MakeJoinPlan(current)
		if candidate == nil {
			continue
		}
		candidateRecords := candidate.RecordsOutput()
		if bestIndex < 0 || candidateRecords < bestRecords {
			bestIndex = index
			bestPlan = candidate
			bestRecords = candidateRecords
		}
	}

	if bestIndex < 0 {
		return nil, append([]*TablePlanner(nil), planners...)
	}
	remaining := make([]*TablePlanner, 0, len(planners)-1)
	remaining = append(remaining, planners[:bestIndex]...)
	remaining = append(remaining, planners[bestIndex+1:]...)
	return bestPlan, remaining
}

func chooseLowestProductPlan(planners []*TablePlanner, current Plan, factory ProductPlanFactory) (Plan, []*TablePlanner, error) {
	if len(planners) == 0 {
		return nil, nil, fmt.Errorf("choose next query product: no table planners")
	}
	if isNilTablePlannerDependency(current) {
		return nil, nil, fmt.Errorf("choose next query product: current plan is nil")
	}
	if factory == nil {
		return nil, nil, fmt.Errorf("choose next query product: product factory is nil")
	}

	bestIndex := -1
	var bestPlan Plan
	bestRecords := 0
	for index, planner := range planners {
		if planner == nil {
			return nil, nil, fmt.Errorf("choose next query product: table planner at index %d is nil", index)
		}
		if planner.tablePlan == nil {
			return nil, nil, fmt.Errorf("choose next query product: table plan for %q is nil", planner.tableName)
		}
		candidate := planner.MakeProductPlan(current, factory)
		if isNilTablePlannerDependency(candidate) {
			return nil, nil, fmt.Errorf("choose next query product: product plan for %q is nil", planner.tableName)
		}
		candidateRecords := candidate.RecordsOutput()
		if bestIndex < 0 || candidateRecords < bestRecords {
			bestIndex = index
			bestPlan = candidate
			bestRecords = candidateRecords
		}
	}

	remaining := make([]*TablePlanner, 0, len(planners)-1)
	remaining = append(remaining, planners[:bestIndex]...)
	remaining = append(remaining, planners[bestIndex+1:]...)
	return bestPlan, remaining, nil
}
