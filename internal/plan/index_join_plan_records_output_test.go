package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexJoinPlanRecordsOutputMultipliesLeftAndIndexEstimates(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, 10_000)
	join := NewIndexJoinPlan(&productPlanProbe{records: 4}, nil, indexInfo, "student_id")

	if got := join.RecordsOutput(); got != 8 {
		t.Fatalf("RecordsOutput() = %d, want 8", got)
	}
}

func TestIndexJoinPlanRecordsOutputReturnsZeroForMissingLeftOrIndex(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, 10_000)
	left := &productPlanProbe{records: 4}
	var nilPlan *IndexJoinPlan
	cases := map[string]*IndexJoinPlan{
		"nil receiver": nilPlan,
		"nil left":     NewIndexJoinPlan(nil, nil, indexInfo, "student_id"),
		"nil index":    NewIndexJoinPlan(left, nil, nil, "student_id"),
	}
	for name, join := range cases {
		t.Run(name, func(t *testing.T) {
			if got := join.RecordsOutput(); got != 0 {
				t.Fatalf("RecordsOutput() = %d, want zero for %s", got, name)
			}
		})
	}
}

func TestIndexJoinPlanRecordsOutputPreservesJavaIntOverflow(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("course_id")
	maxInt32 := int(^uint32(0) >> 1)
	indexInfo := newIndexJoinPlanEstimateInfo(t, &indexJoinPlanEstimateTransaction{tablePlanTransaction: &tablePlanTransaction{}, blockSize: 512}, &schema, maxInt32)
	left := &productPlanProbe{records: maxInt32}
	join := NewIndexJoinPlan(left, nil, indexInfo, "student_id")
	want := int(int32(maxInt32) * int32(indexInfo.RecordsOutput()))

	if got := join.RecordsOutput(); got != want {
		t.Fatalf("RecordsOutput() = %d, want Java signed-32-bit result %d", got, want)
	}
}
