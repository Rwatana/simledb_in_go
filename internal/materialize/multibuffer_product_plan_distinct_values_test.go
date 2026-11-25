package materialize

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestMultibufferProductPlanDistinctValuesDelegatesToOwningChild(t *testing.T) {
	leftSchema := schemaWithIntegerFields("left_id", "shared")
	rightSchema := schemaWithIntegerFields("right_id", "shared")
	tests := []struct {
		name           string
		field          string
		leftDistinct   map[string]int
		rightDistinct  map[string]int
		want           int
		wantLeftCalls  []string
		wantRightCalls []string
	}{
		{
			name:           "left field",
			field:          "left_id",
			leftDistinct:   map[string]int{"left_id": 13},
			rightDistinct:  map[string]int{"left_id": 99},
			want:           13,
			wantLeftCalls:  []string{"left_id"},
			wantRightCalls: nil,
		},
		{
			name:           "right field",
			field:          "right_id",
			leftDistinct:   map[string]int{"right_id": 99},
			rightDistinct:  map[string]int{"right_id": 17},
			want:           17,
			wantLeftCalls:  nil,
			wantRightCalls: []string{"right_id"},
		},
		{
			name:           "left field wins duplicate name",
			field:          "shared",
			leftDistinct:   map[string]int{"shared": 19},
			rightDistinct:  map[string]int{"shared": 23},
			want:           19,
			wantLeftCalls:  []string{"shared"},
			wantRightCalls: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left := &materializePlan{schema: leftSchema, distinct: test.leftDistinct}
			right := &materializePlan{schema: rightSchema, distinct: test.rightDistinct}
			transaction := newMaterializePlanTransaction()
			product := NewMultibufferProductPlan(transaction, left, right)

			if got := product.DistinctValues(test.field); got != test.want {
				t.Fatalf("DistinctValues(%q) = %d, want %d", test.field, got, test.want)
			}
			if !reflect.DeepEqual(left.distinctCalls, test.wantLeftCalls) || !reflect.DeepEqual(right.distinctCalls, test.wantRightCalls) {
				t.Errorf("distinct calls = %v/%v, want %v/%v", left.distinctCalls, right.distinctCalls, test.wantLeftCalls, test.wantRightCalls)
			}
			if left.openCalls != 0 || right.openCalls != 0 || transaction.calls != 0 || transaction.availableCalls != 0 {
				t.Errorf("DistinctValues performed scan/storage IO: open=%d/%d transaction pin/buffer=%d/%d", left.openCalls, right.openCalls, transaction.calls, transaction.availableCalls)
			}
			if left.schemaCalls != 1 || right.schemaCalls != 1 {
				t.Errorf("schema calls = %d/%d, want one read from each child", left.schemaCalls, right.schemaCalls)
			}
		})
	}
}

func TestMultibufferProductPlanDistinctValuesReturnsZeroWhenUnavailable(t *testing.T) {
	var nilPlan *materializePlan
	validSchema := schemaWithIntegerFields("known")
	left := &materializePlan{schema: validSchema}
	right := &materializePlan{schema: validSchema}
	tests := []struct {
		name    string
		product *MultibufferProductPlan
	}{
		{name: "nil receiver"},
		{name: "nil transaction", product: NewMultibufferProductPlan(nil, left, right)},
		{name: "typed nil transaction", product: NewMultibufferProductPlan((*materializePlanTransaction)(nil), left, right)},
		{name: "nil left", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), nil, right)},
		{name: "typed nil left", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), nilPlan, right)},
		{name: "nil right", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), left, nil)},
		{name: "typed nil right", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), left, nilPlan)},
		{name: "missing left schema", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{}, right)},
		{name: "missing right schema", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), left, &materializePlan{})},
		{name: "unknown field", product: NewMultibufferProductPlan(newMaterializePlanTransaction(), left, right)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.product.DistinctValues("unknown"); got != 0 {
				t.Fatalf("DistinctValues(%q) = %d, want 0", "unknown", got)
			}
		})
	}
}

func schemaWithIntegerFields(fieldNames ...string) *record.Schema {
	schema := &record.Schema{}
	for _, fieldName := range fieldNames {
		schema.AddIntegerField(fieldName)
	}
	return schema
}
