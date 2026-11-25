package materialize

import "testing"

func TestMultibufferProductPlanRecordsOutputMultipliesChildEstimates(t *testing.T) {
	left := &materializePlan{records: 7}
	right := &materializePlan{records: 11}
	transaction := newMaterializePlanTransaction()

	if got, want := NewMultibufferProductPlan(transaction, left, right).RecordsOutput(), 77; got != want {
		t.Fatalf("RecordsOutput() = %d, want %d", got, want)
	}
	if left.openCalls != 0 || right.openCalls != 0 || left.schemaCalls != 0 || right.schemaCalls != 0 || transaction.calls != 0 {
		t.Errorf("RecordsOutput() performed IO: open=%d/%d schema=%d/%d transaction=%d", left.openCalls, right.openCalls, left.schemaCalls, right.schemaCalls, transaction.calls)
	}
}

func TestMultibufferProductPlanRecordsOutputUsesSigned32BitProduct(t *testing.T) {
	leftRows := 50_000
	rightRows := 50_000
	want := int(int32(leftRows) * int32(rightRows))
	plan := NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{records: leftRows}, &materializePlan{records: rightRows})

	if got := plan.RecordsOutput(); got != want {
		t.Fatalf("RecordsOutput() = %d, want Java-compatible int32 product %d", got, want)
	}
}

func TestMultibufferProductPlanRecordsOutputReturnsZeroForUnavailablePlans(t *testing.T) {
	if got := (*MultibufferProductPlan)(nil).RecordsOutput(); got != 0 {
		t.Errorf("nil receiver RecordsOutput() = %d, want 0", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), nil, &materializePlan{records: 3}).RecordsOutput(); got != 0 {
		t.Errorf("nil left RecordsOutput() = %d, want 0", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{records: 3}, nil).RecordsOutput(); got != 0 {
		t.Errorf("nil right RecordsOutput() = %d, want 0", got)
	}
}
