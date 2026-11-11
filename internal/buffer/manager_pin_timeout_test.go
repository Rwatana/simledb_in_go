package buffer

import (
	"errors"
	"testing"
	"time"
)

func TestNewManagerUsesBookPinWaitTimeout(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if got := manager.pinWaitTimeout; got != 10*time.Second {
		t.Errorf("pinWaitTimeout = %v, want 10s", got)
	}
}

func TestManagerPinTimesOutWithoutMutationAndCanRecover(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.pinWaitTimeout = 40 * time.Millisecond
	current, currentPage := appendDataBlock(t, files, 1)
	target, _ := appendDataBlock(t, files, 2)
	owned, err := manager.Pin(current)
	if err != nil {
		t.Fatalf("Pin(current) error = %v", err)
	}

	started := time.Now()
	frame, err := manager.Pin(target)
	if !errors.Is(err, ErrPinTimeout) {
		t.Fatalf("Pin(target) error = %v, want ErrPinTimeout", err)
	}
	if frame != nil {
		t.Errorf("Pin(target) frame = %p, want nil after timeout", frame)
	}
	if elapsed := time.Since(started); elapsed < manager.pinWaitTimeout/2 {
		t.Errorf("Pin(target) returned after %v, want a bounded wait near %v", elapsed, manager.pinWaitTimeout)
	}
	assertReassignmentState(t, owned, current, currentPage.Snapshot())
	if got := owned.PinCount(); got != 1 {
		t.Errorf("PinCount() after timeout = %d, want unchanged 1", got)
	}
	if got := manager.Available(); got != 0 {
		t.Errorf("Available() after timeout = %d, want unchanged 0", got)
	}

	if err := manager.Unpin(owned); err != nil {
		t.Fatalf("Unpin() after timeout error = %v", err)
	}
	recovered, err := manager.Pin(target)
	if err != nil {
		t.Fatalf("Pin(target) after release error = %v", err)
	}
	if recovered != owned {
		t.Errorf("Pin(target) after release frame = %p, want %p", recovered, owned)
	}
}

func TestManagerPinReleaseBeforeDeadlineStillSucceeds(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.pinWaitTimeout = time.Second
	current, _ := appendDataBlock(t, files, 1)
	target, _ := appendDataBlock(t, files, 2)
	owned, err := manager.Pin(current)
	if err != nil {
		t.Fatalf("Pin(current) error = %v", err)
	}
	result := pinInBackground(manager, target)
	assertPinStillWaiting(t, result, "before release")

	if err := manager.Unpin(owned); err != nil {
		t.Fatalf("Unpin() error = %v", err)
	}
	got := awaitPinResult(t, result)
	if got.err != nil {
		t.Fatalf("Pin(target) error = %v", got.err)
	}
	if got.frame != owned {
		t.Errorf("Pin(target) frame = %p, want released frame %p", got.frame, owned)
	}
}

func TestManagerConcurrentPinWaitersCanIndependentlyTimeOut(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.pinWaitTimeout = 50 * time.Millisecond
	current, _ := appendDataBlock(t, files, 1)
	owned, err := manager.Pin(current)
	if err != nil {
		t.Fatalf("Pin(current) error = %v", err)
	}

	const waiters = 2
	results := make(chan pinResult, waiters)
	for index := range waiters {
		target, _ := appendDataBlock(t, files, int32(index+2))
		go func() {
			frame, err := manager.Pin(target)
			results <- pinResult{frame: frame, err: err}
		}()
	}
	for range waiters {
		got := awaitPinResult(t, results)
		if got.frame != nil || !errors.Is(got.err, ErrPinTimeout) {
			t.Errorf("Pin() = (%p, %v), want (nil, ErrPinTimeout)", got.frame, got.err)
		}
	}
	if got := owned.PinCount(); got != 1 {
		t.Errorf("owned PinCount() after waiter timeouts = %d, want 1", got)
	}
	if got := manager.Available(); got != 0 {
		t.Errorf("Available() after waiter timeouts = %d, want 0", got)
	}
}
