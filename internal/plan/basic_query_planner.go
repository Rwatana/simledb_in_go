package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// BasicPlannerMetadata provides the table metadata required by BasicQueryPlanner.
type BasicPlannerMetadata interface {
	ViewDefinition(viewName string, transaction record.TableScanTransaction) (string, bool, error)
	Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error)
	Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*metadata.StatInfo, error)
}

// BasicQueryPlanner builds plans for SELECT statements over tables and views.
type BasicQueryPlanner struct {
	metadata BasicPlannerMetadata
}

// NewBasicQueryPlanner creates a planner that resolves table layouts and estimates through metadata.
func NewBasicQueryPlanner(metadata BasicPlannerMetadata) *BasicQueryPlanner {
	return &BasicQueryPlanner{metadata: metadata}
}

// CreatePlan composes table scans, a selection, and a projection for a parsed query.
func (planner *BasicQueryPlanner) CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (Plan, error) {
	return planner.createPlan(data, transaction, make(map[string]bool))
}

func (planner *BasicQueryPlanner) createPlan(data parse.QueryData, transaction record.TableScanTransaction, viewStack map[string]bool) (Plan, error) {
	sources, err := planner.sourcePlans(data, transaction, viewStack)
	if err != nil {
		return nil, err
	}
	var root Plan
	for _, source := range sources {
		if root == nil {
			root = source
			continue
		}
		root = NewProductPlan(root, source)
	}
	return finishQueryPlan(root, data)
}

// sourcePlans resolves physical tables and recursively expands views in source order.
func (planner *BasicQueryPlanner) sourcePlans(data parse.QueryData, transaction record.TableScanTransaction, viewStack map[string]bool) ([]Plan, error) {
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

	sources := make([]Plan, 0, len(tableNames))
	for _, tableName := range tableNames {
		definition, isView, err := planner.metadata.ViewDefinition(tableName, transaction)
		if err != nil {
			return nil, fmt.Errorf("create basic query plan for view %q: resolve definition: %w", tableName, err)
		}
		if isView {
			if viewStack[tableName] {
				return nil, fmt.Errorf("create basic query plan: recursive view %q", tableName)
			}
			viewStack[tableName] = true
			viewData, err := parse.NewParser(definition).ParseQuery()
			if err != nil {
				delete(viewStack, tableName)
				return nil, fmt.Errorf("create basic query plan for view %q: parse definition: %w", tableName, err)
			}
			viewPlan, err := planner.createPlan(viewData, transaction, viewStack)
			delete(viewStack, tableName)
			if err != nil {
				return nil, fmt.Errorf("create basic query plan for view %q: %w", tableName, err)
			}
			sources = append(sources, viewPlan)
			continue
		}

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
		sources = append(sources, tablePlan)
	}
	return sources, nil
}

func finishQueryPlan(root Plan, data parse.QueryData) (Plan, error) {
	selected := NewSelectPlan(root, data.Predicate())
	projected, err := NewProjectPlan(selected, data.Fields())
	if err != nil {
		return nil, fmt.Errorf("create basic query plan projection: %w", err)
	}
	return projected, nil
}
