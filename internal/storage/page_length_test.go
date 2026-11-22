package storage

import (
	"math"
	"strings"
	"testing"
)

func TestValidateByteSequenceLengthRejectsUnencodableLength(t *testing.T) {
	if err := validateByteSequenceLength(12, int64(math.MaxInt32)+1); err == nil {
		t.Fatal("validateByteSequenceLength() error = nil, want signed-prefix overflow")
	} else if !strings.Contains(err.Error(), "length 2147483648 exceeds four-byte signed prefix") {
		t.Errorf("validateByteSequenceLength() error = %q, want signed-prefix limit context", err)
	}
}

func TestValidateByteSequenceLengthAcceptsLargestEncodableLength(t *testing.T) {
	if err := validateByteSequenceLength(12, math.MaxInt32); err != nil {
		t.Errorf("validateByteSequenceLength() error = %v, want nil at the signed-prefix limit", err)
	}
}

func TestPageSetBytesRejectsUnencodableLengthWithoutAllocation(t *testing.T) {
	page, err := NewPage(4)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	err = page.setBytesWithLength(12, nil, int64(math.MaxInt32)+1)
	if err == nil || !strings.Contains(err.Error(), "length 2147483648 exceeds four-byte signed prefix") {
		t.Fatalf("setBytesWithLength() error = %v, want signed-prefix overflow", err)
	}
}

func TestPageSetBytesRejectsMismatchedEncodedLength(t *testing.T) {
	page, err := NewPage(8)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	err = page.setBytesWithLength(0, []byte{1}, 2)
	if err == nil || !strings.Contains(err.Error(), "encoded length 2 does not match payload length 1") {
		t.Fatalf("setBytesWithLength() error = %v, want payload-length mismatch", err)
	}
}
