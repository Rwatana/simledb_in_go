package plan

import "fmt"

func chooseLowestSelectPlan(planners []*TablePlanner) (Plan, []*TablePlanner, error) {
	if len(planners) == 0 {
		return nil, nil, fmt.Errorf("choose initial query plan: no table planners")
	}

	bestIndex := -1
	var bestPlan Plan
	bestRecords := 0
	for index, planner := range planners {
		if planner == nil {
			return nil, nil, fmt.Errorf("choose initial query plan: table planner at index %d is nil", index)
		}
		candidate := planner.MakeSelectPlan()
		if isNilTablePlannerDependency(candidate) {
			return nil, nil, fmt.Errorf("choose initial query plan: table plan for %q is nil", planner.tableName)
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

func chooseLowestJoinPlan(planners []*TablePlanner, current Plan) (Plan, []*TablePlanner, error) {
	if len(planners) == 0 {
		return nil, nil, fmt.Errorf("choose next query join: no table planners")
	}
	if isNilTablePlannerDependency(current) {
		return nil, nil, fmt.Errorf("choose next query join: current plan is nil")
	}

	bestIndex := -1
	var bestPlan Plan
	bestRecords := 0
	for index, planner := range planners {
		if planner == nil {
			return nil, nil, fmt.Errorf("choose next query join: table planner at index %d is nil", index)
		}
		if planner.tablePlan == nil {
			return nil, nil, fmt.Errorf("choose next query join: table plan for %q is nil", planner.tableName)
		}
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
		return nil, append([]*TablePlanner(nil), planners...), nil
	}
	remaining := make([]*TablePlanner, 0, len(planners)-1)
	remaining = append(remaining, planners[:bestIndex]...)
	remaining = append(remaining, planners[bestIndex+1:]...)
	return bestPlan, remaining, nil
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
