package materialize

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestMultibufferProductPlanBlocksAccessedCountsMaterializedRightAndChunkPasses(t *testing.T) {
	tests := []struct {
		name       string
		rightRows  int
		leftBlocks int
		want       int
	}{
		{name: "whole chunks", rightRows: 10, leftBlocks: 3, want: 25},
		{name: "final partial chunk", rightRows: 5, leftBlocks: 2, want: 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			product, transaction, left, right := multibufferProductPlanForBlocksAccessed(t, test.rightRows, test.leftBlocks)

			if got := product.BlocksAccessed(); got != test.want {
				t.Fatalf("BlocksAccessed() = %d, want %d", got, test.want)
			}
			if left.blocksCalls != 1 || right.blocksCalls != 0 {
				t.Errorf("child BlocksAccessed calls = %d/%d, want left once and right never", left.blocksCalls, right.blocksCalls)
			}
			if left.openCalls != 0 || right.openCalls != 0 || left.schemaCalls != 0 || right.schemaCalls != 1 {
				t.Errorf("child calls open=%d/%d schema=%d/%d, want only right schema for materialized-size estimate", left.openCalls, right.openCalls, left.schemaCalls, right.schemaCalls)
			}
			if transaction.availableCalls != 1 || transaction.calls != 1 || transaction.pins != 0 || transaction.unpins != 0 || transaction.sizeCalls != 0 || len(transaction.fileSizes) != 0 {
				t.Errorf("transaction calls available=%d block=%d pin/unpin=%d/%d size=%d files=%d, want one block-size and one buffer read without storage IO", transaction.availableCalls, transaction.calls, transaction.pins, transaction.unpins, transaction.sizeCalls, len(transaction.fileSizes))
			}
		})
	}
}

func TestMultibufferProductPlanBlocksAccessedUsesSigned32BitArithmetic(t *testing.T) {
	product, _, _, _ := multibufferProductPlanForBlocksAccessed(t, 10, 500_000_000)
	rightBlocks, leftBlocks, chunks := 10, 500_000_000, 5
	want := int(int32(rightBlocks) + int32(leftBlocks)*int32(chunks))
	if got := product.BlocksAccessed(); got != want {
		t.Fatalf("BlocksAccessed() = %d, want Java-compatible int32 estimate %d", got, want)
	}
}

func TestMultibufferProductPlanBlocksAccessedReturnsZeroWhenUnavailable(t *testing.T) {
	if got := (*MultibufferProductPlan)(nil).BlocksAccessed(); got != 0 {
		t.Errorf("nil receiver BlocksAccessed() = %d, want 0", got)
	}
	if got := NewMultibufferProductPlan(nil, &materializePlan{}, &materializePlan{}).BlocksAccessed(); got != 0 {
		t.Errorf("nil transaction BlocksAccessed() = %d, want 0", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), nil, &materializePlan{}).BlocksAccessed(); got != 0 {
		t.Errorf("nil left BlocksAccessed() = %d, want 0", got)
	}
	if got := NewMultibufferProductPlan(newMaterializePlanTransaction(), &materializePlan{}, nil).BlocksAccessed(); got != 0 {
		t.Errorf("nil right BlocksAccessed() = %d, want 0", got)
	}

	schema := &record.Schema{}
	schema.AddIntegerField("id")
	for _, test := range []struct {
		name        string
		transaction *materializePlanTransaction
		right       *materializePlan
	}{
		{name: "empty right result", transaction: newMaterializePlanTransaction(), right: &materializePlan{schema: schema}},
		{name: "unavailable block size", transaction: &materializePlanTransaction{blockSizeErr: errors.New("block size unavailable")}, right: &materializePlan{schema: schema, records: 2}},
		{name: "missing right schema", transaction: newMaterializePlanTransaction(), right: &materializePlan{records: 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			left := &materializePlan{blocks: 4}
			product := NewMultibufferProductPlan(test.transaction, left, test.right)
			if got := product.BlocksAccessed(); got != 0 {
				t.Fatalf("BlocksAccessed() = %d, want 0", got)
			}
			if test.transaction.availableCalls != 0 {
				t.Errorf("available buffer calls = %d, want 0 for unavailable right size", test.transaction.availableCalls)
			}
		})
	}
}

func multibufferProductPlanForBlocksAccessed(t *testing.T, rightRows, leftBlocks int) (*MultibufferProductPlan, *materializePlanTransaction, *materializePlan, *materializePlan) {
	t.Helper()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	transaction := newMaterializePlanTransaction()
	transaction.blockSize = record.NewLayout(schema).SlotSize()
	transaction.availableBuffers = 4
	left := &materializePlan{blocks: leftBlocks}
	right := &materializePlan{schema: schema, records: rightRows}
	return NewMultibufferProductPlan(transaction, left, right), transaction, left, right
}
