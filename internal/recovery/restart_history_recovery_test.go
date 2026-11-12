package recovery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestRecoverRestartHistoryCreatesBoundedIteratorAndUndoesUnfinishedUpdates(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	finished, err := EncodeSetIntRecord(7, block, 12, 99)
	if err != nil {
		t.Fatal(err)
	}
	unfinished, err := EncodeSetStringRecord(8, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	older, err := EncodeSetIntRecord(10, block, 20, 4)
	if err != nil {
		t.Fatal(err)
	}
	source := &sliceEncodedRecordIterator{records: [][]byte{
		EncodeCommitRecord(7), finished, unfinished, EncodeCheckpointRecord(), older,
	}}
	factoryCalls := 0
	target := &recordingRollbackUndoTarget{}

	err = RecoverRestartHistory(func() (EncodedRecordIterator, error) {
		factoryCalls++
		return source, nil
	}, target)
	if err != nil {
		t.Fatalf("RecoverRestartHistory() error = %v", err)
	}
	want := []string{"pin:" + block.String(), "setstring:以前:false", "unpin:" + block.String()}
	if factoryCalls != 1 || source.next != 4 || !reflect.DeepEqual(target.events, want) {
		t.Fatalf("factory calls = %d source.next = %d events = %#v", factoryCalls, source.next, target.events)
	}
}

func TestRecoverRestartHistoryStopsAfterIteratorCreationFailure(t *testing.T) {
	want := errors.New("iterator failed")
	target := &recordingRollbackUndoTarget{}
	err := RecoverRestartHistory(func() (EncodedRecordIterator, error) {
		return nil, want
	}, target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restart recovery") || len(target.events) != 0 {
		t.Fatalf("error = %v events = %v", err, target.events)
	}
}

func TestRecoverRestartHistoryReturnsUndoFailure(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetStringRecord(8, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("restore failed")
	err = RecoverRestartHistory(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{encoded, EncodeCheckpointRecord()}}, nil
	}, &recordingRollbackUndoTarget{setStringErr: want})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restart recovery") {
		t.Fatalf("error = %v", err)
	}
}
