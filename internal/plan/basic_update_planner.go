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

// ExecuteInsert inserts one record using the parsed field names and values.
func (planner *BasicUpdatePlanner) ExecuteInsert(data parse.InsertData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute insert: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute insert: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute insert: transaction is nil")
	}
	tableName := data.TableName()
	if tableName == "" {
		return 0, fmt.Errorf("execute insert: table name is empty")
	}
	fields := data.Fields()
	values := data.Values()
	if len(fields) == 0 {
		return 0, fmt.Errorf("execute insert for table %q: no fields provided", tableName)
	}
	if len(fields) != len(values) {
		return 0, fmt.Errorf("execute insert for table %q: field/value count mismatch: %d fields, %d values", tableName, len(fields), len(values))
	}
	for _, field := range fields {
		if field == "" {
			return 0, fmt.Errorf("execute insert for table %q: field name is empty", tableName)
		}
	}
	layout, err := planner.metadata.Layout(tableName, transaction)
	if err != nil {
		return 0, fmt.Errorf("execute insert for table %q: resolve layout: %w", tableName, err)
	}
	if layout == nil {
		return 0, fmt.Errorf("execute insert for table %q: layout is nil", tableName)
	}
	plan := NewTablePlan(transaction, tableName, layout, nil)
	scan, err := plan.Open()
	if err != nil {
		return 0, fmt.Errorf("execute insert for table %q: open table scan: %w", tableName, err)
	}
	return insertRowIntoScan(tableName, fields, values, scan)
}

// ExecuteCreateTable creates the table described by data and returns no row count.
func (planner *BasicUpdatePlanner) ExecuteCreateTable(data parse.CreateTableData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute create table: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute create table: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute create table: transaction is nil")
	}
	tableName := data.TableName()
	if tableName == "" {
		return 0, fmt.Errorf("execute create table: table name is empty")
	}
	schema := data.Schema()
	if err := planner.metadata.CreateTable(tableName, &schema, transaction); err != nil {
		return 0, fmt.Errorf("execute create table %q: create metadata: %w", tableName, err)
	}
	return 0, nil
}

// ExecuteCreateView stores the parsed view definition and returns no row count.
func (planner *BasicUpdatePlanner) ExecuteCreateView(data parse.CreateViewData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute create view: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute create view: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute create view: transaction is nil")
	}
	viewName := data.ViewName()
	if viewName == "" {
		return 0, fmt.Errorf("execute create view: view name is empty")
	}
	definition := data.ViewDefinition()
	if definition == "" || definition == "<unset-query>" {
		return 0, fmt.Errorf("execute create view %q: definition is unset", viewName)
	}
	if err := planner.metadata.CreateView(viewName, definition, transaction); err != nil {
		return 0, fmt.Errorf("execute create view %q: create metadata: %w", viewName, err)
	}
	return 0, nil
}

// ExecuteCreateIndex asks metadata to create the parsed index and returns no row count.
func (planner *BasicUpdatePlanner) ExecuteCreateIndex(data parse.CreateIndexData, transaction record.TableScanTransaction) (int, error) {
	if planner == nil {
		return 0, fmt.Errorf("execute create index: planner is nil")
	}
	if planner.metadata == nil {
		return 0, fmt.Errorf("execute create index: metadata is nil")
	}
	if transaction == nil {
		return 0, fmt.Errorf("execute create index: transaction is nil")
	}
	indexName := data.IndexName()
	if indexName == "" {
		return 0, fmt.Errorf("execute create index: index name is empty")
	}
	tableName := data.TableName()
	if tableName == "" {
		return 0, fmt.Errorf("execute create index %q: table name is empty", indexName)
	}
	fieldName := data.FieldName()
	if fieldName == "" {
		return 0, fmt.Errorf("execute create index %q: field name is empty", indexName)
	}
	if err := planner.metadata.CreateIndex(indexName, tableName, fieldName, transaction); err != nil {
		return 0, fmt.Errorf("execute create index %q on %s.%s: create metadata: %w", indexName, tableName, fieldName, err)
	}
	return 0, nil
}

func insertRowIntoScan(tableName string, fields []string, values []query.Constant, scan query.Scan) (int, error) {
	updateScan, err := requireUpdateScan(scan)
	if err != nil {
		return 0, closeUpdatePlannerScan(scan, fmt.Errorf("execute insert for table %q: %w", tableName, err))
	}
	if err := updateScan.Insert(); err != nil {
		primary := fmt.Errorf("execute insert for table %q: insert row: %w", tableName, err)
		return 0, closeUpdatePlannerScan(scan, primary)
	}
	for index, field := range fields {
		if err := updateScan.SetVal(field, values[index]); err != nil {
			primary := fmt.Errorf("execute insert for table %q: set field %q: %w", tableName, field, err)
			return 0, closeUpdatePlannerScan(scan, primary)
		}
	}
	if err := closeUpdatePlannerScan(scan, nil); err != nil {
		return 1, fmt.Errorf("execute insert for table %q: close table scan: %w", tableName, err)
	}
	return 1, nil
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
