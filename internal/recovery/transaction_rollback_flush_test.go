package recovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestRollbackTransactionChangesFlushesAfterHistoryUndo(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	encoded, err := EncodeSetIntRecord(7, block, 12, 4)
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
			t.Fatalf("flush observed %d undo events, want completed undo", len(target.events))
		}
		return nil
	}

	err = RollbackTransactionChanges(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{encoded, EncodeStartRecord(7)}}, nil
	}, 7, target, flush)
	if err != nil {
		t.Fatalf("RollbackTransactionChanges() error = %v", err)
	}
	if flushCalls != 1 || flushedTransaction != 7 {
		t.Fatalf("flush calls = %d transaction = %d", flushCalls, flushedTransaction)
	}
}

func TestRollbackTransactionChangesDoesNotFlushAfterHistoryFailure(t *testing.T) {
	want := errors.New("iterator failed")
	flushCalls := 0
	err := RollbackTransactionChanges(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		flushCalls++
		return nil
	})
	if !errors.Is(err, want) || flushCalls != 0 {
		t.Fatalf("error = %v flush calls = %d", err, flushCalls)
	}
}

func TestRollbackTransactionChangesReturnsFlushFailure(t *testing.T) {
	want := errors.New("flush failed")
	err := RollbackTransactionChanges(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		return want
	})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 7") || !strings.Contains(err.Error(), "flush") {
		t.Fatalf("error = %v", err)
	}
}
