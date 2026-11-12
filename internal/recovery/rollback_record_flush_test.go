package recovery

import (
	"errors"
	"strings"
	"testing"
)

func TestAppendAndFlushRollbackRecordFlushesReturnedLSN(t *testing.T) {
	logs := &recordingDurableLog{recordingAppender: recordingAppender{lsn: 53}}
	lsn, err := AppendAndFlushRollbackRecord(logs, TransactionID(6))
	if err != nil {
		t.Fatalf("AppendAndFlushRollbackRecord() error = %v", err)
	}
	if lsn != 53 || len(logs.flushed) != 1 || logs.flushed[0] != 53 {
		t.Fatalf("result LSN = %d, flushed = %v, want 53 and [53]", lsn, logs.flushed)
	}
}

func TestAppendAndFlushRollbackRecordDoesNotFlushAfterAppendFailure(t *testing.T) {
	wantErr := errors.New("append failed")
	logs := &recordingDurableLog{recordingAppender: recordingAppender{err: wantErr}}
	_, err := AppendAndFlushRollbackRecord(logs, TransactionID(7))
	if !errors.Is(err, wantErr) || len(logs.flushed) != 0 {
		t.Fatalf("error = %v, flushed = %v, want wrapped append error and no flush", err, logs.flushed)
	}
}

func TestAppendAndFlushRollbackRecordReturnsFlushFailureWithContext(t *testing.T) {
	wantErr := errors.New("flush failed")
	logs := &recordingDurableLog{recordingAppender: recordingAppender{lsn: 59}, flushErr: wantErr}
	_, err := AppendAndFlushRollbackRecord(logs, TransactionID(8))
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "transaction 8") || !strings.Contains(err.Error(), "LSN 59") {
		t.Fatalf("error = %q, want transaction and LSN context", err)
	}
	if len(logs.flushed) != 1 {
		t.Fatalf("Flush() calls = %d, want 1 without retry", len(logs.flushed))
	}
}
