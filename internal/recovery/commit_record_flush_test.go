package recovery

import (
	"errors"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type recordingDurableLog struct {
	recordingAppender
	flushed  []logmanager.LSN
	flushErr error
}

func (l *recordingDurableLog) Flush(lsn logmanager.LSN) error {
	l.flushed = append(l.flushed, lsn)
	return l.flushErr
}

func TestAppendAndFlushCommitRecordFlushesReturnedLSN(t *testing.T) {
	logs := &recordingDurableLog{recordingAppender: recordingAppender{lsn: 31}}

	lsn, err := AppendAndFlushCommitRecord(logs, TransactionID(6))
	if err != nil {
		t.Fatalf("AppendAndFlushCommitRecord() error = %v", err)
	}
	if lsn != 31 {
		t.Fatalf("AppendAndFlushCommitRecord() LSN = %d, want 31", lsn)
	}
	if len(logs.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1", len(logs.records))
	}
	if len(logs.flushed) != 1 || logs.flushed[0] != 31 {
		t.Fatalf("Flush() LSNs = %v, want [31]", logs.flushed)
	}
}

func TestAppendAndFlushCommitRecordDoesNotFlushAfterAppendFailure(t *testing.T) {
	wantErr := errors.New("append failed")
	logs := &recordingDurableLog{recordingAppender: recordingAppender{err: wantErr}}

	_, err := AppendAndFlushCommitRecord(logs, TransactionID(7))
	if !errors.Is(err, wantErr) {
		t.Fatalf("AppendAndFlushCommitRecord() error = %v, want wrapped %v", err, wantErr)
	}
	if len(logs.flushed) != 0 {
		t.Fatalf("Flush() calls = %d, want 0", len(logs.flushed))
	}
}

func TestAppendAndFlushCommitRecordReturnsFlushFailureWithContext(t *testing.T) {
	wantErr := errors.New("flush failed")
	logs := &recordingDurableLog{
		recordingAppender: recordingAppender{lsn: 41},
		flushErr:          wantErr,
	}

	_, err := AppendAndFlushCommitRecord(logs, TransactionID(8))
	if !errors.Is(err, wantErr) {
		t.Fatalf("AppendAndFlushCommitRecord() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "transaction 8") || !strings.Contains(err.Error(), "LSN 41") {
		t.Fatalf("AppendAndFlushCommitRecord() error = %q, want transaction and LSN context", err)
	}
	if len(logs.flushed) != 1 {
		t.Fatalf("Flush() calls = %d, want 1 without retry", len(logs.flushed))
	}
}

func TestLogManagerSatisfiesDurableRecoveryLog(t *testing.T) {
	var _ DurableLog = (*logmanager.Manager)(nil)
}
