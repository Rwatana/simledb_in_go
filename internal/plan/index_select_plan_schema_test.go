package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectPlanSchemaReturnsTheChildSchema(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	transaction := &tablePlanTransaction{}
	indexInfo, err := metadata.NewIndexInfo("student_idx", "student_id", &schema, transaction, metadata.NewStatInfo(1, 1))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	searchKey := query.NewIntConstant(6)
	child := &productPlanProbe{schema: &schema}
	indexSelect := NewIndexSelectPlan(child, indexInfo, searchKey)

	if indexSelect.child != child || indexSelect.indexInfo != indexInfo || indexSelect.searchKey != searchKey {
		t.Fatalf("retained plan state = %#v, want supplied child, index metadata, and search key", indexSelect)
	}
	if got := indexSelect.Schema(); got != &schema {
		t.Fatalf("Schema() = %p, want unchanged child schema %p", got, &schema)
	}
}

func TestIndexSelectPlanSchemaReturnsNilForNilPlanOrChild(t *testing.T) {
	var indexSelect *IndexSelectPlan
	if got := indexSelect.Schema(); got != nil {
		t.Fatalf("nil plan Schema() = %v, want nil", got)
	}
	if got := NewIndexSelectPlan(nil, nil, query.Constant{}).Schema(); got != nil {
		t.Fatalf("plan without child Schema() = %v, want nil", got)
	}
}
