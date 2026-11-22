package buffer

import (
	"sync"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPinExistingBufferUpdatesAvailabilityOnlyOnFirstPin(t *testing.T) {
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

	first, found := manager.pinExistingBuffer(block)
	if !found || first != manager.frames[1] {
		t.Errorf("first pinExistingBuffer() = (%p, %t), want (frames[1], true)", first, found)
	}
	if got := first.PinCount(); got != 1 {
		t.Errorf("PinCount() after first pin = %d, want 1", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after first pin = %d, want 1", got)
	}

	second, found := manager.pinExistingBuffer(block)
	if !found || second != first {
		t.Errorf("second pinExistingBuffer() = (%p, %t), want (%p, true)", second, found, first)
	}
	if got := first.PinCount(); got != 2 {
		t.Errorf("PinCount() after second pin = %d, want 2", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after second pin = %d, want unchanged 1", got)
	}

	missing := storage.NewBlockID("missing.tbl", 0)
	if frame, found := manager.pinExistingBuffer(missing); found || frame != nil {
		t.Errorf("pinExistingBuffer(missing) = (%p, %t), want (nil, false)", frame, found)
	}
	if got := first.PinCount(); got != 2 {
		t.Errorf("PinCount() after missing pin = %d, want unchanged 2", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after missing pin = %d, want unchanged 1", got)
	}
}

func TestManagerPinRepinsCachedBufferAfterFinalUnpin(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 2)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)

	first, err := manager.Pin(block)
	if err != nil {
		t.Fatalf("first Pin() error = %v", err)
	}
	if err := manager.Unpin(first); err != nil {
		t.Fatalf("Unpin() error = %v", err)
	}
	if got := manager.Available(); got != 2 {
		t.Fatalf("Available() after final unpin = %d, want 2", got)
	}

	repinned, err := manager.Pin(block)
	if err != nil {
		t.Fatalf("repin cached block error = %v", err)
	}
	if repinned != first {
		t.Errorf("repinned frame = %p, want cached frame %p", repinned, first)
	}
	if got := repinned.PinCount(); got != 1 {
		t.Errorf("PinCount() after repin = %d, want 1", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after repin = %d, want 1", got)
	}
}

func TestPinExistingBufferConcurrentCallsKeepEveryPinAndOneTransition(t *testing.T) {
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
	var wait sync.WaitGroup
	errors := make(chan string, clients)
	for range clients {
		wait.Add(1)
		go func() {
			defer wait.Done()
			frame, found := manager.pinExistingBuffer(block)
			if !found || frame != manager.frames[0] {
				errors <- "pinExistingBuffer() did not return the assigned frame"
			}
		}()
	}
	wait.Wait()
	close(errors)
	for message := range errors {
		t.Error(message)
	}

	if got := manager.frames[0].PinCount(); got != clients {
		t.Errorf("PinCount() after concurrent pins = %d, want %d", got, clients)
	}
	if got := manager.Available(); got != 2 {
		t.Errorf("Available() after concurrent pins = %d, want one transition to 2", got)
	}
}
