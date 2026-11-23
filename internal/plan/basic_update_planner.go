package plan

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// BasicUpdatePlannerMetadata provides catalog operations required by update commands.
type BasicUpdatePlannerMetadata interface {
	Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error)
	CreateTable(tableName string, schema *record.Schema, transaction record.TableScanTransaction) error
	CreateView(viewName, definition string, transaction record.TableScanTransaction) error
	CreateIndex(indexName, tableName, fieldName string, transaction record.TableScanTransaction) error
}

// BasicUpdatePlanner executes parsed SQL update commands using table and selection plans.
type BasicUpdatePlanner struct {
	metadata BasicUpdatePlannerMetadata
}

// NewBasicUpdatePlanner creates an update planner backed by metadata.
func NewBasicUpdatePlanner(metadata BasicUpdatePlannerMetadata) *BasicUpdatePlanner {
	return &BasicUpdatePlanner{metadata: metadata}
}

// ExecuteDelete deletes rows that satisfy data's predicate and returns the number deleted.
func (planner *BasicUpdatePlanner) ExecuteDelete(data parse.DeleteData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute delete: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute delete: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute delete: transaction is nil")
	}
	tableName := data.TableName()
	if tableName == "" {
		return 0, fmt.Errorf("execute delete: table name is empty")
	}
	layout, err := planner.metadata.Layout(tableName, transaction)
	if err != nil {
		return 0, fmt.Errorf("execute delete for table %q: resolve layout: %w", tableName, err)
	}
	if layout == nil {
		return 0, fmt.Errorf("execute delete for table %q: layout is nil", tableName)
	}
	selection := NewSelectPlan(NewTablePlan(transaction, tableName, layout, nil), data.Predicate())
	scan, err := selection.Open()
	if err != nil {
		return 0, fmt.Errorf("execute delete for table %q: open selection scan: %w", tableName, err)
	}
	return deleteRowsFromScan(tableName, scan)
}

// ExecuteModify evaluates the update expression for matching rows and returns their count.
func (planner *BasicUpdatePlanner) ExecuteModify(data parse.ModifyData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute update: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute update: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute update: transaction is nil")
	}
	tableName := data.TableName()
	if tableName == "" {
		return 0, fmt.Errorf("execute update: table name is empty")
	}
	targetField := data.TargetField()
	if targetField == "" {
		return 0, fmt.Errorf("execute update for table %q: target field is empty", tableName)
	}
	layout, err := planner.metadata.Layout(tableName, transaction)
	if err != nil {
		return 0, fmt.Errorf("execute update for table %q: resolve layout: %w", tableName, err)
	}
	if layout == nil {
		return 0, fmt.Errorf("execute update for table %q: layout is nil", tableName)
	}
	selection := NewSelectPlan(NewTablePlan(transaction, tableName, layout, nil), data.Predicate())
	scan, err := selection.Open()
	if err != nil {
		return 0, fmt.Errorf("execute update for table %q: open selection scan: %w", tableName, err)
	}
	return modifyRowsFromScan(tableName, targetField, data.NewValue(), scan)
}

func modifyRowsFromScan(tableName, targetField string, expression query.Expression, scan query.Scan) (int, error) {
	updateScan, err := requireUpdateScan(scan)
	if err != nil {
		return 0, closeUpdatePlannerScan(scan, fmt.Errorf("execute update for table %q: %w", tableName, err))
	}

	modified := 0
	for {
		next, err := updateScan.Next()
		if err != nil {
			primary := fmt.Errorf("execute update for table %q: advance selection scan: %w", tableName, err)
			return modified, closeUpdatePlannerScan(scan, primary)
		}
		if !next {
			break
		}
		value, err := expression.Evaluate(updateScan)
		if err != nil {
			primary := fmt.Errorf("execute update for table %q: evaluate replacement expression: %w", tableName, err)
			return modified, closeUpdatePlannerScan(scan, primary)
		}
		if err := updateScan.SetVal(targetField, value); err != nil {
			primary := fmt.Errorf("execute update for table %q: write field %q: %w", tableName, targetField, err)
			return modified, closeUpdatePlannerScan(scan, primary)
		}
		modified++
	}
	if err := closeUpdatePlannerScan(scan, nil); err != nil {
		return modified, fmt.Errorf("execute update for table %q: close selection scan: %w", tableName, err)
	}
	return modified, nil
}

func deleteRowsFromScan(tableName string, scan query.Scan) (int, error) {
	updateScan, err := requireUpdateScan(scan)
	if err != nil {
		return 0, closeUpdatePlannerScan(scan, fmt.Errorf("execute delete for table %q: %w", tableName, err))
	}

	deleted := 0
	for {
		next, err := updateScan.Next()
		if err != nil {
			primary := fmt.Errorf("execute delete for table %q: advance selection scan: %w", tableName, err)
			return deleted, closeUpdatePlannerScan(scan, primary)
		}
		if !next {
			break
		}
		if err := updateScan.Delete(); err != nil {
			primary := fmt.Errorf("execute delete for table %q: delete current row: %w", tableName, err)
			return deleted, closeUpdatePlannerScan(scan, primary)
		}
		deleted++
	}
	if err := closeUpdatePlannerScan(scan, nil); err != nil {
		return deleted, fmt.Errorf("execute delete for table %q: close selection scan: %w", tableName, err)
	}
	return deleted, nil
}

func requireUpdateScan(scan query.Scan) (query.UpdateScan, error) {
	if scan == nil {
		return nil, fmt.Errorf("update scan is nil")
	}
	updateScan, ok := scan.(query.UpdateScan)
	if !ok {
		return nil, fmt.Errorf("scan %T does not support updates", scan)
	}
	return updateScan, nil
}

func closeUpdatePlannerScan(scan query.Scan, primary error) error {
	if scan == nil {
		return primary
	}
	if err := scan.Close(); err != nil {
		return errors.Join(primary, fmt.Errorf("close update scan: %w", err))
	}
	return primary
}
