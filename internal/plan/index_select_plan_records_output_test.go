package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectPlanRecordsOutputDelegatesToIndexMetadata(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	info, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, &tablePlanTransaction{}, metadata.NewStatInfo(3, 12))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	plan := NewIndexSelectPlan(nil, info, query.NewIntConstant(6))
	if got, want := plan.RecordsOutput(), 2; got != want {
		t.Fatalf("RecordsOutput() = %d, want matching-row estimate %d", got, want)
	}
}

func TestIndexSelectPlanRecordsOutputReturnsZeroWithoutIndexMetadata(t *testing.T) {
	if got := (*IndexSelectPlan)(nil).RecordsOutput(); got != 0 {
		t.Fatalf("nil plan RecordsOutput() = %d, want 0", got)
	}
	if got := NewIndexSelectPlan(nil, nil, query.Constant{}).RecordsOutput(); got != 0 {
		t.Fatalf("plan without index metadata RecordsOutput() = %d, want 0", got)
	}
}
