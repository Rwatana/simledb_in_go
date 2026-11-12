package recovery

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestSetIntRecordRoundTrip(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetIntRecord(TransactionID(7), block, 16, -5)
	if err != nil {
		t.Fatalf("EncodeSetIntRecord() error = %v", err)
	}
	want := []byte{
		0, 0, 0, 4,
		0, 0, 0, 7,
		0, 0, 0, 3, 't', 'b', 'l',
		0, 0, 0, 2,
		0, 0, 0, 16,
		255, 255, 255, 251,
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded = %v, want %v", encoded, want)
	}

	record, err := DecodeSetIntRecord(encoded)
	if err != nil {
		t.Fatalf("DecodeSetIntRecord() error = %v", err)
	}
	if record.Operation() != OperationSetInt || record.TransactionID() != 7 || record.Block() != block || record.Offset() != 16 || record.OldValue() != -5 {
		t.Fatalf("decoded = operation %d transaction %d block %v offset %d old %d", record.Operation(), record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestSetIntRecordRoundTripsUTF8FilenameAndSignedFields(t *testing.T) {
	block := storage.NewBlockID("表.tbl", -2)
	encoded, err := EncodeSetIntRecord(TransactionID(-1), block, -8, math.MinInt32)
	if err != nil {
		t.Fatalf("EncodeSetIntRecord() error = %v", err)
	}
	record, err := DecodeSetIntRecord(encoded)
	if err != nil {
		t.Fatalf("DecodeSetIntRecord() error = %v", err)
	}
	if record.TransactionID() != -1 || record.Block() != block || record.Offset() != -8 || record.OldValue() != math.MinInt32 {
		t.Fatalf("decoded signed fields = transaction %d block %v offset %d old %d", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestEncodeSetIntRecordRejectsBlockOutsideInt32(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent a value outside int32")
	}
	_, err := EncodeSetIntRecord(1, storage.NewBlockID("tbl", int(int64(math.MaxInt32)+1)), 0, 0)
	if err == nil || !strings.Contains(err.Error(), "block number") {
		t.Fatalf("EncodeSetIntRecord() error = %v, want block number context", err)
	}
}

func TestDecodeSetIntRecordRejectsInvalidEncoding(t *testing.T) {
	valid, err := EncodeSetIntRecord(7, storage.NewBlockID("tbl", 2), 16, -5)
	if err != nil {
		t.Fatal(err)
	}
	wrongOperation := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(wrongOperation[:4], uint32(OperationRollback))
	negativeName := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(negativeName[8:12], math.MaxUint32)

	tests := []struct {
		name    string
		encoded []byte
		wantErr string
	}{
		{name: "too short", encoded: valid[:23], wantErr: "at least 24 bytes"},
		{name: "wrong operation", encoded: wrongOperation, wantErr: "operation 3"},
		{name: "negative filename length", encoded: negativeName, wantErr: "negative filename length"},
		{name: "truncated filename or fields", encoded: valid[:len(valid)-1], wantErr: "record length"},
		{name: "trailing byte", encoded: append(append([]byte(nil), valid...), 0), wantErr: "record length"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeSetIntRecord(test.encoded)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("DecodeSetIntRecord() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}
