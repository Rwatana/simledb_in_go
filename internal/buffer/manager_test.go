package buffer

import (
	"bytes"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

func TestNewManagerCreatesAFixedSizePoolOfFreshFrames(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 3)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if got := manager.Capacity(); got != 3 {
		t.Errorf("Capacity() = %d, want 3", got)
	}
	if got := manager.Available(); got != 3 {
		t.Errorf("Available() = %d, want 3", got)
	}
	if got := len(manager.frames); got != 3 {
		t.Fatalf("len(frames) = %d, want 3", got)
	}
	seen := make(map[*Buffer]struct{}, len(manager.frames))
	for index, frame := range manager.frames {
		if frame == nil {
			t.Fatalf("frames[%d] = nil, want a Buffer", index)
		}
		if _, duplicate := seen[frame]; duplicate {
			t.Errorf("frames[%d] aliases an earlier Buffer", index)
		}
		seen[frame] = struct{}{}
		if _, assigned := frame.Block(); assigned {
			t.Errorf("frames[%d] is assigned, want a fresh frame", index)
		}
		if frame.IsPinned() || frame.PinCount() != 0 {
			t.Errorf("frames[%d] pin state = (%t, %d), want (false, 0)", index, frame.IsPinned(), frame.PinCount())
		}
		if transaction, modified := frame.ModifyingTransaction(); modified {
			t.Errorf("frames[%d] modifying transaction = (%d, true), want (_, false)", index, transaction)
		}
		if lsn, exists := frame.RecoveryLSN(); exists {
			t.Errorf("frames[%d] recovery LSN = (%d, true), want (_, false)", index, lsn)
		}
		contents := frame.Contents()
		if got := contents.Capacity(); got != files.BlockSize() {
			t.Errorf("frames[%d] Page capacity = %d, want %d", index, got, files.BlockSize())
		}
		if got, want := contents.Snapshot(), make([]byte, files.BlockSize()); !bytes.Equal(got, want) {
			t.Errorf("frames[%d] contents = %v, want zero-filled Page", index, got)
		}
	}
}

func TestNewManagerRejectsInvalidCapacityOrDependencies(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	for _, capacity := range []int{0, -1} {
		if _, err := NewManager(files, logs, capacity); err == nil {
			t.Errorf("NewManager() capacity %d error = nil, want an invalid capacity error", capacity)
		}
	}
	if _, err := NewManager(nil, logs, 1); err == nil {
		t.Error("NewManager(nil, logs, 1) error = nil, want a FileManager dependency error")
	}
	if _, err := NewManager(files, nil, 1); err == nil {
		t.Error("NewManager(files, nil, 1) error = nil, want a LogManager dependency error")
	}
}

func newManagerDependencies(t *testing.T, blockSize int) (*storagefile.Manager, *logmanager.Manager) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	return files, logs
}
