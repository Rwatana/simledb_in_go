package recovery

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAppendRollbackRecordAppendsExactEncodingAndReturnsLSN(t *testing.T) {
	appender := &recordingAppender{lsn: 47}
	lsn, err := AppendRollbackRecord(appender, TransactionID(42))
	if err != nil {
		t.Fatalf("AppendRollbackRecord() error = %v", err)
	}
	if lsn != 47 {
		t.Fatalf("AppendRollbackRecord() LSN = %d, want 47", lsn)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1", len(appender.records))
	}
	want := []byte{0, 0, 0, 3, 0, 0, 0, 42}
	if !bytes.Equal(appender.records[0], want) {
		t.Fatalf("appended record = %v, want %v", appender.records[0], want)
	}
}

func TestAppendRollbackRecordReturnsAppendFailureWithContext(t *testing.T) {
	wantErr := errors.New("disk unavailable")
	appender := &recordingAppender{err: wantErr}
	_, err := AppendRollbackRecord(appender, TransactionID(9))
	if !errors.Is(err, wantErr) {
		t.Fatalf("AppendRollbackRecord() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "rollback record for transaction 9") {
		t.Fatalf("AppendRollbackRecord() error = %q, want rollback context", err)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1 without retry", len(appender.records))
	}
}
