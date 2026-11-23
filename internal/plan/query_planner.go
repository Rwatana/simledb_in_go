package plan

import (
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// QueryPlanner creates executable plans from parsed SELECT data and a transaction.
type QueryPlanner interface {
	// CreatePlan builds a plan for data using transaction for metadata and scans.
	CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (Plan, error)
}
