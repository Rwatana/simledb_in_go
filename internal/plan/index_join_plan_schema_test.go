package plan

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewIndexJoinPlanSchemaCombinesChildSchemasInOrder(t *testing.T) {
	var leftSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	leftSchema.AddVarcharField("student_name", 24)
	var rightSchema record.Schema
	rightSchema.AddVarcharField("course_title", 48)
	rightSchema.AddIntegerField("course_id")
	left := &productPlanProbe{schema: &leftSchema}
	right := &productPlanProbe{schema: &rightSchema}
	indexInfo, err := metadata.NewIndexInfo("course_idx", "course_id", &rightSchema, &tablePlanTransaction{}, metadata.NewStatInfo(8, 4))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}

	join := NewIndexJoinPlan(left, right, indexInfo, "student_id")
	if join == nil || join.left != left || join.right != right || join.indexInfo != indexInfo || join.joinField != "student_id" {
		t.Fatalf("constructed plan = %#v, want retained children, index metadata, and join field", join)
	}
	schema := join.Schema()
	if schema == nil || schema == &leftSchema || schema == &rightSchema {
		t.Fatalf("Schema() = %p, want a distinct combined schema", schema)
	}
	wantFields := []string{"student_id", "student_name", "course_title", "course_id"}
	if got := schema.Fields(); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, wantFields)
	}
	for field, want := range map[string]record.FieldDefinition{
		"student_id":   {Type: record.IntegerField},
		"student_name": {Type: record.VarcharField, Length: 24},
		"course_title": {Type: record.VarcharField, Length: 48},
		"course_id":    {Type: record.IntegerField},
	} {
		got, ok := schema.Field(field)
		if !ok || got != want {
			t.Errorf("Schema().Field(%q) = (%+v, %t), want (%+v, true)", field, got, ok, want)
		}
	}

	leftSchema.AddIntegerField("later_left_field")
	rightSchema.AddIntegerField("later_right_field")
	if schema.HasField("later_left_field") || schema.HasField("later_right_field") {
		t.Fatal("combined schema changed after child schemas were mutated")
	}
}

func TestIndexJoinPlanSchemaHandlesMissingChildrenAndSchemas(t *testing.T) {
	var nilPlan *IndexJoinPlan
	if schema := nilPlan.Schema(); schema != nil {
		t.Fatalf("nil plan Schema() = %v, want nil", schema)
	}
	if schema := NewIndexJoinPlan(nil, nil, nil, "student_id").Schema(); schema == nil || len(schema.Fields()) != 0 {
		t.Fatalf("empty join plan schema = %v, want an empty non-nil schema", schema)
	}
	if schema := NewIndexJoinPlan(&productPlanProbe{}, &productPlanProbe{}, nil, "student_id").Schema(); schema == nil || len(schema.Fields()) != 0 {
		t.Fatalf("join plan schema with missing child schemas = %v, want empty schema", schema)
	}

	var leftSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	left := &productPlanProbe{schema: &leftSchema}
	if got := NewIndexJoinPlan(left, nil, nil, "student_id").Schema().Fields(); !reflect.DeepEqual(got, []string{"student_id"}) {
		t.Fatalf("left-only join schema = %v, want [student_id]", got)
	}
	var rightSchema record.Schema
	rightSchema.AddIntegerField("course_id")
	right := &productPlanProbe{schema: &rightSchema}
	if got := NewIndexJoinPlan(nil, right, nil, "student_id").Schema().Fields(); !reflect.DeepEqual(got, []string{"course_id"}) {
		t.Fatalf("right-only join schema = %v, want [course_id]", got)
	}
}
