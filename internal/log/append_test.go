package log

import (
	"bytes"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestAppendStoresRecordsRightToLeftAndAllocatesLSN(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 32)
	first := []byte{0xde, 0xad, 0x00}

	firstLSN, err := manager.Append(first)
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	if firstLSN != 1 {
		t.Errorf("Append(first) LSN = %d, want 1", firstLSN)
	}
	first[0] = 0

	secondLSN, err := manager.Append([]byte{})
	if err != nil {
		t.Fatalf("Append(empty) error = %v", err)
	}
	if secondLSN != 2 {
		t.Errorf("Append(empty) LSN = %d, want 2", secondLSN)
	}

	boundary, err := manager.page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(boundary) error = %v", err)
	}
	if got, want := boundary, int32(21); got != want {
		t.Fatalf("boundary = %d, want %d", got, want)
	}
	second, err := manager.page.GetBytes(int(boundary))
	if err != nil {
		t.Fatalf("GetBytes(second) error = %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second record = %v, want empty", second)
	}
	firstStored, err := manager.page.GetBytes(int(boundary) + 4)
	if err != nil {
		t.Fatalf("GetBytes(first) error = %v", err)
	}
	if got, want := firstStored, []byte{0xde, 0xad, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("first record = %v, want %v", got, want)
	}
}

func TestAppendDoesNotChangeStateWhenRecordDoesNotFit(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 16)
	before := manager.page.Snapshot()

	if _, err := manager.Append(make([]byte, 9)); err == nil {
		t.Error("Append(oversized) error = nil, want a capacity error")
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, before) {
		t.Errorf("tail Page changed after rejected append: got %v, want %v", got, before)
	}

	lsn, err := manager.Append(make([]byte, 8))
	if err != nil {
		t.Fatalf("Append(exact fit) error = %v", err)
	}
	if lsn != 1 {
		t.Errorf("Append(exact fit) LSN = %d, want 1 after rejected append", lsn)
	}
}

func TestAppendDoesNotFlushTheTailPage(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 32)
	if _, err := manager.Append([]byte("pending")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	diskPage, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(log block) error = %v", err)
	}
	boundary, err := diskPage.GetInt(0)
	if err != nil {
		t.Fatalf("diskPage.GetInt(0) error = %v", err)
	}
	if boundary != 32 {
		t.Errorf("disk boundary = %d, want unchanged boundary 32", boundary)
	}
}

func newLogManager(t *testing.T, blockSize int) (*Manager, *storagefile.Manager) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	manager, err := NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, files
}
