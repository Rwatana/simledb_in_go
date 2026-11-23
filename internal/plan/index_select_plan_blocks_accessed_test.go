package plan

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectPlanBlocksAccessedAddsIndexAndMatchingRows(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	info, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, &tablePlanTransaction{}, metadata.NewStatInfo(1, 32_000))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	plan := NewIndexSelectPlan(&productPlanProbe{schema: &schema}, info, query.NewIntConstant(6))
	if got, want := plan.BlocksAccessed(), 12; got != want {
		t.Fatalf("BlocksAccessed() = %d, want index traversal plus matching rows = %d", got, want)
	}
}

func TestIndexSelectPlanBlocksAccessedReturnsZeroWhenUnavailable(t *testing.T) {
	if got := (*IndexSelectPlan)(nil).BlocksAccessed(); got != 0 {
		t.Fatalf("nil plan BlocksAccessed() = %d, want 0", got)
	}
	if got := NewIndexSelectPlan(nil, nil, query.Constant{}).BlocksAccessed(); got != 0 {
		t.Fatalf("plan without index metadata BlocksAccessed() = %d, want 0", got)
	}

	var schema record.Schema
	schema.AddIntegerField("student_id")
	blockSizeErr := errors.New("block size unavailable")
	transaction := &indexSelectPlanBlockSizeTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSizeErr: blockSizeErr}
	info, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, transaction, metadata.NewStatInfo(1, 10))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	if got := NewIndexSelectPlan(nil, info, query.NewIntConstant(6)).BlocksAccessed(); got != 0 {
		t.Fatalf("unavailable index estimate BlocksAccessed() = %d, want 0", got)
	}
}

func TestIndexSelectPlanBlocksAccessedUsesSignedInt32Addition(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("the overflow case requires a 64-bit int")
	}
	var schema record.Schema
	schema.AddIntegerField("student_id")
	recordCount := int(1 << 43)
	info, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, &tablePlanTransaction{}, metadata.NewStatInfo(1, recordCount))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	indexCost, err := info.BlocksAccessed()
	if err != nil {
		t.Fatalf("IndexInfo.BlocksAccessed() error = %v", err)
	}
	rowEstimate := info.RecordsOutput()
	if int64(indexCost)+int64(rowEstimate) <= 1<<31-1 {
		t.Fatalf("test estimate %d + %d does not overflow signed int32", indexCost, rowEstimate)
	}
	plan := NewIndexSelectPlan(nil, info, query.NewIntConstant(6))
	if got, want := plan.BlocksAccessed(), int(int32(indexCost)+int32(rowEstimate)); got != want {
		t.Fatalf("BlocksAccessed() = %d, want signed int32 result %d", got, want)
	}
}

type indexSelectPlanBlockSizeTransaction struct {
	*tablePlanTransaction
	blockSizeErr error
}

func (transaction *indexSelectPlanBlockSizeTransaction) BlockSize() (int, error) {
	return 512, transaction.blockSizeErr
}

var _ record.TableScanTransaction = (*indexSelectPlanBlockSizeTransaction)(nil)
