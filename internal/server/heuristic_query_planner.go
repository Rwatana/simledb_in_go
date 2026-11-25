package server

import (
	"github.com/Rwatana/simledb_in_go/internal/materialize"
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

type heuristicQueryPlanner struct {
	metadata *metadata.MetadataManager
}

func (planner *heuristicQueryPlanner) CreatePlan(data parse.QueryData, transaction record.TableScanTransaction) (plan.Plan, error) {
	return plan.NewHeuristicQueryPlanner(planner.metadata, heuristicProductPlanFactory(transaction)).CreatePlan(data, transaction)
}

func heuristicProductPlanFactory(transaction record.TableScanTransaction) plan.ProductPlanFactory {
	multibufferTransaction, ok := transaction.(materialize.MultibufferProductPlanTransaction)
	if !ok {
		return nil
	}
	return materialize.NewMultibufferProductPlanFactory(multibufferTransaction)
}
