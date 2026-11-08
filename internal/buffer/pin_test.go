package buffer

import (
	"path/filepath"
	"sync"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

func TestPinIncrementsCountAndReportsOnlyTheFirstTransition(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	if frame.IsPinned() {
		t.Error("IsPinned() = true, want false for a fresh frame")
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() = %d, want 0", frame.PinCount())
	}

	if becamePinned := frame.pin(); !becamePinned {
		t.Error("first pin() transition = false, want true")
	}
	if !frame.IsPinned() || frame.PinCount() != 1 {
		t.Errorf("after first pin: IsPinned() = %t, PinCount() = %d; want true, 1", frame.IsPinned(), frame.PinCount())
	}

	if becamePinned := frame.pin(); becamePinned {
		t.Error("second pin() transition = true, want false")
	}
	if !frame.IsPinned() || frame.PinCount() != 2 {
		t.Errorf("after second pin: IsPinned() = %t, PinCount() = %d; want true, 2", frame.IsPinned(), frame.PinCount())
	}
}

func TestPinIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	const goroutines = 32
	transitions := make(chan bool, goroutines)
	var wait sync.WaitGroup
	wait.Add(goroutines)
	for range goroutines {
		go func() {
			defer wait.Done()
			transitions <- frame.pin()
		}()
	}
	wait.Wait()
	close(transitions)

	transitionCount := 0
	for transition := range transitions {
		if transition {
			transitionCount++
		}
	}
	if transitionCount != 1 {
		t.Errorf("available-to-pinned transitions = %d, want 1", transitionCount)
	}
	if frame.PinCount() != goroutines {
		t.Errorf("PinCount() = %d, want %d", frame.PinCount(), goroutines)
	}
}

func newBufferForTest(t *testing.T) *Buffer {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	frame, err := New(files, logs)
	if err != nil {
		t.Fatalf("buffer.New() error = %v", err)
	}
	return frame
}
