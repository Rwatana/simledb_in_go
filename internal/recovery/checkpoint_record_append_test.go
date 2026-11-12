package recovery

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAppendCheckpointRecordAppendsExactEncodingAndReturnsLSN(t *testing.T) {
	appender := &recordingAppender{lsn: 97}

	lsn, err := AppendCheckpointRecord(appender)
	if err != nil {
		t.Fatalf("AppendCheckpointRecord() error = %v", err)
	}
	if lsn != 97 || len(appender.records) != 1 {
		t.Fatalf("LSN = %d append calls = %d", lsn, len(appender.records))
	}
	if want := []byte{0, 0, 0, 0}; !bytes.Equal(appender.records[0], want) {
		t.Fatalf("appended record = %v, want %v", appender.records[0], want)
	}
}

func TestAppendCheckpointRecordReturnsAppendFailureWithContext(t *testing.T) {
	want := errors.New("disk unavailable")
	appender := &recordingAppender{err: want}

	_, err := AppendCheckpointRecord(appender)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "checkpoint") || len(appender.records) != 1 {
		t.Fatalf("error = %v append calls = %d", err, len(appender.records))
	}
}
