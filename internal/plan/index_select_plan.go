package plan

import (
	"errors"
	"fmt"

	indexquery "github.com/Rwatana/simledb_in_go/internal/index/query"
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

// Open opens the child table scan and positions its index-select scan before the first match.
func (p *IndexSelectPlan) Open() (query.Scan, error) {
	if p == nil {
		return nil, fmt.Errorf("open nil index select plan")
	}
	if p.child == nil {
		return nil, fmt.Errorf("open index select plan: child plan is nil")
	}
	if p.indexInfo == nil {
		return nil, fmt.Errorf("open index select plan: index metadata is nil")
	}
	if _, ok := p.searchKey.HashCode(); !ok {
		return nil, fmt.Errorf("open index select plan: search key is unset")
	}
	childScan, err := p.child.Open()
	if err != nil {
		primary := fmt.Errorf("open index select plan child plan: %w", err)
		if childScan == nil {
			return nil, primary
		}
		return nil, errors.Join(primary, childScan.Close())
	}
	if childScan == nil {
		return nil, fmt.Errorf("open index select plan: child scan is nil")
	}
	dataScan, ok := childScan.(*query.RecordScan)
	if !ok {
		return nil, errors.Join(fmt.Errorf("open index select plan: child scan %T is not a query record scan", childScan), childScan.Close())
	}
	if dataScan == nil || dataScan.TableScan == nil {
		return nil, fmt.Errorf("open index select plan: child query record scan table scan is nil")
	}
	indexScan := p.indexInfo.Open()
	scan, err := indexquery.NewIndexSelectScan(dataScan, indexScan, p.searchKey)
	if err != nil {
		primary := fmt.Errorf("open index select plan index scan: %w", err)
		return nil, errors.Join(primary, indexScan.Close(), dataScan.Close())
	}
	return scan, nil
}
