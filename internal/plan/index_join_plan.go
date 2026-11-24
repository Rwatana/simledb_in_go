package plan

import (
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// IndexJoinPlan retains the plans, index metadata, and output schema for an index join.
type IndexJoinPlan struct {
	left      Plan
	right     Plan
	indexInfo *metadata.IndexInfo
	joinField string
	schema    record.Schema
}

// NewIndexJoinPlan creates an index join plan with a left-then-right output schema.
func NewIndexJoinPlan(left, right Plan, indexInfo *metadata.IndexInfo, joinField string) *IndexJoinPlan {
	join := &IndexJoinPlan{left: left, right: right, indexInfo: indexInfo, joinField: joinField}
	if left != nil {
		if schema := left.Schema(); schema != nil {
			join.schema.AddAllFrom(schema)
		}
	}
	if right != nil {
		if schema := right.Schema(); schema != nil {
			join.schema.AddAllFrom(schema)
		}
	}
	return join
}

// Schema returns the combined left-then-right schema of the index join.
func (join *IndexJoinPlan) Schema() *record.Schema {
	if join == nil {
		return nil
	}
	return &join.schema
}
