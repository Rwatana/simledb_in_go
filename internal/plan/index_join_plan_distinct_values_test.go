package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexJoinPlanDistinctValuesRoutesByLeftSchema(t *testing.T) {
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	leftSchema.AddVarcharField("shared", 20)
	rightSchema.AddIntegerField("course_id")
	rightSchema.AddIntegerField("shared")
	left := &productPlanProbe{
		schema:   &leftSchema,
		distinct: map[string]int{"student_id": 8, "shared": 3},
	}
	right := &productPlanProbe{
		schema:   &rightSchema,
		distinct: map[string]int{"course_id": 5, "missing": 11, "shared": 99},
	}
	join := NewIndexJoinPlan(left, right, nil, "student_id")
	for fieldName, want := range map[string]int{"student_id": 8, "course_id": 5, "shared": 3, "missing": 11} {
		if got := join.DistinctValues(fieldName); got != want {
			t.Errorf("DistinctValues(%q) = %d, want %d", fieldName, got, want)
		}
	}
}

func TestIndexJoinPlanDistinctValuesHandlesNilPlansAndSchemas(t *testing.T) {
	var nilPlan *IndexJoinPlan
	if got := nilPlan.DistinctValues("student_id"); got != 0 {
		t.Fatalf("nil plan DistinctValues() = %d, want zero", got)
	}
	if got := NewIndexJoinPlan(nil, nil, nil, "student_id").DistinctValues("student_id"); got != 0 {
		t.Fatalf("plan without children DistinctValues() = %d, want zero", got)
	}
	if got := NewIndexJoinPlan(nil, &productPlanProbe{distinct: map[string]int{"student_id": 4}}, nil, "student_id").DistinctValues("student_id"); got != 0 {
		t.Fatalf("plan without left child DistinctValues() = %d, want zero", got)
	}

	leftWithoutSchema := &productPlanProbe{distinct: map[string]int{"student_id": 2}}
	right := &productPlanProbe{distinct: map[string]int{"student_id": 6}}
	if got := NewIndexJoinPlan(leftWithoutSchema, right, nil, "student_id").DistinctValues("student_id"); got != 6 {
		t.Fatalf("DistinctValues() with missing left schema = %d, want right estimate 6", got)
	}

	var leftSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	left := &productPlanProbe{schema: &leftSchema, distinct: map[string]int{"student_id": 7}}
	if got := NewIndexJoinPlan(left, nil, nil, "student_id").DistinctValues("student_id"); got != 7 {
		t.Fatalf("left-only DistinctValues() = %d, want 7", got)
	}
	if got := NewIndexJoinPlan(left, nil, nil, "student_id").DistinctValues("other"); got != 0 {
		t.Fatalf("unknown-field DistinctValues() without right plan = %d, want zero", got)
	}
}
