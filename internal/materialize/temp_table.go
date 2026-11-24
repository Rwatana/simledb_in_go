// Package materialize builds temporary relations for query processing.
package materialize

import (
	"fmt"
	"sync/atomic"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

var nextTemporaryTableNumber atomic.Uint64

// TempTable describes a temporary relation and retains the transaction used to access it.
type TempTable struct {
	transaction record.TableScanTransaction
	tableName   string
	layout      *record.Layout
}

// NewTempTable creates a temporary relation descriptor without creating its physical table.
func NewTempTable(transaction record.TableScanTransaction, schema *record.Schema) (*TempTable, error) {
	if transaction == nil {
		return nil, fmt.Errorf("create temporary table: transaction must not be nil")
	}
	if schema == nil {
		return nil, fmt.Errorf("create temporary table: schema must not be nil")
	}

	name := fmt.Sprintf("temp%d", nextTemporaryTableNumber.Add(1))
	return &TempTable{
		transaction: transaction,
		tableName:   name,
		layout:      record.NewLayout(schema),
	}, nil
}

// TableName returns the generated name of the temporary relation.
func (t *TempTable) TableName() string {
	return t.tableName
}

// Layout returns the physical record layout derived from the temporary relation schema.
func (t *TempTable) Layout() *record.Layout {
	return t.layout
}
