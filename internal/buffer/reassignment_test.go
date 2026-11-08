package buffer

import (
	"bytes"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestReassignToBlockLoadsAnotherBlockIntoACleanUnpinnedFrame(t *testing.T) {
	t.Parallel()

	frame, files := newBufferWithFiles(t)
	first, _ := appendDataBlock(t, files, 1)
	second, secondPage := appendDataBlock(t, files, 2)
	if err := frame.assignToBlock(first); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}

	if err := frame.reassignToBlock(second); err != nil {
		t.Fatalf("reassignToBlock() error = %v", err)
	}

	if assigned, ok := frame.Block(); !ok || assigned != second {
		t.Errorf("Block() = (%v, %t), want (%v, true)", assigned, ok, second)
	}
	if got := frame.Contents().Snapshot(); !bytes.Equal(got, secondPage.Snapshot()) {
		t.Errorf("Contents() = %v, want target disk Page %v", got, secondPage.Snapshot())
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() after reassignment = %d, want 0", frame.PinCount())
	}
}

func TestReassignToBlockRejectsUnassignedDirtyOrPinnedFrames(t *testing.T) {
	t.Parallel()

	t.Run("unassigned", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		target, _ := appendDataBlock(t, files, 2)
		before := frame.Contents().Snapshot()

		if err := frame.reassignToBlock(target); err == nil {
			t.Fatal("reassignToBlock() unassigned frame error = nil, want an assignment error")
		}
		if _, assigned := frame.Block(); assigned {
			t.Error("unassigned frame became assigned after rejected reassignment")
		}
		if got := frame.Contents().Snapshot(); !bytes.Equal(got, before) {
			t.Error("Contents() changed after rejected unassigned reassignment")
		}
	})

	t.Run("dirty", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		first, _ := appendDataBlock(t, files, 1)
		second, _ := appendDataBlock(t, files, 2)
		if err := frame.assignToBlock(first); err != nil {
			t.Fatalf("assignToBlock() error = %v", err)
		}
		if err := frame.Contents().SetInt(0, 88); err != nil {
			t.Fatalf("Contents().SetInt() error = %v", err)
		}
		frame.MarkModified(TransactionID(7))
		before := frame.Contents().Snapshot()

		if err := frame.reassignToBlock(second); err == nil {
			t.Fatal("reassignToBlock() dirty frame error = nil, want a dirty-state error")
		}
		assertReassignmentState(t, frame, first, before)
		if transaction, modified := frame.ModifyingTransaction(); !modified || transaction != 7 {
			t.Errorf("dirty state after rejection = (%d, %t), want (7, true)", transaction, modified)
		}
	})

	t.Run("pinned", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		first, firstPage := appendDataBlock(t, files, 1)
		second, _ := appendDataBlock(t, files, 2)
		if err := frame.assignToBlock(first); err != nil {
			t.Fatalf("assignToBlock() error = %v", err)
		}
		frame.pin()

		if err := frame.reassignToBlock(second); err == nil {
			t.Fatal("reassignToBlock() pinned frame error = nil, want a pinned-state error")
		}
		assertReassignmentState(t, frame, first, firstPage.Snapshot())
		if frame.PinCount() != 1 {
			t.Errorf("PinCount() after rejection = %d, want 1", frame.PinCount())
		}
	})
}

func TestReassignToBlockPreservesCurrentStateWhenTargetReadFails(t *testing.T) {
	t.Parallel()

	frame, files := newBufferWithFiles(t)
	first, firstPage := appendDataBlock(t, files, 1)
	if err := frame.assignToBlock(first); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}
	missing := storage.NewBlockID("missing.tbl", 0)

	if err := frame.reassignToBlock(missing); err == nil {
		t.Fatal("reassignToBlock(missing) error = nil, want a read error")
	}

	assertReassignmentState(t, frame, first, firstPage.Snapshot())
}

func assertReassignmentState(t *testing.T, frame *Buffer, block storage.BlockID, contents []byte) {
	t.Helper()

	if assigned, ok := frame.Block(); !ok || assigned != block {
		t.Errorf("Block() after rejected reassignment = (%v, %t), want (%v, true)", assigned, ok, block)
	}
	if got := frame.Contents().Snapshot(); !bytes.Equal(got, contents) {
		t.Error("Contents() changed after rejected reassignment")
	}
}
