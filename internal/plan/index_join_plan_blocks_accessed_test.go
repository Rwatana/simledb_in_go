package plan

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexJoinPlanBlocksAccessedUsesIndexJoinFormula(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, 10_000)
	left := &productPlanProbe{blocks: 11, records: 4}
	right := &productPlanProbe{}
	join := NewIndexJoinPlan(left, right, indexInfo, "student_id")

	got := join.BlocksAccessed()
	if got != 31 {
		t.Fatalf("BlocksAccessed() = %d, want 31 from B(left) + R(left)*B(index) + R(join)", got)
	}
}

func TestIndexJoinPlanBlocksAccessedReturnsZeroForMissingDependencies(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, 10_000)
	left := &productPlanProbe{blocks: 11, records: 4}
	right := &productPlanProbe{}
	var nilPlan *IndexJoinPlan
	cases := map[string]*IndexJoinPlan{
		"nil receiver": nilPlan,
		"nil left":     NewIndexJoinPlan(nil, right, indexInfo, "student_id"),
		"nil right":    NewIndexJoinPlan(left, nil, indexInfo, "student_id"),
		"nil index":    NewIndexJoinPlan(left, right, nil, "student_id"),
	}
	for name, join := range cases {
		t.Run(name, func(t *testing.T) {
			if got := join.BlocksAccessed(); got != 0 {
				t.Fatalf("BlocksAccessed() = %d, want zero for %s", got, name)
			}
		})
	}
}

func TestIndexJoinPlanBlocksAccessedReturnsZeroWhenIndexEstimateFails(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{
		tablePlanTransaction: &tablePlanTransaction{},
		blockSizeErr:         errors.New("block size unavailable"),
	}, &schema, 10_000)
	join := NewIndexJoinPlan(&productPlanProbe{blocks: 11, records: 4}, &productPlanProbe{}, indexInfo, "student_id")

	if got := join.BlocksAccessed(); got != 0 {
		t.Fatalf("BlocksAccessed() = %d, want zero when index metadata cannot estimate", got)
	}
}

func TestIndexJoinPlanBlocksAccessedPreservesJavaIntOverflow(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	maxInt32 := int(^uint32(0) >> 1)
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, maxInt32)
	indexBlocks, err := indexInfo.BlocksAccessed()
	if err != nil {
		t.Fatalf("IndexInfo.BlocksAccessed() error = %v", err)
	}
	left := &productPlanProbe{blocks: maxInt32, records: maxInt32}
	join := NewIndexJoinPlan(left, &productPlanProbe{}, indexInfo, "student_id")
	want := int(int32(maxInt32) + int32(maxInt32)*int32(indexBlocks) + int32(maxInt32)*int32(indexInfo.RecordsOutput()))

	if got := join.BlocksAccessed(); got != want {
		t.Fatalf("BlocksAccessed() = %d, want Java signed-32-bit result %d", got, want)
	}
}

func newIndexJoinPlanEstimateInfo(t *testing.T, transaction record.TableScanTransaction, schema *record.Schema, records int) *metadata.IndexInfo {
	t.Helper()
	info, err := metadata.NewIndexInfo("course_idx", "course_id", schema, transaction, metadata.NewStatInfo(100, records))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	return info
}

type indexJoinPlanEstimateTransaction struct {
	*tablePlanTransaction
	blockSize    int
	blockSizeErr error
}

func (transaction *indexJoinPlanEstimateTransaction) BlockSize() (int, error) {
	return transaction.blockSize, transaction.blockSizeErr
}
