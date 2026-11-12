package recovery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingStringUndoTarget struct {
	calls     []string
	block     storage.BlockID
	offset    int32
	value     string
	logUpdate bool
	pinErr    error
	setErr    error
	unpinErr  error
}

func (t *recordingStringUndoTarget) Pin(block storage.BlockID) error {
	t.calls = append(t.calls, "pin")
	t.block = block
	return t.pinErr
}

func (t *recordingStringUndoTarget) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	t.calls = append(t.calls, "setstring")
	t.block = block
	t.offset = offset
	t.value = value
	t.logUpdate = logUpdate
	return t.setErr
}

func (t *recordingStringUndoTarget) Unpin(block storage.BlockID) error {
	t.calls = append(t.calls, "unpin")
	t.block = block
	return t.unpinErr
}

func TestSetStringRecordUndoRestoresOldValueWithoutLogging(t *testing.T) {
	block := storage.NewBlockID("tbl", 4)
	record := SetStringRecord{transaction: 7, block: block, offset: 16, oldValue: "以前"}
	target := &recordingStringUndoTarget{}

	if err := record.Undo(target); err != nil {
		t.Fatalf("Undo() error = %v", err)
	}
	if !reflect.DeepEqual(target.calls, []string{"pin", "setstring", "unpin"}) {
		t.Fatalf("calls = %v", target.calls)
	}
	if target.block != block || target.offset != 16 || target.value != "以前" || target.logUpdate {
		t.Fatalf("restore = block %v offset %d value %q log %t", target.block, target.offset, target.value, target.logUpdate)
	}
}

func TestSetStringRecordUndoStopsAfterPinFailure(t *testing.T) {
	want := errors.New("pin failed")
	target := &recordingStringUndoTarget{pinErr: want}
	record := SetStringRecord{transaction: 7, block: storage.NewBlockID("tbl", 4)}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "pin") || !reflect.DeepEqual(target.calls, []string{"pin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}

func TestSetStringRecordUndoUnpinsAfterRestoreFailure(t *testing.T) {
	want := errors.New("restore failed")
	target := &recordingStringUndoTarget{setErr: want}
	record := SetStringRecord{transaction: 7, block: storage.NewBlockID("tbl", 4), offset: 16, oldValue: "以前"}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restore") || !reflect.DeepEqual(target.calls, []string{"pin", "setstring", "unpin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}

func TestSetStringRecordUndoReturnsUnpinFailure(t *testing.T) {
	want := errors.New("unpin failed")
	target := &recordingStringUndoTarget{unpinErr: want}
	record := SetStringRecord{transaction: 7, block: storage.NewBlockID("tbl", 4), offset: 16, oldValue: "以前"}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "unpin") || !reflect.DeepEqual(target.calls, []string{"pin", "setstring", "unpin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}
