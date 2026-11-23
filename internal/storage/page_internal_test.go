package storage

import (
	"math"
	"testing"
)

func TestEncodedBytesLengthRejectsValuesOutsideInt32(t *testing.T) {
	if _, err := encodedBytesLength(7, int64(math.MaxInt32)+1); err == nil {
		t.Fatal("encodedBytesLength() error = nil, want an oversized-value error")
	}
}

func TestEncodedBytesLengthAcceptsMaximumInt32(t *testing.T) {
	got, err := encodedBytesLength(7, math.MaxInt32)
	if err != nil {
		t.Fatalf("encodedBytesLength() error = %v", err)
	}
	if want := uint32(math.MaxInt32); got != want {
		t.Errorf("encodedBytesLength() = %d, want %d", got, want)
	}
}

func TestPageRejectsOversizedBytesWithoutAllocatingThem(t *testing.T) {
	page, err := NewPage(8)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if err := page.setBytes(0, nil, int64(math.MaxInt32)+1); err == nil {
		t.Fatal("setBytes() error = nil, want an oversized-value error")
	}
}
