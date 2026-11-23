package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// BasicPlannerMetadata provides the table metadata required by BasicQueryPlanner.
type BasicPlannerMetadata interface {
	Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error)
	Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*metadata.StatInfo, error)
}

// BasicQueryPlanner builds plans for SELECT statements over physical tables.
type BasicQueryPlanner struct {
	metadata BasicPlannerMetadata
}

// NewBasicQueryPlanner creates a planner that resolves table layouts and estimates through metadata.
func NewBasicQueryPlanner(metadata BasicPlannerMetadata) *BasicQueryPlanner {
	return &BasicQueryPlanner{metadata: metadata}
}

// CreatePlan composes table scans, a selection, and a projection for a parsed query.
func (planner *BasicQueryPlanner) CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (Plan, error) {
	if planner == nil {
		return nil, fmt.Errorf("create basic query plan: planner is nil")
	}
	if planner.metadata == nil {
		return nil, fmt.Errorf("create basic query plan: metadata is nil")
	}
	if transaction == nil {
		return nil, fmt.Errorf("create basic query plan: transaction is nil")
	}
	tableNames := data.Tables()
	if len(tableNames) == 0 {
		return nil, fmt.Errorf("create basic query plan: query has no tables")
	}

	var root Plan
	for _, tableName := range tableNames {
		layout, err := planner.metadata.Layout(tableName, transaction)
		if err != nil {
			return nil, fmt.Errorf("create basic query plan for table %q: resolve layout: %w", tableName, err)
		}
		if layout == nil {
			return nil, fmt.Errorf("create basic query plan for table %q: layout is nil", tableName)
		}
		statistics, err := planner.metadata.Statistics(tableName, layout, transaction)
		if err != nil {
			return nil, fmt.Errorf("create basic query plan for table %q: read statistics: %w", tableName, err)
		}
		tablePlan := NewTablePlan(transaction, tableName, layout, statistics)
		if root == nil {
			root = tablePlan
			continue
		}
		root = NewProductPlan(root, tablePlan)
	}

	selected := NewSelectPlan(root, data.Predicate())
	projected, err := NewProjectPlan(selected, data.Fields())
	if err != nil {
		return nil, fmt.Errorf("create basic query plan projection: %w", err)
	}
	return projected, nil
}
