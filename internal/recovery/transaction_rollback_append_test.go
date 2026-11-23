package recovery

import (
	"errors"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type orderedRollbackAppender struct {
	flushed             *bool
	appendedBeforeFlush bool
	calls               int
	lsn                 logmanager.LSN
	err                 error
}

func (a *orderedRollbackAppender) Append([]byte) (logmanager.LSN, error) {
	a.calls++
	if a.flushed != nil && !*a.flushed {
		a.appendedBeforeFlush = true
	}
	return a.lsn, a.err
}

func TestRollbackTransactionAndAppendWritesMarkerAfterBufferFlush(t *testing.T) {
	flushed := false
	appender := &orderedRollbackAppender{flushed: &flushed, lsn: 83}
	lsn, err := RollbackTransactionAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error {
		flushed = true
		return nil
	}, appender)
	if err != nil {
		t.Fatalf("RollbackTransactionAndAppend() error = %v", err)
	}
	if lsn != 83 || appender.calls != 1 || appender.appendedBeforeFlush {
		t.Fatalf("LSN = %d calls = %d appended before flush = %t", lsn, appender.calls, appender.appendedBeforeFlush)
	}
}

func TestRollbackTransactionAndAppendDoesNotAppendAfterHistoryFailure(t *testing.T) {
	want := errors.New("iterator failed")
	appender := &orderedRollbackAppender{}
	_, err := RollbackTransactionAndAppend(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, appender)
	if !errors.Is(err, want) || appender.calls != 0 {
		t.Fatalf("error = %v append calls = %d", err, appender.calls)
	}
}

func TestRollbackTransactionAndAppendDoesNotAppendAfterBufferFlushFailure(t *testing.T) {
	want := errors.New("buffer flush failed")
	appender := &orderedRollbackAppender{}
	_, err := RollbackTransactionAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return want }, appender)
	if !errors.Is(err, want) || appender.calls != 0 {
		t.Fatalf("error = %v append calls = %d", err, appender.calls)
	}
}

func TestRollbackTransactionAndAppendReturnsAppendFailure(t *testing.T) {
	want := errors.New("append failed")
	appender := &orderedRollbackAppender{err: want}
	_, err := RollbackTransactionAndAppend(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, appender)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "rollback") || !strings.Contains(err.Error(), "transaction 7") {
		t.Fatalf("error = %v", err)
	}
}
