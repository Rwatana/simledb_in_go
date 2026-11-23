package buffer

import (
	"sync"
	"testing"
)

func TestManagerUnpinRestoresAvailabilityOnlyOnFinalRelease(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 2)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)
	if err := manager.frames[1].assignToBlock(block); err != nil {
		t.Fatalf("frames[1].assignToBlock() error = %v", err)
	}
	frame, found := manager.pinExistingBuffer(block)
	if !found {
		t.Fatal("first pinExistingBuffer() found = false, want true")
	}
	if _, found := manager.pinExistingBuffer(block); !found {
		t.Fatal("second pinExistingBuffer() found = false, want true")
	}

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("first Unpin() error = %v", err)
	}
	if got := frame.PinCount(); got != 1 {
		t.Errorf("PinCount() after first Unpin() = %d, want 1", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after first Unpin() = %d, want unchanged 1", got)
	}

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("second Unpin() error = %v", err)
	}
	if got := frame.PinCount(); got != 0 {
		t.Errorf("PinCount() after second Unpin() = %d, want 0", got)
	}
	if got := manager.Available(); got != 2 {
		t.Errorf("Available() after second Unpin() = %d, want 2", got)
	}
}

func TestManagerUnpinRejectsInvalidFramesWithoutMutation(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := manager.Unpin(nil); err == nil {
		t.Fatal("Unpin(nil) error = nil, want a nil-frame error")
	}
	if err := manager.Unpin(manager.frames[0]); err == nil {
		t.Fatal("Unpin(unpinned) error = nil, want a pin-count error")
	}

	foreign, err := New(files, logs)
	if err != nil {
		t.Fatalf("New(foreign) error = %v", err)
	}
	foreign.pin()
	if err := manager.Unpin(foreign); err == nil {
		t.Fatal("Unpin(foreign) error = nil, want an ownership error")
	}
	if got := foreign.PinCount(); got != 1 {
		t.Errorf("foreign PinCount() after rejected Unpin() = %d, want unchanged 1", got)
	}
	if got := manager.frames[0].PinCount(); got != 0 {
		t.Errorf("owned PinCount() after rejected calls = %d, want unchanged 0", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after rejected calls = %d, want unchanged 1", got)
	}
}

func TestManagerUnpinConcurrentCallsKeepEveryReleaseAndOneTransition(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 3)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)
	if err := manager.frames[0].assignToBlock(block); err != nil {
		t.Fatalf("frames[0].assignToBlock() error = %v", err)
	}

	const clients = 32
	var frame *Buffer
	for range clients {
		var found bool
		frame, found = manager.pinExistingBuffer(block)
		if !found {
			t.Fatal("pinExistingBuffer() found = false, want true")
		}
	}

	var wait sync.WaitGroup
	errors := make(chan error, clients)
	for range clients {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := manager.Unpin(frame); err != nil {
				errors <- err
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent Unpin() error = %v", err)
	}

	if got := frame.PinCount(); got != 0 {
		t.Errorf("PinCount() after concurrent Unpin() calls = %d, want 0", got)
	}
	if got := manager.Available(); got != 3 {
		t.Errorf("Available() after concurrent Unpin() calls = %d, want one transition to 3", got)
	}
}
