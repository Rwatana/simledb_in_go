package materialize

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestMultibufferProductPlanSchemaCombinesChildFieldsInOrder(t *testing.T) {
	leftSchema := &record.Schema{}
	leftSchema.AddIntegerField("left_id")
	leftSchema.AddVarcharField("left_name", 24)
	rightSchema := &record.Schema{}
	rightSchema.AddVarcharField("right_name", 32)
	rightSchema.AddIntegerField("right_id")
	left := &materializePlan{schema: leftSchema}
	right := &materializePlan{schema: rightSchema}
	transaction := newMaterializePlanTransaction()

	schema := NewMultibufferProductPlan(transaction, left, right).Schema()
	if schema == nil {
		t.Fatal("Schema() = nil, want combined schema")
	}
	wantFields := []string{"left_id", "left_name", "right_name", "right_id"}
	if got := schema.Fields(); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, wantFields)
	}
	if field, ok := schema.Field("left_id"); !ok || field != (record.FieldDefinition{Type: record.IntegerField}) {
		t.Errorf("Schema().Field(left_id) = (%+v, %t), want integer", field, ok)
	}
	if field, ok := schema.Field("left_name"); !ok || field != (record.FieldDefinition{Type: record.VarcharField, Length: 24}) {
		t.Errorf("Schema().Field(left_name) = (%+v, %t), want varchar(24)", field, ok)
	}
	if field, ok := schema.Field("right_name"); !ok || field != (record.FieldDefinition{Type: record.VarcharField, Length: 32}) {
		t.Errorf("Schema().Field(right_name) = (%+v, %t), want varchar(32)", field, ok)
	}
	if left.openCalls != 0 || right.openCalls != 0 || transaction.calls != 0 {
		t.Errorf("Schema() opened plans or accessed transaction: opens=%d/%d calls=%d", left.openCalls, right.openCalls, transaction.calls)
	}
	if got := leftSchema.Fields(); !reflect.DeepEqual(got, []string{"left_id", "left_name"}) {
		t.Errorf("left schema fields changed to %v", got)
	}
	if got := rightSchema.Fields(); !reflect.DeepEqual(got, []string{"right_name", "right_id"}) {
		t.Errorf("right schema fields changed to %v", got)
	}
}

func TestMultibufferProductPlanSchemaReturnsNilForUnavailableChildren(t *testing.T) {
	if got := (*MultibufferProductPlan)(nil).Schema(); got != nil {
		t.Errorf("nil receiver Schema() = %v, want nil", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), nil, &materializePlan{}).Schema(); got != nil {
		t.Errorf("nil left plan Schema() = %v, want nil", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{}, nil).Schema(); got != nil {
		t.Errorf("nil right plan Schema() = %v, want nil", got)
	}
	validSchema := &record.Schema{}
	validSchema.AddIntegerField("id")
	valid := &materializePlan{schema: validSchema}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{}, valid).Schema(); got != nil {
		t.Errorf("nil left schema Schema() = %v, want nil", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), valid, &materializePlan{}).Schema(); got != nil {
		t.Errorf("nil right schema Schema() = %v, want nil", got)
	}
}

func TestNewMultibufferProductPlanFactoryBindsTransactionAndInputs(t *testing.T) {
	leftSchema := &record.Schema{}
	leftSchema.AddIntegerField("department_id")
	rightSchema := &record.Schema{}
	rightSchema.AddIntegerField("student_department_id")
	left := &materializePlan{schema: leftSchema}
	right := &materializePlan{schema: rightSchema}
	transaction := newMaterializePlanTransaction()

	factory := NewMultibufferProductPlanFactory(transaction)
	factoryResult := factory(left, right)
	product, ok := factoryResult.(*MultibufferProductPlan)
	if !ok {
		t.Fatalf("factory result type = %T, want *MultibufferProductPlan", factoryResult)
	}
	if product.transaction != transaction || product.left != left || product.right != right {
		t.Errorf("factory product = (%p, %p, %p), want (%p, %p, %p)",
			product.transaction, product.left, product.right, transaction, left, right)
	}
	if left.openCalls != 0 || right.openCalls != 0 || transaction.calls != 0 {
		t.Errorf("factory opened plans or accessed transaction: opens=%d/%d calls=%d", left.openCalls, right.openCalls, transaction.calls)
	}
}
