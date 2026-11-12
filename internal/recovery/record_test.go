package recovery

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestDecodeRecordDispatchesEveryOperation(t *testing.T) {
	setInt, err := EncodeSetIntRecord(1, storage.NewBlockID("tbl", 2), 8, 3)
	if err != nil {
		t.Fatal(err)
	}
	setString, err := EncodeSetStringRecord(1, storage.NewBlockID("tbl", 2), 8, "old")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		encoded   []byte
		operation Operation
		wantType  string
	}{
		{"checkpoint", EncodeCheckpointRecord(), OperationCheckpoint, "recovery.CheckpointRecord"},
		{"start", EncodeStartRecord(1), OperationStart, "recovery.StartRecord"},
		{"commit", EncodeCommitRecord(1), OperationCommit, "recovery.CommitRecord"},
		{"rollback", EncodeRollbackRecord(1), OperationRollback, "recovery.RollbackRecord"},
		{"setint", setInt, OperationSetInt, "recovery.SetIntRecord"},
		{"setstring", setString, OperationSetString, "recovery.SetStringRecord"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := DecodeRecord(test.encoded)
			if err != nil {
				t.Fatalf("DecodeRecord() error = %v", err)
			}
			if record.Operation() != test.operation || fmt.Sprintf("%T", record) != test.wantType {
				t.Fatalf("record = %T operation %d, want %s operation %d", record, record.Operation(), test.wantType, test.operation)
			}
		})
	}
}

func TestDecodeRecordRejectsShortAndUnknownOperation(t *testing.T) {
	tests := []struct {
		name    string
		encoded []byte
		want    string
	}{
		{"short", []byte{0, 0, 0}, "operation prefix"},
		{"unknown", []byte{0, 0, 0, 99}, "unknown operation 99"},
		{"preserves selected decoder error", []byte{0, 0, 0, 1}, "exactly 8 bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRecord(test.encoded)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
