package recovery

import (
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingBufferView struct {
	block    storage.BlockID
	assigned bool
	page     *storage.Page
}

func (v recordingBufferView) Block() (storage.BlockID, bool) { return v.block, v.assigned }
func (v recordingBufferView) Contents() *storage.Page        { return v.page }

func TestLogPreviousIntegerAppendsValueBeforeUpdate(t *testing.T) {
	page, err := storage.NewPage(64)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetInt(12, 99); err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 3)
	view := recordingBufferView{block: block, assigned: true, page: page}
	appender := &recordingAppender{lsn: 71}

	lsn, err := LogPreviousInteger(appender, 8, view, 12)
	if err != nil {
		t.Fatalf("LogPreviousInteger() error = %v", err)
	}
	if lsn != 71 || len(appender.records) != 1 {
		t.Fatalf("LSN = %d append calls = %d", lsn, len(appender.records))
	}
	record, err := DecodeSetIntRecord(appender.records[0])
	if err != nil {
		t.Fatal(err)
	}
	if record.TransactionID() != 8 || record.Block() != block || record.Offset() != 12 || record.OldValue() != 99 {
		t.Fatalf("record = tx %d block %v offset %d old %d", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
}

func TestLogPreviousIntegerRejectsMissingAssignmentBeforeAppend(t *testing.T) {
	page, _ := storage.NewPage(16)
	appender := &recordingAppender{}
	_, err := LogPreviousInteger(appender, 8, recordingBufferView{page: page}, 0)
	if err == nil || !strings.Contains(err.Error(), "not assigned") || len(appender.records) != 0 {
		t.Fatalf("error = %v calls = %d, want assignment error and no append", err, len(appender.records))
	}
}

func TestLogPreviousIntegerRejectsPageReadFailureBeforeAppend(t *testing.T) {
	page, _ := storage.NewPage(4)
	appender := &recordingAppender{}
	_, err := LogPreviousInteger(appender, 8, recordingBufferView{block: storage.NewBlockID("tbl", 1), assigned: true, page: page}, 2)
	if err == nil || !strings.Contains(err.Error(), "offset 2") || len(appender.records) != 0 {
		t.Fatalf("error = %v calls = %d, want read error and no append", err, len(appender.records))
	}
}

func TestBufferSatisfiesIntegerUpdateView(t *testing.T) {
	var _ IntegerUpdateView = (*buffer.Buffer)(nil)
}
