package buffer

import (
	"sync"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestTryToPinComposesCacheMissAndCacheHit(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 2)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	target, targetPage := appendDataBlock(t, files, 41)

	first, pinned, err := manager.tryToPin(target)
	if err != nil {
		t.Fatalf("first tryToPin() error = %v", err)
	}
	if !pinned || first != manager.frames[0] {
		t.Fatalf("first tryToPin() = (%p, %t), want (frames[0], true)", first, pinned)
	}
	assertPinnedAssignment(t, manager, first, target, targetPage.Snapshot(), 1)

	second, pinned, err := manager.tryToPin(target)
	if err != nil {
		t.Fatalf("second tryToPin() error = %v", err)
	}
	if !pinned || second != first {
		t.Fatalf("second tryToPin() = (%p, %t), want (%p, true)", second, pinned, first)
	}
	if got := first.PinCount(); got != 2 {
		t.Errorf("PinCount() after cache hit = %d, want 2", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after cache hit = %d, want unchanged 1", got)
	}
	if _, assigned := manager.frames[1].Block(); assigned {
		t.Error("cache hit assigned a duplicate frame")
	}
}

func TestTryToPinReturnsImmediatelyWhenEveryFrameIsPinned(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	first, _ := appendDataBlock(t, files, 1)
	second, _ := appendDataBlock(t, files, 2)
	frame, pinned, err := manager.tryToPin(first)
	if err != nil || !pinned {
		t.Fatalf("tryToPin(first) = (%p, %t, %v), want a pinned frame", frame, pinned, err)
	}

	if got, pinned, err := manager.tryToPin(second); err != nil || pinned || got != nil {
		t.Errorf("tryToPin(all pinned) = (%p, %t, %v), want (nil, false, nil)", got, pinned, err)
	}
	if assigned, exists := frame.Block(); !exists || assigned != first {
		t.Errorf("Block() after exhaustion = (%v, %t), want (%v, true)", assigned, exists, first)
	}
	if got := frame.PinCount(); got != 1 {
		t.Errorf("PinCount() after exhaustion = %d, want unchanged 1", got)
	}
	if got := manager.Available(); got != 0 {
		t.Errorf("Available() after exhaustion = %d, want 0", got)
	}
}

func TestTryToPinPropagatesCacheMissFailuresWithoutConsumingAFrame(t *testing.T) {
	t.Parallel()

	t.Run("read failure", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 32)
		manager, err := NewManager(files, logs, 1)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		missing := storage.NewBlockID("missing.tbl", 0)

		if frame, pinned, err := manager.tryToPin(missing); err == nil || pinned || frame != nil {
			t.Fatalf("tryToPin(missing) = (%p, %t, %v), want (nil, false, error)", frame, pinned, err)
		}
		if got := manager.Available(); got != 1 {
			t.Errorf("Available() after read failure = %d, want 1", got)
		}
	})

	t.Run("WAL failure", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 32)
		manager, err := NewManager(files, logs, 1)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		oldBlock, oldPage := appendDataBlock(t, files, 1)
		target, _ := appendDataBlock(t, files, 2)
		frame := manager.frames[0]
		if err := frame.assignToBlock(oldBlock); err != nil {
			t.Fatalf("assignToBlock() error = %v", err)
		}
		frame.MarkModified(TransactionID(12))
		if err := frame.RecordRecoveryLSN(logmanager.LSN(99)); err != nil {
			t.Fatalf("RecordRecoveryLSN() error = %v", err)
		}

		if got, pinned, err := manager.tryToPin(target); err == nil || pinned || got != nil {
			t.Fatalf("tryToPin(WAL failure) = (%p, %t, %v), want (nil, false, error)", got, pinned, err)
		}
		assertReassignmentState(t, frame, oldBlock, oldPage.Snapshot())
		assertDirtyMetadata(t, frame, 12, 99, true)
		if got := manager.Available(); got != 1 {
			t.Errorf("Available() after WAL failure = %d, want 1", got)
		}
	})
}

func TestTryToPinConcurrentSameBlockUsesOneFrameAndKeepsEveryPin(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 2)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	target, _ := appendDataBlock(t, files, 1)

	const clients = 32
	type result struct {
		frame  *Buffer
		pinned bool
		err    error
	}
	results := make(chan result, clients)
	var wait sync.WaitGroup
	for range clients {
		wait.Add(1)
		go func() {
			defer wait.Done()
			frame, pinned, err := manager.tryToPin(target)
			results <- result{frame: frame, pinned: pinned, err: err}
		}()
	}
	wait.Wait()
	close(results)

	var shared *Buffer
	for got := range results {
		if got.err != nil || !got.pinned || got.frame == nil {
			t.Errorf("tryToPin() = (%p, %t, %v), want a pinned frame", got.frame, got.pinned, got.err)
			continue
		}
		if shared == nil {
			shared = got.frame
		} else if got.frame != shared {
			t.Errorf("tryToPin() frame = %p, want shared frame %p", got.frame, shared)
		}
	}
	if shared == nil {
		t.Fatal("concurrent tryToPin() calls returned no frame")
	}
	if got := shared.PinCount(); got != clients {
		t.Errorf("PinCount() = %d, want %d", got, clients)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() = %d, want 1", got)
	}
	assignedFrames := 0
	for _, frame := range manager.frames {
		if assigned, exists := frame.Block(); exists && assigned == target {
			assignedFrames++
		}
	}
	if assignedFrames != 1 {
		t.Errorf("frames assigned to target = %d, want 1", assignedFrames)
	}
}
