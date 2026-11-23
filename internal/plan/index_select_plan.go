package plan

import (
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// IndexSelectPlan retains a child plan and index metadata for one indexed selection.
type IndexSelectPlan struct {
	child     Plan
	indexInfo *metadata.IndexInfo
	searchKey query.Constant
}

// NewIndexSelectPlan creates an indexed selection over child for searchKey.
func NewIndexSelectPlan(child Plan, indexInfo *metadata.IndexInfo, searchKey query.Constant) *IndexSelectPlan {
	return &IndexSelectPlan{child: child, indexInfo: indexInfo, searchKey: searchKey}
}

// Schema returns the child schema exposed by the index selection.
func (p *IndexSelectPlan) Schema() *record.Schema {
	if p == nil || p.child == nil {
		return nil
	}
	return p.child.Schema()
}
