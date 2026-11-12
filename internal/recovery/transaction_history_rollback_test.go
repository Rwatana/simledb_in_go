package recovery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestRollbackTransactionHistoryCreatesBoundedIteratorAndUndoesUpdates(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	newest, err := EncodeSetStringRecord(7, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	other, err := EncodeSetIntRecord(8, block, 12, 99)
	if err != nil {
		t.Fatal(err)
	}
	older, err := EncodeSetIntRecord(7, block, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	source := &sliceEncodedRecordIterator{records: [][]byte{newest, other, older, EncodeStartRecord(7)}}
	factoryCalls := 0
	factory := func() (EncodedRecordIterator, error) {
		factoryCalls++
		return source, nil
	}
	target := &recordingRollbackUndoTarget{}

	if err := RollbackTransactionHistory(factory, 7, target); err != nil {
		t.Fatalf("RollbackTransactionHistory() error = %v", err)
	}
	want := []string{
		"pin:" + block.String(), "setstring:以前:false", "unpin:" + block.String(),
		"pin:" + block.String(), "setint:4:false", "unpin:" + block.String(),
	}
	if factoryCalls != 1 || !reflect.DeepEqual(target.events, want) {
		t.Fatalf("factory calls = %d events = %#v, want %#v", factoryCalls, target.events, want)
	}
}

func TestRollbackTransactionHistoryStopsAfterIteratorCreationFailure(t *testing.T) {
	want := errors.New("iterator failed")
	target := &recordingRollbackUndoTarget{}
	err := RollbackTransactionHistory(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 7, target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 7") || len(target.events) != 0 {
		t.Fatalf("error = %v events = %v", err, target.events)
	}
}

func TestRollbackTransactionHistoryReturnsBoundedUndoFailure(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetStringRecord(7, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("restore failed")
	target := &recordingRollbackUndoTarget{setStringErr: want}
	err = RollbackTransactionHistory(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{encoded, EncodeStartRecord(7)}}, nil
	}, 7, target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 7") {
		t.Fatalf("error = %v", err)
	}
}
