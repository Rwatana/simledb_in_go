package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// TablePlan is a leaf query plan backed by one table's resolved record layout.
type TablePlan struct {
	transaction record.TableScanTransaction
	tableName   string
	layout      *record.Layout
}

// NewTablePlan creates a plan for tableName using its transaction and resolved layout.
func NewTablePlan(transaction record.TableScanTransaction, tableName string, layout *record.Layout) *TablePlan {
	return &TablePlan{transaction: transaction, tableName: tableName, layout: layout}
}

// Schema returns the logical schema resolved for the table, or nil if no layout is available.
func (p *TablePlan) Schema() *record.Schema {
	if p == nil || p.layout == nil {
		return nil
	}
	return p.layout.Schema()
}

// Open creates a table scan positioned before the first record.
func (p *TablePlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open table plan: nil plan")
	}
	if p.transaction == nil {
		return nil, fmt.Errorf("open table plan for %q: transaction is nil", p.tableName)
	}
	if p.layout == nil {
		return nil, fmt.Errorf("open table plan for %q: layout is nil", p.tableName)
	}
	scan, err := query.NewRecordScan(p.transaction, p.tableName, p.layout)
	if err != nil {
		return nil, fmt.Errorf("open table plan for %q: %w", p.tableName, err)
	}
	return scan, nil
}
