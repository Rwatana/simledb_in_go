package plan

import "github.com/Rwatana/simledb_in_go/internal/record"

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
