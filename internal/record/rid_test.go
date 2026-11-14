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

func TestRIDEqualComparesBothCoordinates(t *testing.T) {
	rid := NewRID(7, 3)
	if !rid.Equal(NewRID(7, 3)) {
		t.Fatal("Equal() = false for matching coordinates")
	}
	if rid.Equal(NewRID(8, 3)) {
		t.Fatal("Equal() = true for different block numbers")
	}
	if rid.Equal(NewRID(7, 4)) {
		t.Fatal("Equal() = true for different slots")
	}
}
