package transaction

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

func TestNewInitializesIsolatedTransactionLocalState(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	buffers := &recordingBufferPinner{}

	first := New(&allocator, &table, buffers)
	second := New(&allocator, &table, buffers)

	if got := first.ID(); got != recovery.TransactionID(1) {
		t.Fatalf("first ID = %d, want 1", got)
	}
	if got := second.ID(); got != recovery.TransactionID(2) {
		t.Fatalf("second ID = %d, want 2", got)
	}
	if first.locks == second.locks {
		t.Fatal("transactions share local concurrency manager")
	}
	if first.buffers == second.buffers {
		t.Fatal("transactions share local buffer ownership")
	}
	if len(first.buffers.pins) != 0 || len(second.buffers.pins) != 0 {
		t.Fatalf("new transaction pins = (%v, %v), want empty", first.buffers.pins, second.buffers.pins)
	}
}
