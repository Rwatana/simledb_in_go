package recovery

import (
	"errors"
	"strings"
	"testing"
)

func TestRestartRecordIteratorStopsPermanentlyAtCheckpoint(t *testing.T) {
	source := &sliceEncodedRecordIterator{records: [][]byte{
		EncodeCommitRecord(7),
		EncodeCheckpointRecord(),
		EncodeStartRecord(8),
	}}
	iterator := NewRestartRecordIterator(source)

	record, ok, err := iterator.Next()
	if err != nil || !ok || record.Operation() != OperationCommit {
		t.Fatalf("first Next() = (%T, %t, %v)", record, ok, err)
	}
	for call := 0; call < 2; call++ {
		record, ok, err = iterator.Next()
		if err != nil || ok || record != nil {
			t.Fatalf("Next() after CHECKPOINT call %d = (%T, %t, %v)", call, record, ok, err)
		}
	}
	if source.next != 2 {
		t.Fatalf("source consumed %d records, want 2 without reading older history", source.next)
	}
}

func TestRestartRecordIteratorAllowsEndOfLogBeforeCheckpoint(t *testing.T) {
	iterator := NewRestartRecordIterator(&sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}})
	if _, ok, err := iterator.Next(); err != nil || !ok {
		t.Fatalf("first Next() = (_, %t, %v)", ok, err)
	}
	if record, ok, err := iterator.Next(); err != nil || ok || record != nil {
		t.Fatalf("exhausted Next() = (%T, %t, %v)", record, ok, err)
	}
}

func TestRestartRecordIteratorReturnsIteratorAndDecodeFailures(t *testing.T) {
	want := errors.New("next failed")
	iterator := NewRestartRecordIterator(&sliceEncodedRecordIterator{err: want})
	if _, _, err := iterator.Next(); !errors.Is(err, want) || !strings.Contains(err.Error(), "restart recovery") {
		t.Fatalf("iterator error = %v", err)
	}

	iterator = NewRestartRecordIterator(&sliceEncodedRecordIterator{records: [][]byte{{0}}})
	if _, _, err := iterator.Next(); err == nil || !strings.Contains(err.Error(), "decode") || !strings.Contains(err.Error(), "restart recovery") {
		t.Fatalf("decode error = %v", err)
	}
}
