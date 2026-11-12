package recovery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingIntegerUndoTarget struct {
	calls     []string
	block     storage.BlockID
	offset    int32
	value     int32
	logUpdate bool
	pinErr    error
	setErr    error
	unpinErr  error
}

func (t *recordingIntegerUndoTarget) Pin(block storage.BlockID) error {
	t.calls = append(t.calls, "pin")
	t.block = block
	return t.pinErr
}

func (t *recordingIntegerUndoTarget) SetInt(block storage.BlockID, offset, value int32, logUpdate bool) error {
	t.calls = append(t.calls, "setint")
	t.block = block
	t.offset = offset
	t.value = value
	t.logUpdate = logUpdate
	return t.setErr
}

func (t *recordingIntegerUndoTarget) Unpin(block storage.BlockID) error {
	t.calls = append(t.calls, "unpin")
	t.block = block
	return t.unpinErr
}

func TestSetIntRecordUndoRestoresOldValueWithoutLogging(t *testing.T) {
	block := storage.NewBlockID("tbl", 4)
	record := SetIntRecord{transaction: 7, block: block, offset: 16, oldValue: 91}
	target := &recordingIntegerUndoTarget{}

	if err := record.Undo(target); err != nil {
		t.Fatalf("Undo() error = %v", err)
	}
	if !reflect.DeepEqual(target.calls, []string{"pin", "setint", "unpin"}) {
		t.Fatalf("calls = %v", target.calls)
	}
	if target.block != block || target.offset != 16 || target.value != 91 || target.logUpdate {
		t.Fatalf("restore = block %v offset %d value %d log %t", target.block, target.offset, target.value, target.logUpdate)
	}
}

func TestSetIntRecordUndoStopsAfterPinFailure(t *testing.T) {
	want := errors.New("pin failed")
	target := &recordingIntegerUndoTarget{pinErr: want}
	record := SetIntRecord{transaction: 7, block: storage.NewBlockID("tbl", 4)}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "pin") || !reflect.DeepEqual(target.calls, []string{"pin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}

func TestSetIntRecordUndoUnpinsAfterRestoreFailure(t *testing.T) {
	want := errors.New("restore failed")
	target := &recordingIntegerUndoTarget{setErr: want}
	record := SetIntRecord{transaction: 7, block: storage.NewBlockID("tbl", 4), offset: 16, oldValue: 91}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "restore") || !reflect.DeepEqual(target.calls, []string{"pin", "setint", "unpin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}

func TestSetIntRecordUndoReturnsUnpinFailure(t *testing.T) {
	want := errors.New("unpin failed")
	target := &recordingIntegerUndoTarget{unpinErr: want}
	record := SetIntRecord{transaction: 7, block: storage.NewBlockID("tbl", 4), offset: 16, oldValue: 91}

	err := record.Undo(target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "unpin") || !reflect.DeepEqual(target.calls, []string{"pin", "setint", "unpin"}) {
		t.Fatalf("error = %v calls = %v", err, target.calls)
	}
}
