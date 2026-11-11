package recovery

import (
	"bytes"
	"strings"
	"testing"
)

func TestRollbackRecordRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		transaction TransactionID
		want        []byte
	}{
		{name: "positive", transaction: 42, want: []byte{0, 0, 0, 3, 0, 0, 0, 42}},
		{name: "zero", transaction: 0, want: []byte{0, 0, 0, 3, 0, 0, 0, 0}},
		{name: "negative bit pattern", transaction: -1, want: []byte{0, 0, 0, 3, 255, 255, 255, 255}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := EncodeRollbackRecord(test.transaction)
			if !bytes.Equal(encoded, test.want) {
				t.Fatalf("EncodeRollbackRecord(%d) = %v, want %v", test.transaction, encoded, test.want)
			}
			record, err := DecodeRollbackRecord(encoded)
			if err != nil {
				t.Fatalf("DecodeRollbackRecord() error = %v", err)
			}
			if got := record.Operation(); got != OperationRollback {
				t.Fatalf("Operation() = %d, want %d", got, OperationRollback)
			}
			if got := record.TransactionID(); got != test.transaction {
				t.Fatalf("TransactionID() = %d, want %d", got, test.transaction)
			}
		})
	}
}

func TestDecodeRollbackRecordRejectsInvalidEncoding(t *testing.T) {
	tests := []struct {
		name    string
		encoded []byte
		wantErr string
	}{
		{name: "truncated", encoded: []byte{0, 0, 0, 3, 0, 0, 0}, wantErr: "exactly 8 bytes"},
		{name: "trailing bytes", encoded: []byte{0, 0, 0, 3, 0, 0, 0, 7, 0}, wantErr: "exactly 8 bytes"},
		{name: "different operation", encoded: []byte{0, 0, 0, 2, 0, 0, 0, 7}, wantErr: "operation 2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRollbackRecord(test.encoded)
			if err == nil {
				t.Fatal("DecodeRollbackRecord() error = nil")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("DecodeRollbackRecord() error = %q, want substring %q", err, test.wantErr)
			}
		})
	}
}
