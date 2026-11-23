package record

import "testing"

func TestRIDBlockNumberReturnsConstructedValue(t *testing.T) {
	rid := NewRID(7, 3)
	if got := rid.BlockNumber(); got != 7 {
		t.Fatalf("BlockNumber() = %d, want 7", got)
	}
}

func TestRIDSlotReturnsConstructedValue(t *testing.T) {
	rid := NewRID(7, 3)
	if got := rid.Slot(); got != 3 {
		t.Fatalf("Slot() = %d, want 3", got)
	}
}
