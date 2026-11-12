package recovery

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestAppendSetStringRecordAppendsExactEncodingAndReturnsLSN(t *testing.T) {
	appender := &recordingAppender{lsn: 67}
	block := storage.NewBlockID("tbl", 2)
	want, err := EncodeSetStringRecord(7, block, 16, "old")
	if err != nil {
		t.Fatal(err)
	}
	lsn, err := AppendSetStringRecord(appender, 7, block, 16, "old")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if lsn != 67 || len(appender.records) != 1 || !bytes.Equal(appender.records[0], want) {
		t.Fatalf("LSN = %d records = %v, want 67 and exact one record", lsn, appender.records)
	}
}

func TestAppendSetStringRecordDoesNotAppendAfterEncodingFailure(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("host int cannot exceed int32")
	}
	appender := &recordingAppender{}
	block := storage.NewBlockID("tbl", int(int64(math.MaxInt32)+1))
	_, err := AppendSetStringRecord(appender, 7, block, 16, "old")
	if err == nil || len(appender.records) != 0 {
		t.Fatalf("error = %v calls = %d, want encoding error and no append", err, len(appender.records))
	}
}

func TestAppendSetStringRecordReturnsAppendFailureWithContext(t *testing.T) {
	wantErr := errors.New("disk unavailable")
	appender := &recordingAppender{err: wantErr}
	block := storage.NewBlockID("tbl", 2)
	_, err := AppendSetStringRecord(appender, 9, block, 16, "old")
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "transaction 9") || !strings.Contains(err.Error(), block.String()) {
		t.Fatalf("error = %v, want wrapped cause with transaction/block context", err)
	}
	if len(appender.records) != 1 {
		t.Fatalf("Append() calls = %d, want 1", len(appender.records))
	}
}
