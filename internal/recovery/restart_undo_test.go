package recovery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestUndoRestartRecordsRestoresOnlyUnfinishedUpdatesInSourceOrder(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	source := &sliceRollbackRecordSource{records: []Record{
		CommitRecord{transaction: 7},
		SetIntRecord{transaction: 7, block: block, oldValue: 99},
		SetStringRecord{transaction: 8, block: block, oldValue: "以前"},
		RollbackRecord{transaction: 9},
		SetStringRecord{transaction: 9, block: block, oldValue: "skip"},
		SetIntRecord{transaction: 10, block: block, oldValue: 4},
		StartRecord{transaction: 10},
	}}
	target := &recordingRollbackUndoTarget{}

	if err := UndoRestartRecords(source, target); err != nil {
		t.Fatalf("UndoRestartRecords() error = %v", err)
	}
	want := []string{
		"pin:" + block.String(), "setstring:以前:false", "unpin:" + block.String(),
		"pin:" + block.String(), "setint:4:false", "unpin:" + block.String(),
	}
	if !reflect.DeepEqual(target.events, want) {
		t.Fatalf("events = %#v, want %#v", target.events, want)
	}
}

func TestUndoRestartRecordsReturnsSourceFailure(t *testing.T) {
	want := errors.New("source failed")
	err := UndoRestartRecords(&sliceRollbackRecordSource{err: want}, &recordingRollbackUndoTarget{})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restart recovery") {
		t.Fatalf("error = %v", err)
	}
}

func TestUndoRestartRecordsStopsAfterUnfinishedUndoFailure(t *testing.T) {
	want := errors.New("restore failed")
	block := storage.NewBlockID("tbl", 2)
	source := &sliceRollbackRecordSource{records: []Record{
		SetStringRecord{transaction: 8, block: block, oldValue: "以前"},
		SetIntRecord{transaction: 10, block: block, oldValue: 4},
	}}
	target := &recordingRollbackUndoTarget{setStringErr: want}

	err := UndoRestartRecords(source, target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "SETSTRING") || source.next != 1 {
		t.Fatalf("error = %v source.next = %d", err, source.next)
	}
}
