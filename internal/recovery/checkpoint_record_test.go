package recovery

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckpointRecordRoundTrip(t *testing.T) {
	encoded := EncodeCheckpointRecord()
	want := []byte{0, 0, 0, 0}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("EncodeCheckpointRecord() = %v, want %v", encoded, want)
	}

	record, err := DecodeCheckpointRecord(encoded)
	if err != nil {
		t.Fatalf("DecodeCheckpointRecord() error = %v", err)
	}
	if got := record.Operation(); got != OperationCheckpoint {
		t.Fatalf("Operation() = %d, want %d", got, OperationCheckpoint)
	}
}

func TestDecodeCheckpointRecordRejectsInvalidEncoding(t *testing.T) {
	tests := []struct {
		name    string
		encoded []byte
		wantErr string
	}{
		{name: "truncated", encoded: []byte{0, 0, 0}, wantErr: "exactly 4 bytes"},
		{name: "trailing bytes", encoded: []byte{0, 0, 0, 0, 0}, wantErr: "exactly 4 bytes"},
		{name: "different operation", encoded: []byte{0, 0, 0, 1}, wantErr: "operation 1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeCheckpointRecord(test.encoded)
			if err == nil {
				t.Fatal("DecodeCheckpointRecord() error = nil")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("DecodeCheckpointRecord() error = %q, want substring %q", err, test.wantErr)
			}
		})
	}
}
