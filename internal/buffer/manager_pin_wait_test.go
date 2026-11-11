package buffer

import (
	"testing"
	"time"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagerPinReturnsCachedFrameImmediately(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	block, _ := appendDataBlock(t, files, 1)

	first, err := manager.Pin(block)
	if err != nil {
		t.Fatalf("first Pin() error = %v", err)
	}
	second, err := manager.Pin(block)
	if err != nil {
		t.Fatalf("second Pin() error = %v", err)
	}
	if second != first {
		t.Errorf("second Pin() frame = %p, want cached frame %p", second, first)
	}
	if got := first.PinCount(); got != 2 {
		t.Errorf("PinCount() = %d, want 2", got)
	}
	if got := manager.Available(); got != 0 {
		t.Errorf("Available() = %d, want 0", got)
	}
}

func TestManagerPinWaitsForFinalReleaseAndRetries(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	current, _ := appendDataBlock(t, files, 1)
	target, _ := appendDataBlock(t, files, 2)
	frame, err := manager.Pin(current)
	if err != nil {
		t.Fatalf("first Pin(current) error = %v", err)
	}
	if _, err := manager.Pin(current); err != nil {
		t.Fatalf("second Pin(current) error = %v", err)
	}

	result := pinInBackground(manager, target)
	assertPinStillWaiting(t, result, "before any release")

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("first Unpin() error = %v", err)
	}
	assertPinStillWaiting(t, result, "after intermediate release")

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("second Unpin() error = %v", err)
	}
	got := awaitPinResult(t, result)
	if got.err != nil {
		t.Fatalf("waiting Pin() error = %v", got.err)
	}
	if got.frame != frame {
		t.Errorf("waiting Pin() frame = %p, want released frame %p", got.frame, frame)
	}
	if assigned, exists := got.frame.Block(); !exists || assigned != target {
		t.Errorf("Block() = (%v, %t), want (%v, true)", assigned, exists, target)
	}
	if pinCount := got.frame.PinCount(); pinCount != 1 {
		t.Errorf("PinCount() = %d, want 1", pinCount)
	}
	if available := manager.Available(); available != 0 {
		t.Errorf("Available() = %d, want 0", available)
	}
}

func TestManagerPinWaitersRecheckAfterBroadcast(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	current, _ := appendDataBlock(t, files, 1)
	targets := make([]storage.BlockID, 2)
	for index := range targets {
		targets[index], _ = appendDataBlock(t, files, int32(index+2))
	}
	frame, err := manager.Pin(current)
	if err != nil {
		t.Fatalf("Pin(current) error = %v", err)
	}

	results := make(chan pinResult, 2)
	for _, target := range targets {
		target := target
		go func() {
			pinned, err := manager.Pin(target)
			results <- pinResult{frame: pinned, err: err}
		}()
	}
	select {
	case got := <-results:
		t.Fatalf("Pin() returned before release: frame=%p error=%v", got.frame, got.err)
	case <-time.After(50 * time.Millisecond):
	}

	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("first Unpin() error = %v", err)
	}
	first := awaitPinResult(t, results)
	if first.err != nil {
		t.Fatalf("first waiting Pin() error = %v", first.err)
	}
	assertPinStillWaiting(t, results, "while the first awakened client owns the frame")

	if err := manager.Unpin(first.frame); err != nil {
		t.Fatalf("second Unpin() error = %v", err)
	}
	second := awaitPinResult(t, results)
	if second.err != nil {
		t.Fatalf("second waiting Pin() error = %v", second.err)
	}
	if second.frame != first.frame {
		t.Errorf("second waiting frame = %p, want reused frame %p", second.frame, first.frame)
	}
}

func TestManagerPinReturnsAssignmentErrorsWithoutWaiting(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	missing := storage.NewBlockID("missing.tbl", 0)

	if frame, err := manager.Pin(missing); err == nil || frame != nil {
		t.Fatalf("Pin(missing) = (%p, %v), want (nil, error)", frame, err)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after failed Pin() = %d, want 1", got)
	}
}

type pinResult struct {
	frame *Buffer
	err   error
}

func pinInBackground(manager *Manager, block storage.BlockID) <-chan pinResult {
	result := make(chan pinResult, 1)
	go func() {
		frame, err := manager.Pin(block)
		result <- pinResult{frame: frame, err: err}
	}()
	return result
}

func assertPinStillWaiting(t *testing.T, result <-chan pinResult, point string) {
	t.Helper()

	select {
	case got := <-result:
		t.Fatalf("Pin() returned %s: frame=%p error=%v", point, got.frame, got.err)
	case <-time.After(50 * time.Millisecond):
	}
}

func awaitPinResult(t *testing.T, result <-chan pinResult) pinResult {
	t.Helper()

	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatal("Pin() did not return after a final release")
		return pinResult{}
	}
}
