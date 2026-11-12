package recovery

import (
	"errors"
	"strings"
	"testing"
)

func TestRecoverRestartFlushesReturnedCheckpointLSN(t *testing.T) {
	logs := &orderedRollbackLog{orderedRollbackAppender: orderedRollbackAppender{lsn: 103}}
	lsn, err := RecoverRestart(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if err != nil {
		t.Fatalf("RecoverRestart() error = %v", err)
	}
	if lsn != 103 || logs.calls != 1 || logs.flushCalls != 1 || logs.flushedLSN != 103 {
		t.Fatalf("LSN = %d append calls = %d flush calls = %d flushed LSN = %d", lsn, logs.calls, logs.flushCalls, logs.flushedLSN)
	}
}

func TestRecoverRestartDoesNotFlushCheckpointAfterEarlierFailure(t *testing.T) {
	want := errors.New("iterator failed")
	logs := &orderedRollbackLog{}
	_, err := RecoverRestart(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || logs.calls != 0 || logs.flushCalls != 0 {
		t.Fatalf("error = %v append calls = %d flush calls = %d", err, logs.calls, logs.flushCalls)
	}
}

func TestRecoverRestartDoesNotFlushCheckpointAfterAppendFailure(t *testing.T) {
	want := errors.New("append failed")
	logs := &orderedRollbackLog{orderedRollbackAppender: orderedRollbackAppender{err: want}}
	_, err := RecoverRestart(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || logs.flushCalls != 0 {
		t.Fatalf("error = %v flush calls = %d", err, logs.flushCalls)
	}
}

func TestRecoverRestartReturnsCheckpointFlushFailure(t *testing.T) {
	want := errors.New("checkpoint flush failed")
	logs := &orderedRollbackLog{
		orderedRollbackAppender: orderedRollbackAppender{lsn: 103},
		flushErr:                want,
	}
	_, err := RecoverRestart(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 11") || !strings.Contains(err.Error(), "LSN 103") {
		t.Fatalf("error = %v", err)
	}
}
