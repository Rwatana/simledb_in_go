package buffer

import "testing"

func TestChooseUnpinnedBufferReturnsTheFirstCandidateWithoutMutation(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 3)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	first, found := manager.chooseUnpinnedBuffer()
	if !found || first != manager.frames[0] {
		t.Errorf("chooseUnpinnedBuffer() = (%p, %t), want (frames[0], true)", first, found)
	}
	if got := manager.Available(); got != 3 {
		t.Errorf("Available() after first selection = %d, want unchanged 3", got)
	}

	block, _ := appendDataBlock(t, files, 1)
	if err := manager.frames[1].assignToBlock(block); err != nil {
		t.Fatalf("frames[1].assignToBlock() error = %v", err)
	}
	manager.frames[1].MarkModified(TransactionID(9))
	manager.frames[0].pin()

	second, found := manager.chooseUnpinnedBuffer()
	if !found || second != manager.frames[1] {
		t.Errorf("chooseUnpinnedBuffer() after pin = (%p, %t), want (frames[1], true)", second, found)
	}
	if transaction, modified := manager.frames[1].ModifyingTransaction(); !modified || transaction != 9 {
		t.Errorf("selected dirty metadata = (%d, %t), want (9, true)", transaction, modified)
	}
	if assigned, exists := manager.frames[1].Block(); !exists || assigned != block {
		t.Errorf("selected Block() = (%v, %t), want (%v, true)", assigned, exists, block)
	}
	if got := manager.frames[1].PinCount(); got != 0 {
		t.Errorf("selected PinCount() = %d, want unchanged 0", got)
	}
	if got := manager.Available(); got != 3 {
		t.Errorf("Available() after dirty selection = %d, want unchanged 3", got)
	}

	manager.frames[1].pin()
	manager.frames[2].pin()
	if frame, found := manager.chooseUnpinnedBuffer(); found || frame != nil {
		t.Errorf("chooseUnpinnedBuffer() with all pinned = (%p, %t), want (nil, false)", frame, found)
	}
	for index, frame := range manager.frames {
		if got := frame.PinCount(); got != 1 {
			t.Errorf("frames[%d].PinCount() after exhausted selection = %d, want unchanged 1", index, got)
		}
	}
}
