package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectPlanDistinctValuesUsesIndexMetadata(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	schema.AddVarcharField("name", 20)
	info, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, &tablePlanTransaction{}, metadata.NewStatInfo(3, 12))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	child := &productPlanProbe{distinct: map[string]int{"student_id": 99, "name": 88}}
	plan := NewIndexSelectPlan(child, info, query.NewIntConstant(6))
	for _, test := range []struct {
		field string
		want  int
	}{
		{field: "student_id", want: 1},
		{field: "name", want: 5},
	} {
		if got := plan.DistinctValues(test.field); got != test.want {
			t.Errorf("DistinctValues(%q) = %d, want index metadata estimate %d", test.field, got, test.want)
		}
	}
}

func TestIndexSelectPlanDistinctValuesReturnsZeroWithoutIndexMetadata(t *testing.T) {
	if got := (*IndexSelectPlan)(nil).DistinctValues("student_id"); got != 0 {
		t.Fatalf("nil plan DistinctValues() = %d, want 0", got)
	}
	if got := NewIndexSelectPlan(nil, nil, query.Constant{}).DistinctValues("student_id"); got != 0 {
		t.Fatalf("plan without index metadata DistinctValues() = %d, want 0", got)
	}
}
