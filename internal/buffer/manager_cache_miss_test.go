package buffer

import (
	"sync"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestPinCacheMissAssignsTheFirstUnpinnedFrame(t *testing.T) {
	t.Parallel()

	t.Run("unassigned", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 32)
		manager, err := NewManager(files, logs, 2)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		target, targetPage := appendDataBlock(t, files, 21)

		frame, pinned, err := manager.pinCacheMiss(target)
		if err != nil {
			t.Fatalf("pinCacheMiss() error = %v", err)
		}
		if !pinned || frame != manager.frames[0] {
			t.Fatalf("pinCacheMiss() = (%p, %t), want (frames[0], true)", frame, pinned)
		}
		assertPinnedAssignment(t, manager, frame, target, targetPage.Snapshot(), 1)
	})

	t.Run("clean assigned", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 32)
		manager, err := NewManager(files, logs, 1)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		oldBlock, _ := appendDataBlock(t, files, 1)
		target, targetPage := appendDataBlock(t, files, 22)
		if err := manager.frames[0].assignToBlock(oldBlock); err != nil {
			t.Fatalf("assignToBlock() error = %v", err)
		}

		frame, pinned, err := manager.pinCacheMiss(target)
		if err != nil {
			t.Fatalf("pinCacheMiss() error = %v", err)
		}
		if !pinned || frame != manager.frames[0] {
			t.Fatalf("pinCacheMiss() = (%p, %t), want (frames[0], true)", frame, pinned)
		}
		assertPinnedAssignment(t, manager, frame, target, targetPage.Snapshot(), 0)
	})

	t.Run("dirty assigned", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 64)
		manager, err := NewManager(files, logs, 1)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		oldBlock, _ := appendDataBlock(t, files, 1)
		target, targetPage := appendDataBlock(t, files, 23)
		frame := manager.frames[0]
		if err := frame.assignToBlock(oldBlock); err != nil {
			t.Fatalf("assignToBlock() error = %v", err)
		}
		if err := frame.Contents().SetInt(0, 99); err != nil {
			t.Fatalf("Contents().SetInt() error = %v", err)
		}
		lsn, err := logs.Append([]byte("manager dirty replacement"))
		if err != nil {
			t.Fatalf("logs.Append() error = %v", err)
		}
		frame.MarkModified(TransactionID(8))
		if err := frame.RecordRecoveryLSN(lsn); err != nil {
			t.Fatalf("RecordRecoveryLSN() error = %v", err)
		}

		got, pinned, err := manager.pinCacheMiss(target)
		if err != nil {
			t.Fatalf("pinCacheMiss() error = %v", err)
		}
		if !pinned || got != frame {
			t.Fatalf("pinCacheMiss() = (%p, %t), want (frames[0], true)", got, pinned)
		}
		assertPinnedAssignment(t, manager, frame, target, targetPage.Snapshot(), 0)
		if diskValue := readBlockInt(t, files, oldBlock); diskValue != 99 {
			t.Errorf("old disk block value = %d, want flushed 99", diskValue)
		}
		if _, dirty := frame.ModifyingTransaction(); dirty {
			t.Error("frame remains dirty after replacement")
		}
	})
}

func TestPinCacheMissReportsExhaustionOrExistingTargetWithoutMutation(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	existing, existingPage := appendDataBlock(t, files, 1)
	missingTarget, _ := appendDataBlock(t, files, 2)
	if err := manager.frames[0].assignToBlock(existing); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}
	frame, found := manager.pinExistingBuffer(existing)
	if !found {
		t.Fatal("pinExistingBuffer() found = false, want true")
	}

	if got, pinned, err := manager.pinCacheMiss(missingTarget); err != nil || pinned || got != nil {
		t.Errorf("pinCacheMiss(all pinned) = (%p, %t, %v), want (nil, false, nil)", got, pinned, err)
	}
	if err := manager.Unpin(frame); err != nil {
		t.Fatalf("Unpin() error = %v", err)
	}
	if got, pinned, err := manager.pinCacheMiss(existing); err == nil || pinned || got != nil {
		t.Errorf("pinCacheMiss(existing) = (%p, %t, %v), want (nil, false, error)", got, pinned, err)
	}
	assertReassignmentState(t, frame, existing, existingPage.Snapshot())
	if got := frame.PinCount(); got != 0 {
		t.Errorf("PinCount() after rejected calls = %d, want 0", got)
	}
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after rejected calls = %d, want 1", got)
	}
}

func TestPinCacheMissFailureDoesNotPinOrConsumeAvailability(t *testing.T) {
	t.Parallel()

	t.Run("unassigned read failure", func(t *testing.T) {
		files, logs := newManagerDependencies(t, 32)
		manager, err := NewManager(files, logs, 1)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		missing := storage.NewBlockID("missing.tbl", 0)

		if frame, pinned, err := manager.pinCacheMiss(missing); err == nil || pinned || frame != nil {
			t.Fatalf("pinCacheMiss(missing) = (%p, %t, %v), want (nil, false, error)", frame, pinned, err)
		}
		if _, assigned := manager.frames[0].Block(); assigned {
			t.Error("frame became assigned after failed read")
		}
		if got := manager.frames[0].PinCount(); got != 0 {
			t.Errorf("PinCount() after failed read = %d, want 0", got)
		}
		if got := manager.Available(); got != 1 {
			t.Errorf("Available() after failed read = %d, want 1", got)
		}
	})

	t.Run("dirty log failure", func(t *testing.T) {
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
		frame.MarkModified(TransactionID(9))
		if err := frame.RecordRecoveryLSN(logmanager.LSN(99)); err != nil {
			t.Fatalf("RecordRecoveryLSN() error = %v", err)
		}

		if got, pinned, err := manager.pinCacheMiss(target); err == nil || pinned || got != nil {
			t.Fatalf("pinCacheMiss(log failure) = (%p, %t, %v), want (nil, false, error)", got, pinned, err)
		}
		assertReassignmentState(t, frame, oldBlock, oldPage.Snapshot())
		assertDirtyMetadata(t, frame, 9, 99, true)
		if got := frame.PinCount(); got != 0 {
			t.Errorf("PinCount() after log failure = %d, want 0", got)
		}
		if got := manager.Available(); got != 1 {
			t.Errorf("Available() after log failure = %d, want 1", got)
		}
	})
}

func TestPinCacheMissConcurrentCallsClaimDifferentFrames(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 2)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	targets := make([]storage.BlockID, 2)
	for index := range targets {
		targets[index], _ = appendDataBlock(t, files, int32(index+1))
	}

	type result struct {
		frame  *Buffer
		pinned bool
		err    error
	}
	results := make(chan result, len(targets))
	var wait sync.WaitGroup
	for _, target := range targets {
		wait.Add(1)
		go func(block storage.BlockID) {
			defer wait.Done()
			frame, pinned, err := manager.pinCacheMiss(block)
			results <- result{frame: frame, pinned: pinned, err: err}
		}(target)
	}
	wait.Wait()
	close(results)

	claimed := make(map[*Buffer]bool)
	for got := range results {
		if got.err != nil || !got.pinned || got.frame == nil {
			t.Errorf("pinCacheMiss() = (%p, %t, %v), want a pinned frame", got.frame, got.pinned, got.err)
			continue
		}
		if claimed[got.frame] {
			t.Errorf("frame %p was claimed by both cache misses", got.frame)
		}
		claimed[got.frame] = true
	}
	if len(claimed) != 2 {
		t.Errorf("claimed frame count = %d, want 2", len(claimed))
	}
	if got := manager.Available(); got != 0 {
		t.Errorf("Available() after concurrent cache misses = %d, want 0", got)
	}
}

func assertPinnedAssignment(t *testing.T, manager *Manager, frame *Buffer, block storage.BlockID, contents []byte, available int) {
	t.Helper()

	assertReassignmentState(t, frame, block, contents)
	if got := frame.PinCount(); got != 1 {
		t.Errorf("PinCount() = %d, want 1", got)
	}
	if got := manager.Available(); got != available {
		t.Errorf("Available() = %d, want %d", got, available)
	}
}
