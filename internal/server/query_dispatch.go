package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

// CreateQueryPlan parses a SELECT statement and plans it with the manager set's query planner.
func (m *Managers) CreateQueryPlan(statement string, transaction *transaction.Transaction) (plan.Plan, error) {
	if m == nil || m.QueryPlanner == nil {
		return nil, fmt.Errorf("create server query plan: manager set is incomplete")
	}
	if transaction == nil {
		return nil, fmt.Errorf("create server query plan: transaction is nil")
	}
	data, err := parse.NewParser(statement).ParseQuery()
	if err != nil {
		return nil, fmt.Errorf("create server query plan: parse SELECT statement: %w", err)
	}
	queryPlan, err := m.QueryPlanner.CreatePlan(data, transaction)
	if err != nil {
		return nil, fmt.Errorf("create server query plan: %w", err)
	}
	return queryPlan, nil
}
