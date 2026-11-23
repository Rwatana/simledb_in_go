package recovery

import (
	"errors"
	"strings"
	"testing"
)

func TestRecoverRestartAndAppendWritesCheckpointAfterBufferFlush(t *testing.T) {
	flushed := false
	appender := &orderedRollbackAppender{flushed: &flushed, lsn: 101}
	lsn, err := RecoverRestartAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		flushed = true
		return nil
	}, appender)
	if err != nil {
		t.Fatalf("RecoverRestartAndAppend() error = %v", err)
	}
	if lsn != 101 || appender.calls != 1 || appender.appendedBeforeFlush {
		t.Fatalf("LSN = %d calls = %d appended before flush = %t", lsn, appender.calls, appender.appendedBeforeFlush)
	}
}

func TestRecoverRestartAndAppendDoesNotAppendAfterHistoryFailure(t *testing.T) {
	want := errors.New("iterator failed")
	appender := &orderedRollbackAppender{}
	_, err := RecoverRestartAndAppend(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, appender)
	if !errors.Is(err, want) || appender.calls != 0 {
		t.Fatalf("error = %v append calls = %d", err, appender.calls)
	}
}

func TestRecoverRestartAndAppendDoesNotAppendAfterBufferFlushFailure(t *testing.T) {
	want := errors.New("buffer flush failed")
	appender := &orderedRollbackAppender{}
	_, err := RecoverRestartAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return want }, appender)
	if !errors.Is(err, want) || appender.calls != 0 {
		t.Fatalf("error = %v append calls = %d", err, appender.calls)
	}
}

func TestRecoverRestartAndAppendReturnsCheckpointFailure(t *testing.T) {
	want := errors.New("append failed")
	appender := &orderedRollbackAppender{err: want}
	_, err := RecoverRestartAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeCheckpointRecord()}}, nil
	}, 11, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, appender)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("error = %v", err)
	}
}
