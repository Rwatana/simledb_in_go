package plan

import (
	"errors"
	"fmt"
	"reflect"

	indexquery "github.com/Rwatana/simledb_in_go/internal/index/query"
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
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

// Open creates an index join scan from the retained left plan, right table plan, and index metadata.
func (join *IndexJoinPlan) Open() (query.Scan, error) {
	if join == nil {
		return nil, fmt.Errorf("open nil index join plan")
	}
	if join.left == nil {
		return nil, fmt.Errorf("open index join plan: left plan is nil")
	}
	if join.right == nil {
		return nil, fmt.Errorf("open index join plan: right plan is nil")
	}
	if join.indexInfo == nil {
		return nil, fmt.Errorf("open index join plan: index metadata is nil")
	}

	leftScan, err := join.left.Open()
	if err != nil {
		primary := fmt.Errorf("open index join plan left child: %w", err)
		return nil, errors.Join(primary, closeIndexJoinPlanScan("left", leftScan))
	}
	if isNilIndexJoinPlanScan(leftScan) {
		return nil, fmt.Errorf("open index join plan: left scan is nil")
	}

	rightScan, err := join.right.Open()
	if err != nil {
		primary := fmt.Errorf("open index join plan right child: %w", err)
		return nil, errors.Join(primary,
			closeIndexJoinPlanScan("right", rightScan),
			closeIndexJoinPlanScan("left", leftScan),
		)
	}
	if rightScan == nil {
		return nil, errors.Join(fmt.Errorf("open index join plan: right scan is nil"), closeIndexJoinPlanScan("left", leftScan))
	}
	rightRecordScan, ok := rightScan.(*query.RecordScan)
	if !ok {
		return nil, errors.Join(
			fmt.Errorf("open index join plan: right scan %T is not a query record scan", rightScan),
			closeIndexJoinPlanScan("right", rightScan),
			closeIndexJoinPlanScan("left", leftScan),
		)
	}
	if rightRecordScan == nil || rightRecordScan.TableScan == nil {
		return nil, errors.Join(fmt.Errorf("open index join plan: right table scan is nil"), closeIndexJoinPlanScan("left", leftScan))
	}

	indexScan := join.indexInfo.Open()
	indexJoinScan, err := indexquery.NewIndexJoinScan(leftScan, indexScan, join.joinField, rightRecordScan)
	if err != nil {
		primary := fmt.Errorf("open index join plan scan: %w", err)
		return nil, errors.Join(primary,
			indexScan.Close(),
			closeIndexJoinPlanScan("right", rightRecordScan),
			closeIndexJoinPlanScan("left", leftScan),
		)
	}
	return indexJoinScan, nil
}

// BlocksAccessed estimates the index join's left, index, and output block accesses.
func (join *IndexJoinPlan) BlocksAccessed() int {
	if join == nil || join.left == nil || join.right == nil || join.indexInfo == nil {
		return 0
	}
	indexBlocks, err := join.indexInfo.BlocksAccessed()
	if err != nil {
		return 0
	}
	leftBlocks := int32(join.left.BlocksAccessed())
	leftRecords := int32(join.left.RecordsOutput())
	indexBlocks32 := int32(indexBlocks)
	indexRecords := int32(join.indexInfo.RecordsOutput())
	return int(leftBlocks + leftRecords*indexBlocks32 + leftRecords*indexRecords)
}

func isNilIndexJoinPlanScan(scan query.Scan) bool {
	if scan == nil {
		return true
	}
	value := reflect.ValueOf(scan)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func closeIndexJoinPlanScan(side string, scan query.Scan) error {
	if isNilIndexJoinPlanScan(scan) {
		return nil
	}
	if recordScan, ok := scan.(*query.RecordScan); ok && (recordScan == nil || recordScan.TableScan == nil) {
		return nil
	}
	if err := scan.Close(); err != nil {
		return fmt.Errorf("close index join plan %s scan after failure: %w", side, err)
	}
	return nil
}
