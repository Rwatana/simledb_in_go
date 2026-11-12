package recovery

import (
	"errors"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type orderedRollbackLog struct {
	orderedRollbackAppender
	flushCalls int
	flushedLSN logmanager.LSN
	flushErr   error
}

func (l *orderedRollbackLog) Flush(lsn logmanager.LSN) error {
	l.flushCalls++
	l.flushedLSN = lsn
	return l.flushErr
}

func TestRollbackTransactionFlushesReturnedMarkerLSN(t *testing.T) {
	logs := &orderedRollbackLog{orderedRollbackAppender: orderedRollbackAppender{lsn: 89}}
	lsn, err := RollbackTransaction(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if err != nil {
		t.Fatalf("RollbackTransaction() error = %v", err)
	}
	if lsn != 89 || logs.calls != 1 || logs.flushCalls != 1 || logs.flushedLSN != 89 {
		t.Fatalf("LSN = %d append calls = %d flush calls = %d flushed LSN = %d", lsn, logs.calls, logs.flushCalls, logs.flushedLSN)
	}
}

func TestRollbackTransactionDoesNotFlushAfterEarlierFailure(t *testing.T) {
	want := errors.New("iterator failed")
	logs := &orderedRollbackLog{}
	_, err := RollbackTransaction(func() (EncodedRecordIterator, error) {
		return nil, want
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || logs.calls != 0 || logs.flushCalls != 0 {
		t.Fatalf("error = %v append calls = %d flush calls = %d", err, logs.calls, logs.flushCalls)
	}
}

func TestRollbackTransactionDoesNotFlushAfterAppendFailure(t *testing.T) {
	want := errors.New("append failed")
	logs := &orderedRollbackLog{orderedRollbackAppender: orderedRollbackAppender{err: want}}
	_, err := RollbackTransaction(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || logs.flushCalls != 0 {
		t.Fatalf("error = %v flush calls = %d", err, logs.flushCalls)
	}
}

func TestRollbackTransactionReturnsMarkerFlushFailure(t *testing.T) {
	want := errors.New("marker flush failed")
	logs := &orderedRollbackLog{
		orderedRollbackAppender: orderedRollbackAppender{lsn: 89},
		flushErr:                want,
	}
	_, err := RollbackTransaction(func() (EncodedRecordIterator, error) {
		return &sliceEncodedRecordIterator{records: [][]byte{EncodeStartRecord(7)}}, nil
	}, 7, &recordingRollbackUndoTarget{}, func(TransactionID) error { return nil }, logs)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 7") || !strings.Contains(err.Error(), "LSN 89") {
		t.Fatalf("error = %v", err)
	}
}
