package recovery

import (
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestLogPreviousStringAppendsValueBeforeUpdate(t *testing.T) {
	page, err := storage.NewPage(64)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetBytes(12, []byte("以前")); err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 3)
	view := recordingBufferView{block: block, assigned: true, page: page}
	appender := &recordingAppender{lsn: 73}

	lsn, err := LogPreviousString(appender, 8, view, 12)
	if err != nil {
		t.Fatalf("LogPreviousString() error = %v", err)
	}
	if lsn != 73 || len(appender.records) != 1 {
		t.Fatalf("LSN = %d append calls = %d", lsn, len(appender.records))
	}
	record, err := DecodeSetStringRecord(appender.records[0])
	if err != nil {
		t.Fatal(err)
	}
	if record.TransactionID() != 8 || record.Block() != block || record.Offset() != 12 || record.OldValue() != "以前" {
		t.Fatalf("record = tx %d block %v offset %d old %q", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestLogPreviousStringRejectsMissingAssignmentBeforeAppend(t *testing.T) {
	page, _ := storage.NewPage(16)
	appender := &recordingAppender{}
	_, err := LogPreviousString(appender, 8, recordingBufferView{page: page}, 0)
	if err == nil || !strings.Contains(err.Error(), "not assigned") || len(appender.records) != 0 {
		t.Fatalf("error = %v calls = %d, want assignment error and no append", err, len(appender.records))
	}
}

func TestLogPreviousStringRejectsPageReadFailureBeforeAppend(t *testing.T) {
	page, _ := storage.NewPage(4)
	appender := &recordingAppender{}
	_, err := LogPreviousString(appender, 8, recordingBufferView{block: storage.NewBlockID("tbl", 1), assigned: true, page: page}, 2)
	if err == nil || !strings.Contains(err.Error(), "offset 2") || len(appender.records) != 0 {
		t.Fatalf("error = %v calls = %d, want read error and no append", err, len(appender.records))
	}
}
