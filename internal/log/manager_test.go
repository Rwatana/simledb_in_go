package log_test

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestNewManagerRejectsBlockSizeOutsideBoundaryCapacity(t *testing.T) {
	t.Parallel()

	tooLarge := int64(math.MaxInt32) + 1
	if strconv.IntSize <= 32 {
		t.Skip("block size cannot exceed the four-byte boundary on 32-bit platforms")
	}
	blockSize := int(tooLarge)
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}

	manager, err := logmanager.NewManager(files, "oversized.log")
	if manager != nil {
		t.Errorf("NewManager() manager = %v, want nil", manager)
	}
	if err == nil || !strings.Contains(err.Error(), "oversized.log") || !strings.Contains(err.Error(), "four-byte boundary capacity") {
		t.Fatalf("NewManager() error = %v, want file-specific boundary-capacity error", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "oversized.log")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("oversized log file stat error = %v, want no file created", statErr)
	}
}

func TestNewManagerRejectsNilFileManager(t *testing.T) {
	t.Parallel()

	manager, err := logmanager.NewManager(nil, "simpledb.log")
	if manager != nil {
		t.Errorf("NewManager() manager = %v, want nil", manager)
	}
	if err == nil || !strings.Contains(err.Error(), `"simpledb.log"`) || !strings.Contains(err.Error(), "file manager must not be nil") {
		t.Errorf("NewManager() error = %v, want file-specific nil dependency context", err)
	}
}
