package buffer

import (
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestFlushWritesRecoveryLogBeforeDirtyPageAndClearsMetadata(t *testing.T) {
	t.Parallel()

	frame, files, logs, block := newAssignedBuffer(t)
	if err := frame.Contents().SetInt(0, 99); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	lsn, err := logs.Append([]byte("update data.tbl block 0"))
	if err != nil {
		t.Fatalf("logs.Append() error = %v", err)
	}
	frame.MarkModified(TransactionID(7))
	if err := frame.RecordRecoveryLSN(lsn); err != nil {
		t.Fatalf("RecordRecoveryLSN() error = %v", err)
	}

	if err := frame.flush(); err != nil {
		t.Fatalf("flush() error = %v", err)
	}

	diskData, err := files.Read(block)
	if err != nil {
		t.Fatalf("Read(data block) error = %v", err)
	}
	value, err := diskData.GetInt(0)
	if err != nil {
		t.Fatalf("diskData.GetInt() error = %v", err)
	}
	if value != 99 {
		t.Errorf("disk data value = %d, want 99", value)
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
		t.Error("log boundary is unchanged after dirty Page flush, want recovery record durable first")
	}
	if transaction, modified := frame.ModifyingTransaction(); modified {
		t.Errorf("ModifyingTransaction() after flush = (%d, true), want (_, false)", transaction)
	}
	if savedLSN, exists := frame.RecoveryLSN(); exists {
		t.Errorf("RecoveryLSN() after flush = (%d, true), want (_, false)", savedLSN)
	}
}

func TestFlushWritesDirtyPageWithoutARecoveryLSN(t *testing.T) {
	t.Parallel()

	frame, files, _, block := newAssignedBuffer(t)
	if err := frame.Contents().SetInt(0, 23); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	frame.MarkModified(TransactionID(8))

	if err := frame.flush(); err != nil {
		t.Fatalf("flush() error = %v", err)
	}
	diskData, err := files.Read(block)
	if err != nil {
		t.Fatalf("Read(data block) error = %v", err)
	}
	value, err := diskData.GetInt(0)
	if err != nil {
		t.Fatalf("diskData.GetInt() error = %v", err)
	}
	if value != 23 {
		t.Errorf("disk data value = %d, want 23", value)
	}
	if _, modified := frame.ModifyingTransaction(); modified {
		t.Error("Buffer remains dirty after successful data-only flush")
	}
}

func TestFlushFailurePreservesDirtyStateAndDiskPage(t *testing.T) {
	t.Parallel()

	frame, files, _, block := newAssignedBuffer(t)
	if err := frame.Contents().SetInt(0, 77); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	frame.MarkModified(TransactionID(9))
	if err := frame.RecordRecoveryLSN(logmanager.LSN(99)); err != nil {
		t.Fatalf("RecordRecoveryLSN() error = %v", err)
	}

	if err := frame.flush(); err == nil {
		t.Fatal("flush() with future recovery LSN error = nil, want a log flush error")
	}
	diskData, err := files.Read(block)
	if err != nil {
		t.Fatalf("Read(data block) error = %v", err)
	}
	value, err := diskData.GetInt(0)
	if err != nil {
		t.Fatalf("diskData.GetInt() error = %v", err)
	}
	if value != 1 {
		t.Errorf("disk data value after failed log flush = %d, want unchanged 1", value)
	}
	if transaction, modified := frame.ModifyingTransaction(); !modified || transaction != 9 {
		t.Errorf("ModifyingTransaction() after failed flush = (%d, %t), want (9, true)", transaction, modified)
	}
	if lsn, exists := frame.RecoveryLSN(); !exists || lsn != 99 {
		t.Errorf("RecoveryLSN() after failed flush = (%d, %t), want (99, true)", lsn, exists)
	}
}

func TestFlushNoOpsForCleanBufferAndRejectsUnassignedDirtyBuffer(t *testing.T) {
	t.Parallel()

	clean := newBufferForTest(t)
	if err := clean.flush(); err != nil {
		t.Errorf("clean flush() error = %v, want nil", err)
	}

	dirty := newBufferForTest(t)
	dirty.MarkModified(TransactionID(1))
	if err := dirty.flush(); err == nil {
		t.Error("unassigned dirty flush() error = nil, want an assignment error")
	}
	if transaction, modified := dirty.ModifyingTransaction(); !modified || transaction != 1 {
		t.Errorf("dirty state after rejected flush = (%d, %t), want (1, true)", transaction, modified)
	}
}

func newAssignedBuffer(t *testing.T) (*Buffer, *storagefile.Manager, *logmanager.Manager, storage.BlockID) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 64)
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
	block, err := files.Append("data.tbl")
	if err != nil {
		t.Fatalf("Append(data block) error = %v", err)
	}
	page, err := storage.NewPage(files.BlockSize())
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}
	if err := page.SetInt(0, 1); err != nil {
		t.Fatalf("page.SetInt() error = %v", err)
	}
	if err := files.Write(block, page); err != nil {
		t.Fatalf("Write(initial data) error = %v", err)
	}
	if err := frame.assignToBlock(block); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}
	return frame, files, logs, block
}
