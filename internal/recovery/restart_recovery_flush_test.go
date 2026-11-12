package recovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestRecoverRestartChangesFlushesAfterHistoryUndo(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetIntRecord(8, block, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	target := &recordingRollbackUndoTarget{}
	flushCalls := 0
	var flushedTransaction TransactionID
	flush := func(transaction TransactionID) error {
		flushCalls++
		flushedTransaction = transaction
		if len(target.events) != 3 {
			t.Fatalf("flush observed %d undo events, want completed recovery", len(target.events))
		}
		return nil
	}

	err = RecoverRestartChanges(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{encoded, EncodeCheckpointRecord()}}, nil
	}, 11, target, flush)
	if err != nil {
		t.Fatalf("RecoverRestartChanges() error = %v", err)
	}
	if flushCalls != 1 || flushedTransaction != 11 {
		t.Fatalf("flush calls = %d transaction = %d", flushCalls, flushedTransaction)
	}
}

func TestRecoverRestartChangesDoesNotFlushAfterHistoryFailure(t *testing.T) {
	want := errors.New("iterator failed")
	flushCalls := 0
	err := RecoverRestartChanges(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		flushCalls++
		return nil
	})
	if !errors.Is(err, want) || flushCalls != 0 {
		t.Fatalf("error = %v flush calls = %d", err, flushCalls)
	}
}

func TestRecoverRestartChangesReturnsFlushFailure(t *testing.T) {
	want := errors.New("flush failed")
	err := RecoverRestartChanges(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		return want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 11") || !strings.Contains(err.Error(), "flush") {
		t.Fatalf("error = %v", err)
	}
}
