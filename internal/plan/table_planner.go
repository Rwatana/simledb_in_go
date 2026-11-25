package plan

import (
	"fmt"
	"reflect"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// TablePlannerMetadata provides the metadata needed to prepare one table for heuristic planning.
type TablePlannerMetadata interface {
	// Layout resolves the physical record layout for a table.
	Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error)
	// Statistics estimates row and block counts for a resolved table layout.
	Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*metadata.StatInfo, error)
	// IndexInfo resolves the registered indexes for a table.
	IndexInfo(tableName string, transaction record.TableScanTransaction) (map[string]*metadata.IndexInfo, error)
}

// TablePlanner retains the resolved state for planning one table in a query.
type TablePlanner struct {
	tableName   string
	predicate   query.Predicate
	transaction record.TableScanTransaction
	tablePlan   *TablePlan
	schema      *record.Schema
	indexes     map[string]*metadata.IndexInfo
}

// NewTablePlanner resolves and retains one table's plan, schema, statistics, and index metadata.
func NewTablePlanner(tableName string, predicate query.Predicate, transaction record.TableScanTransaction, source TablePlannerMetadata) (*TablePlanner, error) {
	if isNilTablePlannerDependency(transaction) {
		return nil, fmt.Errorf("initialize table planner for %q: transaction is nil", tableName)
	}
	if isNilTablePlannerDependency(source) {
		return nil, fmt.Errorf("initialize table planner for %q: metadata is nil", tableName)
	}

	layout, err := source.Layout(tableName, transaction)
	if err != nil {
		return nil, fmt.Errorf("initialize table planner for %q: resolve layout: %w", tableName, err)
	}
	if layout == nil {
		return nil, fmt.Errorf("initialize table planner for %q: layout is nil", tableName)
	}
	schema := layout.Schema()
	if schema == nil {
		return nil, fmt.Errorf("initialize table planner for %q: layout schema is nil", tableName)
	}

	statistics, err := source.Statistics(tableName, layout, transaction)
	if err != nil {
		return nil, fmt.Errorf("initialize table planner for %q: resolve statistics: %w", tableName, err)
	}
	if statistics == nil {
		return nil, fmt.Errorf("initialize table planner for %q: statistics are nil", tableName)
	}

	indexes, err := source.IndexInfo(tableName, transaction)
	if err != nil {
		return nil, fmt.Errorf("initialize table planner for %q: resolve indexes: %w", tableName, err)
	}
	if indexes == nil {
		indexes = make(map[string]*metadata.IndexInfo)
	}

	return &TablePlanner{
		tableName:   tableName,
		predicate:   predicate,
		transaction: transaction,
		tablePlan:   NewTablePlan(transaction, tableName, layout, statistics),
		schema:      schema,
		indexes:     indexes,
	}, nil
}

func isNilTablePlannerDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	return value.Kind() == reflect.Pointer && value.IsNil()
}
