package buffer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestReplaceDirtyWithBlockFlushesOldPageThenLoadsTarget(t *testing.T) {
	t.Parallel()

	frame, files, logs, oldBlock := newAssignedBuffer(t)
	targetBlock, targetPage := appendDataBlock(t, files, 2)
	if err := frame.Contents().SetInt(0, 99); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	lsn, err := logs.Append([]byte("replace dirty data page"))
	if err != nil {
		t.Fatalf("logs.Append() error = %v", err)
	}
	frame.MarkModified(TransactionID(11))
	if err := frame.RecordRecoveryLSN(lsn); err != nil {
		t.Fatalf("RecordRecoveryLSN() error = %v", err)
	}

	if err := frame.replaceDirtyWithBlock(targetBlock); err != nil {
		t.Fatalf("replaceDirtyWithBlock() error = %v", err)
	}

	if got := readBlockInt(t, files, oldBlock); got != 99 {
		t.Errorf("old disk block value = %d, want flushed value 99", got)
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
		t.Error("log boundary is unchanged, want recovery record durable before old data page")
	}
	assertReassignmentState(t, frame, targetBlock, targetPage.Snapshot())
	if _, modified := frame.ModifyingTransaction(); modified {
		t.Error("Buffer remains dirty after successful replacement")
	}
	if _, exists := frame.RecoveryLSN(); exists {
		t.Error("Buffer retains recovery LSN after successful replacement")
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() after replacement = %d, want 0", frame.PinCount())
	}
}

func TestReplaceDirtyWithBlockRejectsUnassignedCleanOrPinnedFrames(t *testing.T) {
	t.Parallel()

	t.Run("unassigned", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		target, _ := appendDataBlock(t, files, 2)
		frame.MarkModified(TransactionID(3))
		before := frame.Contents().Snapshot()

		if err := frame.replaceDirtyWithBlock(target); err == nil {
			t.Fatal("replaceDirtyWithBlock() unassigned error = nil, want an assignment error")
		}
		if _, assigned := frame.Block(); assigned {
			t.Error("unassigned frame became assigned after rejected replacement")
		}
		if got := frame.Contents().Snapshot(); !bytes.Equal(got, before) {
			t.Error("Contents() changed after rejected unassigned replacement")
		}
		assertDirtyMetadata(t, frame, 3, 0, false)
	})

	t.Run("clean", func(t *testing.T) {
		frame, files, _, oldBlock := newAssignedBuffer(t)
		target, _ := appendDataBlock(t, files, 2)
		before := frame.Contents().Snapshot()

		if err := frame.replaceDirtyWithBlock(target); err == nil {
			t.Fatal("replaceDirtyWithBlock() clean error = nil, want a dirty-state error")
		}
		assertReassignmentState(t, frame, oldBlock, before)
	})

	t.Run("pinned", func(t *testing.T) {
		frame, files, _, oldBlock := newAssignedBuffer(t)
		target, _ := appendDataBlock(t, files, 2)
		if err := frame.Contents().SetInt(0, 77); err != nil {
			t.Fatalf("Contents().SetInt() error = %v", err)
		}
		frame.MarkModified(TransactionID(4))
		frame.pin()
		before := frame.Contents().Snapshot()

		if err := frame.replaceDirtyWithBlock(target); err == nil {
			t.Fatal("replaceDirtyWithBlock() pinned error = nil, want a pinned-state error")
		}
		assertReassignmentState(t, frame, oldBlock, before)
		assertDirtyMetadata(t, frame, 4, 0, false)
		if got := readBlockInt(t, files, oldBlock); got != 1 {
			t.Errorf("old disk block value after rejection = %d, want unchanged 1", got)
		}
	})
}

func TestReplaceDirtyWithBlockLogFailurePreservesDirtyOldFrame(t *testing.T) {
	t.Parallel()

	frame, files, _, oldBlock := newAssignedBuffer(t)
	target, _ := appendDataBlock(t, files, 2)
	if err := frame.Contents().SetInt(0, 88); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	frame.MarkModified(TransactionID(5))
	if err := frame.RecordRecoveryLSN(logmanager.LSN(99)); err != nil {
		t.Fatalf("RecordRecoveryLSN() error = %v", err)
	}
	before := frame.Contents().Snapshot()

	if err := frame.replaceDirtyWithBlock(target); err == nil {
		t.Fatal("replaceDirtyWithBlock() with future LSN error = nil, want a log flush error")
	}

	assertReassignmentState(t, frame, oldBlock, before)
	assertDirtyMetadata(t, frame, 5, 99, true)
	if got := readBlockInt(t, files, oldBlock); got != 1 {
		t.Errorf("old disk block value after failed log flush = %d, want unchanged 1", got)
	}
}

func TestReplaceDirtyWithBlockDataWriteFailurePreservesDirtyOldFrame(t *testing.T) {
	t.Parallel()

	database := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(database, 64)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	frame, err := New(files, logs)
	if err != nil {
		t.Fatalf("buffer.New() error = %v", err)
	}
	oldBlock, _ := appendDataBlock(t, files, 1)
	target, _ := appendDataBlock(t, files, 2)
	if err := frame.assignToBlock(oldBlock); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}
	if err := frame.Contents().SetInt(0, 66); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	lsn, err := logs.Append([]byte("data write must fail"))
	if err != nil {
		t.Fatalf("logs.Append() error = %v", err)
	}
	frame.MarkModified(TransactionID(6))
	if err := frame.RecordRecoveryLSN(lsn); err != nil {
		t.Fatalf("RecordRecoveryLSN() error = %v", err)
	}
	before := frame.Contents().Snapshot()
	dataPath := filepath.Join(database, "data.tbl")
	if err := os.Remove(dataPath); err != nil {
		t.Fatalf("Remove(data file) error = %v", err)
	}
	if err := os.Mkdir(dataPath, 0o700); err != nil {
		t.Fatalf("Mkdir(data path) error = %v", err)
	}

	if err := frame.replaceDirtyWithBlock(target); err == nil {
		t.Fatal("replaceDirtyWithBlock() data write error = nil, want an I/O error")
	}

	assertReassignmentState(t, frame, oldBlock, before)
	assertDirtyMetadata(t, frame, 6, lsn, true)
}

func TestReplaceDirtyWithBlockTargetReadFailureKeepsFlushedOldFrame(t *testing.T) {
	t.Parallel()

	frame, files, _, oldBlock := newAssignedBuffer(t)
	if err := frame.Contents().SetInt(0, 55); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	frame.MarkModified(TransactionID(7))
	before := frame.Contents().Snapshot()
	missing := storage.NewBlockID("missing.tbl", 0)

	if err := frame.replaceDirtyWithBlock(missing); err == nil {
		t.Fatal("replaceDirtyWithBlock(missing) error = nil, want a target read error")
	}

	assertReassignmentState(t, frame, oldBlock, before)
	if got := readBlockInt(t, files, oldBlock); got != 55 {
		t.Errorf("old disk block value after target read failure = %d, want flushed 55", got)
	}
	if _, modified := frame.ModifyingTransaction(); modified {
		t.Error("old frame remains dirty after its data page was written successfully")
	}
	if _, exists := frame.RecoveryLSN(); exists {
		t.Error("old frame retains recovery LSN after its data page was written successfully")
	}
}

func readBlockInt(t *testing.T, files *storagefile.Manager, block storage.BlockID) int32 {
	t.Helper()

	page, err := files.Read(block)
	if err != nil {
		t.Fatalf("Read(%v) error = %v", block, err)
	}
	value, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(0) from %v error = %v", block, err)
	}
	return value
}

func assertDirtyMetadata(t *testing.T, frame *Buffer, transaction TransactionID, lsn logmanager.LSN, hasLSN bool) {
	t.Helper()

	if got, modified := frame.ModifyingTransaction(); !modified || got != transaction {
		t.Errorf("ModifyingTransaction() = (%d, %t), want (%d, true)", got, modified, transaction)
	}
	if got, exists := frame.RecoveryLSN(); exists != hasLSN || exists && got != lsn {
		t.Errorf("RecoveryLSN() = (%d, %t), want (%d, %t)", got, exists, lsn, hasLSN)
	}
}
