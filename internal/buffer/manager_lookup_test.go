package buffer

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestFindExistingBufferReturnsTheExactAssignedFrameWithoutMutation(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 3)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	dataBlock, _ := appendDataBlock(t, files, 1)
	unassignedDataBlock, _ := appendDataBlock(t, files, 2)
	otherFileBlock, err := files.Append("other.tbl")
	if err != nil {
		t.Fatalf("Append(other.tbl) error = %v", err)
	}
	if err := manager.frames[0].assignToBlock(dataBlock); err != nil {
		t.Fatalf("frames[0].assignToBlock() error = %v", err)
	}
	if err := manager.frames[1].assignToBlock(otherFileBlock); err != nil {
		t.Fatalf("frames[1].assignToBlock() error = %v", err)
	}

	dataFrame, found := manager.findExistingBuffer(dataBlock)
	if !found || dataFrame != manager.frames[0] {
		t.Errorf("findExistingBuffer(%v) = (%p, %t), want (frames[0], true)", dataBlock, dataFrame, found)
	}
	otherFrame, found := manager.findExistingBuffer(otherFileBlock)
	if !found || otherFrame != manager.frames[1] {
		t.Errorf("findExistingBuffer(%v) = (%p, %t), want (frames[1], true)", otherFileBlock, otherFrame, found)
	}
	if frame, found := manager.findExistingBuffer(unassignedDataBlock); found || frame != nil {
		t.Errorf("findExistingBuffer(%v) = (%p, %t), want (nil, false)", unassignedDataBlock, frame, found)
	}
	if frame, found := manager.findExistingBuffer(storage.NewBlockID("missing.tbl", 0)); found || frame != nil {
		t.Errorf("findExistingBuffer(missing) = (%p, %t), want (nil, false)", frame, found)
	}

	if got := manager.Available(); got != 3 {
		t.Errorf("Available() after lookup = %d, want unchanged 3", got)
	}
	for index, frame := range manager.frames {
		if got := frame.PinCount(); got != 0 {
			t.Errorf("frames[%d].PinCount() after lookup = %d, want 0", index, got)
		}
	}
}
