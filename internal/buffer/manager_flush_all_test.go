package buffer

import (
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagerFlushAllFlushesOnlyMatchingTransactionFrames(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 64)
	manager, err := NewManager(files, logs, 4)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	blocks := assignManagerFrames(t, manager, []int32{10, 20, 30, 40})

	if err := manager.frames[0].Contents().SetInt(0, 101); err != nil {
		t.Fatalf("frames[0].Contents().SetInt() error = %v", err)
	}
	lsn, err := logs.Append([]byte("transaction 7 update"))
	if err != nil {
		t.Fatalf("logs.Append() error = %v", err)
	}
	manager.frames[0].MarkModified(TransactionID(7))
	if err := manager.frames[0].RecordRecoveryLSN(lsn); err != nil {
		t.Fatalf("frames[0].RecordRecoveryLSN() error = %v", err)
	}
	if err := manager.frames[1].Contents().SetInt(0, 202); err != nil {
		t.Fatalf("frames[1].Contents().SetInt() error = %v", err)
	}
	manager.frames[1].MarkModified(TransactionID(8))
	if err := manager.frames[2].Contents().SetInt(0, 303); err != nil {
		t.Fatalf("frames[2].Contents().SetInt() error = %v", err)
	}
	if err := manager.frames[3].Contents().SetInt(0, 404); err != nil {
		t.Fatalf("frames[3].Contents().SetInt() error = %v", err)
	}
	manager.frames[3].MarkModified(TransactionID(7))
	if _, found := manager.pinExistingBuffer(blocks[3]); !found {
		t.Fatal("pinExistingBuffer(blocks[3]) found = false, want true")
	}

	if err := manager.FlushAll(TransactionID(7)); err != nil {
		t.Fatalf("FlushAll(7) error = %v", err)
	}

	for index, want := range []int32{101, 20, 30, 404} {
		if got := readBlockInt(t, files, blocks[index]); got != want {
			t.Errorf("disk block %d value = %d, want %d", index, got, want)
		}
	}
	if _, dirty := manager.frames[0].ModifyingTransaction(); dirty {
		t.Error("matching frames[0] remains dirty")
	}
	if transaction, dirty := manager.frames[1].ModifyingTransaction(); !dirty || transaction != 8 {
		t.Errorf("frames[1] dirty state = (%d, %t), want (8, true)", transaction, dirty)
	}
	if _, dirty := manager.frames[2].ModifyingTransaction(); dirty {
		t.Error("clean frames[2] became dirty")
	}
	if _, dirty := manager.frames[3].ModifyingTransaction(); dirty {
		t.Error("matching pinned frames[3] remains dirty")
	}
	if got := manager.frames[3].PinCount(); got != 1 {
		t.Errorf("frames[3].PinCount() after FlushAll = %d, want unchanged 1", got)
	}
	if got := manager.Available(); got != 3 {
		t.Errorf("Available() after FlushAll = %d, want unchanged 3", got)
	}
	diskLog, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(log block) error = %v", err)
	}
	boundary, err := diskLog.GetInt(0)
	if err != nil {
		t.Fatalf("diskLog.GetInt(0) error = %v", err)
	}
	if boundary == int32(files.BlockSize()) {
		t.Error("recovery log was not durable before matching data pages")
	}
}

func TestManagerFlushAllStopsAtFirstFailure(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 3)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	blocks := assignManagerFrames(t, manager, []int32{1, 2, 3})
	for index, value := range []int32{11, 22, 33} {
		if err := manager.frames[index].Contents().SetInt(0, value); err != nil {
			t.Fatalf("frames[%d].Contents().SetInt() error = %v", index, err)
		}
		manager.frames[index].MarkModified(TransactionID(5))
	}
	if err := manager.frames[1].RecordRecoveryLSN(logmanager.LSN(99)); err != nil {
		t.Fatalf("frames[1].RecordRecoveryLSN() error = %v", err)
	}

	err = manager.FlushAll(TransactionID(5))
	if err == nil {
		t.Fatal("FlushAll(5) error = nil, want a WAL failure")
	}
	if !strings.Contains(err.Error(), "frame 1") {
		t.Errorf("FlushAll(5) error = %q, want frame 1 context", err)
	}
	for index, want := range []int32{11, 2, 3} {
		if got := readBlockInt(t, files, blocks[index]); got != want {
			t.Errorf("disk block %d value after failure = %d, want %d", index, got, want)
		}
	}
	if _, dirty := manager.frames[0].ModifyingTransaction(); dirty {
		t.Error("frames[0] remains dirty after successful earlier flush")
	}
	assertDirtyMetadata(t, manager.frames[1], 5, 99, true)
	assertDirtyMetadata(t, manager.frames[2], 5, 0, false)
}

func TestManagerFlushAllWithoutMatchingFramesIsNoOp(t *testing.T) {
	t.Parallel()

	files, logs := newManagerDependencies(t, 32)
	manager, err := NewManager(files, logs, 1)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	blocks := assignManagerFrames(t, manager, []int32{1})
	if err := manager.frames[0].Contents().SetInt(0, 9); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	manager.frames[0].MarkModified(TransactionID(2))

	if err := manager.FlushAll(TransactionID(3)); err != nil {
		t.Fatalf("FlushAll(3) error = %v, want nil", err)
	}
	if got := readBlockInt(t, files, blocks[0]); got != 1 {
		t.Errorf("disk value after no-op FlushAll = %d, want unchanged 1", got)
	}
	assertDirtyMetadata(t, manager.frames[0], 2, 0, false)
	if got := manager.Available(); got != 1 {
		t.Errorf("Available() after no-op FlushAll = %d, want 1", got)
	}
}

func assignManagerFrames(t *testing.T, manager *Manager, values []int32) []storage.BlockID {
	t.Helper()

	blocks := make([]storage.BlockID, len(values))
	for index, value := range values {
		block, page := appendDataBlock(t, manager.frames[index].files, value)
		blocks[index] = block
		if err := manager.frames[index].assignToBlock(block); err != nil {
			t.Fatalf("frames[%d].assignToBlock() error = %v", index, err)
		}
		if got := manager.frames[index].Contents().Snapshot(); len(got) != len(page.Snapshot()) {
			t.Fatalf("frames[%d] Page length = %d, want %d", index, len(got), len(page.Snapshot()))
		}
	}
	return blocks
}
