package recovery

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type recordingAppender struct {
	records [][]byte
	lsn     logmanager.LSN
	err     error
}

func (a *recordingAppender) Append(record []byte) (logmanager.LSN, error) {
	a.records = append(a.records, append([]byte(nil), record...))
	return a.lsn, a.err
}

func TestAppendStartRecordAppendsExactEncodingAndReturnsLSN(t *testing.T) {
	appender := &recordingAppender{lsn: 17}

	lsn, err := AppendStartRecord(appender, TransactionID(42))
	if err != nil {
		t.Fatalf("AppendStartRecord() error = %v", err)
	}
	if lsn != 17 {
		t.Fatalf("AppendStartRecord() LSN = %d, want 17", lsn)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1", len(appender.records))
	}
	want := []byte{0, 0, 0, 1, 0, 0, 0, 42}
	if !bytes.Equal(appender.records[0], want) {
		t.Fatalf("appended record = %v, want %v", appender.records[0], want)
	}
}

func TestAppendStartRecordReturnsAppendFailureWithContext(t *testing.T) {
	wantErr := errors.New("disk unavailable")
	appender := &recordingAppender{err: wantErr}

	_, err := AppendStartRecord(appender, TransactionID(9))
	if !errors.Is(err, wantErr) {
		t.Fatalf("AppendStartRecord() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "transaction 9") {
		t.Fatalf("AppendStartRecord() error = %q, want transaction context", err)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1 without retry", len(appender.records))
	}
}

func TestLogManagerSatisfiesStartRecordAppender(t *testing.T) {
	var _ LogAppender = (*logmanager.Manager)(nil)
}
