package recovery

import (
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestRollbackRecordIteratorStopsPermanentlyAtMatchingStart(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	newest, err := EncodeSetStringRecord(7, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	older, err := EncodeSetIntRecord(7, block, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	source := &sliceEncodedRecordIterator{records: [][]byte{
		newest,
		EncodeStartRecord(8),
		EncodeStartRecord(7),
		older,
	}}
	iterator := NewRollbackRecordIterator(source, 7)

	record, ok, err := iterator.Next()
	if err != nil || !ok || record.Operation() != OperationSetString {
		t.Fatalf("first Next() = (%T, %t, %v)", record, ok, err)
	}
	for call := 0; call < 2; call++ {
		record, ok, err = iterator.Next()
		if err != nil || ok || record != nil {
			t.Fatalf("Next() after START call %d = (%T, %t, %v)", call, record, ok, err)
		}
	}
	if source.next != 3 {
		t.Fatalf("source consumed %d records, want 3 without reading older history", source.next)
	}
}

func TestRollbackRecordIteratorRejectsExhaustionBeforeStart(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetIntRecord(7, block, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	iterator := NewRollbackRecordIterator(&sliceEncodedRecordIterator{records: [][]byte{encoded}}, 7)
	if _, ok, err := iterator.Next(); err != nil || !ok {
		t.Fatalf("first Next() = (_, %t, %v)", ok, err)
	}
	_, _, err = iterator.Next()
	if err == nil || !strings.Contains(err.Error(), "START") || !strings.Contains(err.Error(), "transaction 7") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRecordIteratorReturnsSelectionFailure(t *testing.T) {
	iterator := NewRollbackRecordIterator(&sliceEncodedRecordIterator{records: [][]byte{{0}}}, 7)
	_, _, err := iterator.Next()
	if err == nil || !strings.Contains(err.Error(), "select") || !strings.Contains(err.Error(), "rollback transaction 7") {
		t.Fatalf("error = %v", err)
	}
}
