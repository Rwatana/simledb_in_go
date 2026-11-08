package log_test

import (
	"bytes"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewManagerInitializesTheFirstLogBlock(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	files := newFileManager(t, blockSize)
	const logFile = "simpledb.log"

	if _, err := logmanager.NewManager(files, logFile); err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	length, err := files.Length(logFile)
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if length != 1 {
		t.Fatalf("Length() = %d, want 1", length)
	}

	page, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	boundary, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(0) error = %v", err)
	}
	if boundary != blockSize {
		t.Errorf("initial boundary = %d, want %d", boundary, blockSize)
	}
	if got, want := page.Snapshot()[4:], make([]byte, blockSize-4); !bytes.Equal(got, want) {
		t.Errorf("bytes after boundary = %v, want zero-filled bytes", got)
	}
}

func TestNewManagerLoadsExistingTailWithoutAppending(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	files := newFileManager(t, blockSize)
	const logFile = "simpledb.log"
	if _, err := logmanager.NewManager(files, logFile); err != nil {
		t.Fatalf("first NewManager() error = %v", err)
	}
	before, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() before reopen error = %v", err)
	}

	if _, err := logmanager.NewManager(files, logFile); err != nil {
		t.Fatalf("second NewManager() error = %v", err)
	}
	length, err := files.Length(logFile)
	if err != nil {
		t.Fatalf("Length() after reopen error = %v", err)
	}
	if length != 1 {
		t.Fatalf("Length() after reopen = %d, want 1", length)
	}
	after, err := files.Read(storage.NewBlockID(logFile, 0))
	if err != nil {
		t.Fatalf("Read() after reopen error = %v", err)
	}
	if !bytes.Equal(after.Snapshot(), before.Snapshot()) {
		t.Error("reopening LogManager changed the existing tail block")
	}
}

func TestNewManagerRejectsInvalidInitialization(t *testing.T) {
	t.Parallel()

	for _, blockSize := range []int{1, 2, 3} {
		files := newFileManager(t, blockSize)
		if _, err := logmanager.NewManager(files, "simpledb.log"); err == nil {
			t.Errorf("NewManager() block size %d error = nil, want a boundary size error", blockSize)
		}
	}

	files := newFileManager(t, 32)
	if _, err := logmanager.NewManager(files, "../escape.log"); err == nil {
		t.Error("NewManager() path traversal error = nil, want an invalid file name error")
	}
}

func newFileManager(t *testing.T, blockSize int) *storagefile.Manager {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	return files
}
