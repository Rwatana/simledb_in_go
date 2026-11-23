package recovery

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAppendCommitRecordAppendsExactEncodingAndReturnsLSN(t *testing.T) {
	appender := &recordingAppender{lsn: 23}

	lsn, err := AppendCommitRecord(appender, TransactionID(42))
	if err != nil {
		t.Fatalf("AppendCommitRecord() error = %v", err)
	}
	if lsn != 23 {
		t.Fatalf("AppendCommitRecord() LSN = %d, want 23", lsn)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1", len(appender.records))
	}
	want := []byte{0, 0, 0, 2, 0, 0, 0, 42}
	if !bytes.Equal(appender.records[0], want) {
		t.Fatalf("appended record = %v, want %v", appender.records[0], want)
	}
}

func TestAppendCommitRecordReturnsAppendFailureWithContext(t *testing.T) {
	wantErr := errors.New("disk unavailable")
	appender := &recordingAppender{err: wantErr}

	_, err := AppendCommitRecord(appender, TransactionID(9))
	if !errors.Is(err, wantErr) {
		t.Fatalf("AppendCommitRecord() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "commit record for transaction 9") {
		t.Fatalf("AppendCommitRecord() error = %q, want commit transaction context", err)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1 without retry", len(appender.records))
	}
}
