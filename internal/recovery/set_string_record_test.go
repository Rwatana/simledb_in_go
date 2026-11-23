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

func TestSetStringRecordRoundTrip(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetStringRecord(7, block, 16, "old")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 0, 5, 0, 0, 0, 7, 0, 0, 0, 3, 't', 'b', 'l', 0, 0, 0, 2, 0, 0, 0, 16, 0, 0, 0, 3, 'o', 'l', 'd'}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded = %v, want %v", encoded, want)
	}
	record, err := DecodeSetStringRecord(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if record.Operation() != OperationSetString || record.TransactionID() != 7 || record.Block() != block || record.Offset() != 16 || record.OldValue() != "old" {
		t.Fatalf("decoded = op %d tx %d block %v offset %d old %q", record.Operation(), record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestEncodeSetStringRecordRejectsBlockOutsideInt32(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot represent a value outside int32")
	}
	_, err := EncodeSetStringRecord(1, storage.NewBlockID("tbl", int(int64(math.MaxInt32)+1)), 0, "old")
	if err == nil || !strings.Contains(err.Error(), "block number") {
		t.Fatalf("error = %v, want block number context", err)
	}
}

func TestEncodeSetStringRecordRejectsOversizedLengthComponentsBeforeAllocation(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		oldValue  string
		maxLength int
	}{
		{name: "filename", fileName: "tbl", oldValue: "", maxLength: 2},
		{name: "old value", fileName: "tbl", oldValue: "previous", maxLength: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encodeSetStringRecordWithLimits(7, storage.NewBlockID(test.fileName, 2), 16, test.oldValue, test.maxLength, int(^uint(0)>>1))
			if encoded != nil || err == nil || !strings.Contains(err.Error(), "string length exceeds int32") {
				t.Fatalf("encodeSetStringRecordWithLimits() = (%v, %v), want nil record and length error", encoded, err)
			}
		})
	}
}

func TestEncodeSetStringRecordRejectsCombinedSizeOverflowBeforeAllocation(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		oldValue string
		maxInt   int
	}{
		{name: "filename leaves no room for fixed fields", fileName: "tbl", oldValue: "", maxInt: setStringFixedSize + 2},
		{name: "old value exceeds remaining capacity", fileName: "tbl", oldValue: "old", maxInt: setStringFixedSize + 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encodeSetStringRecordWithLimits(7, storage.NewBlockID(test.fileName, 2), 16, test.oldValue, math.MaxInt32, test.maxInt)
			if encoded != nil || err == nil || !strings.Contains(err.Error(), "combined string lengths overflow record size") {
				t.Fatalf("encodeSetStringRecordWithLimits() = (%v, %v), want nil record and size error", encoded, err)
			}
		})
	}
}

func TestSetStringRecordRoundTripsUTF8AndSignedFields(t *testing.T) {
	block := storage.NewBlockID("表.tbl", -2)
	encoded, err := EncodeSetStringRecord(-1, block, -8, "以前")
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeSetStringRecord(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if record.TransactionID() != -1 || record.Block() != block || record.Offset() != -8 || record.OldValue() != "以前" {
		t.Fatalf("decoded signed/UTF-8 fields = tx %d block %v offset %d old %q", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestDecodeSetStringRecordRejectsInvalidEncoding(t *testing.T) {
	valid, err := EncodeSetStringRecord(7, storage.NewBlockID("tbl", 2), 16, "old")
	if err != nil {
		t.Fatal(err)
	}
	wrongOp := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(wrongOp[:4], uint32(OperationSetInt))
	negativeFile := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(negativeFile[8:12], math.MaxUint32)
	negativeValue := append([]byte(nil), valid...)
	binary.BigEndian.PutUint32(negativeValue[23:27], math.MaxUint32)
	tests := []struct {
		name    string
		encoded []byte
		want    string
	}{
		{"too short", valid[:23], "at least 24 bytes"},
		{"wrong operation", wrongOp, "operation 4"},
		{"negative filename", negativeFile, "negative filename length"},
		{"negative old value", negativeValue, "negative old value length"},
		{"truncated", valid[:len(valid)-1], "record length"},
		{"trailing", append(append([]byte(nil), valid...), 0), "record length"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeSetStringRecord(test.encoded)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestDecodeSetStringRecordRejectsTruncatedFilenameLength(t *testing.T) {
	encoded, err := EncodeSetStringRecord(7, storage.NewBlockID("tbl", 2), 16, "old")
	if err != nil {
		t.Fatal(err)
	}
	missingBytes := len(encoded) - setStringFixedSize + 1
	binary.BigEndian.PutUint32(encoded[8:12], uint32(missingBytes))

	_, err = DecodeSetStringRecord(encoded)
	if err == nil || !strings.Contains(err.Error(), "does not contain filename length") {
		t.Fatalf("DecodeSetStringRecord() error = %v, want truncated filename length context", err)
	}
}
