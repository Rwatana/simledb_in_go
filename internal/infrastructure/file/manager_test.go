package file_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewManagerCreatesDatabaseDirectory(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "database")
	manager, err := storagefile.NewManager(directory, 32)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", directory, err)
	}
	if !info.IsDir() {
		t.Errorf("database path is not a directory")
	}
	if got, want := manager.BlockSize(), 32; got != want {
		t.Errorf("BlockSize() = %d, want %d", got, want)
	}

	for _, blockSize := range []int{0, -1} {
		if _, err := storagefile.NewManager(filepath.Join(t.TempDir(), "invalid"), blockSize); err == nil {
			t.Errorf("NewManager() with block size %d error = nil, want an invalid size error", blockSize)
		}
	}
}

func TestManagerWritesAndReadsBlocks(t *testing.T) {
	t.Parallel()

	const blockSize = 32
	directory := filepath.Join(t.TempDir(), "database")
	manager := newManager(t, directory, blockSize)

	page0 := newStoragePage(t, blockSize)
	if err := page0.SetInt(0, 42); err != nil {
		t.Fatalf("page0.SetInt() error = %v", err)
	}
	if err := page0.SetBytes(8, []byte("first")); err != nil {
		t.Fatalf("page0.SetBytes() error = %v", err)
	}

	page2 := newStoragePage(t, blockSize)
	if err := page2.SetInt(0, -7); err != nil {
		t.Fatalf("page2.SetInt() error = %v", err)
	}
	if err := page2.SetBytes(8, []byte{0xde, 0xad, 0xbe, 0xef}); err != nil {
		t.Fatalf("page2.SetBytes() error = %v", err)
	}

	block0 := storage.NewBlockID("student.tbl", 0)
	block2 := storage.NewBlockID("student.tbl", 2)
	if err := manager.Write(block0, page0); err != nil {
		t.Fatalf("Write(%v) error = %v", block0, err)
	}
	if err := manager.Write(block2, page2); err != nil {
		t.Fatalf("Write(%v) error = %v", block2, err)
	}

	read0, err := manager.Read(block0)
	if err != nil {
		t.Fatalf("Read(%v) error = %v", block0, err)
	}
	read2, err := manager.Read(block2)
	if err != nil {
		t.Fatalf("Read(%v) error = %v", block2, err)
	}
	if !bytes.Equal(read0.Snapshot(), page0.Snapshot()) {
		t.Errorf("Read(%v) snapshot differs from written page", block0)
	}
	if !bytes.Equal(read2.Snapshot(), page2.Snapshot()) {
		t.Errorf("Read(%v) snapshot differs from written page", block2)
	}

	raw, err := os.ReadFile(filepath.Join(directory, "student.tbl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := len(raw), 3*blockSize; got != want {
		t.Fatalf("file length = %d, want %d", got, want)
	}
	if !bytes.Equal(raw[:blockSize], page0.Snapshot()) {
		t.Errorf("block 0 disk bytes differ from page 0")
	}
	if !bytes.Equal(raw[2*blockSize:3*blockSize], page2.Snapshot()) {
		t.Errorf("block 2 does not start at byte offset %d", 2*blockSize)
	}

	if err := read2.SetInt(0, 99); err != nil {
		t.Fatalf("read2.SetInt() error = %v", err)
	}
	original, err := page2.GetInt(0)
	if err != nil {
		t.Fatalf("page2.GetInt() error = %v", err)
	}
	if original != -7 {
		t.Errorf("written Page aliases read Page: original value = %d, want -7", original)
	}
}

func TestManagerRejectsInvalidBlockAccess(t *testing.T) {
	t.Parallel()

	const blockSize = 16
	directory := filepath.Join(t.TempDir(), "database")
	manager := newManager(t, directory, blockSize)
	page := newStoragePage(t, blockSize)

	negative := storage.NewBlockID("student.tbl", -1)
	if err := manager.Write(negative, page); err == nil {
		t.Error("Write() negative block error = nil, want an invalid block error")
	}
	if _, err := manager.Read(negative); err == nil {
		t.Error("Read() negative block error = nil, want an invalid block error")
	}

	wrongSize := newStoragePage(t, blockSize-1)
	if err := manager.Write(storage.NewBlockID("student.tbl", 0), wrongSize); err == nil {
		t.Error("Write() wrong Page capacity error = nil, want a size mismatch error")
	}
	if err := manager.Write(storage.NewBlockID("../escape.tbl", 0), page); err == nil {
		t.Error("Write() path traversal error = nil, want an invalid file name error")
	}

	if _, err := manager.Read(storage.NewBlockID("missing.tbl", 0)); err == nil {
		t.Error("Read() missing file error = nil, want an I/O error")
	}

	if err := os.WriteFile(filepath.Join(directory, "short.tbl"), make([]byte, blockSize-1), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := manager.Read(storage.NewBlockID("short.tbl", 0)); err == nil {
		t.Error("Read() short block error = nil, want a short read error")
	}

	overflow := storage.NewBlockID("student.tbl", math.MaxInt)
	if _, err := manager.Read(overflow); err == nil {
		t.Error("Read() offset overflow error = nil, want an overflow error")
	}
}

func TestManagerAppendsZeroFilledBlocksAndReportsLength(t *testing.T) {
	t.Parallel()

	const blockSize = 16
	directory := filepath.Join(t.TempDir(), "database")
	manager := newManager(t, directory, blockSize)
	const fileName = "student.tbl"

	length, err := manager.Length(fileName)
	if err != nil {
		t.Fatalf("Length() missing file error = %v", err)
	}
	if length != 0 {
		t.Fatalf("Length() missing file = %d, want 0", length)
	}

	block0, err := manager.Append(fileName)
	if err != nil {
		t.Fatalf("Append() first block error = %v", err)
	}
	if got, want := block0, storage.NewBlockID(fileName, 0); got != want {
		t.Fatalf("Append() first block = %v, want %v", got, want)
	}
	assertZeroBlock(t, manager, block0)

	page0 := newStoragePage(t, blockSize)
	if err := page0.SetInt(0, 42); err != nil {
		t.Fatalf("page0.SetInt() error = %v", err)
	}
	if err := manager.Write(block0, page0); err != nil {
		t.Fatalf("Write(%v) error = %v", block0, err)
	}

	for wantNumber := 1; wantNumber <= 2; wantNumber++ {
		block, err := manager.Append(fileName)
		if err != nil {
			t.Fatalf("Append() block %d error = %v", wantNumber, err)
		}
		if got, want := block, storage.NewBlockID(fileName, wantNumber); got != want {
			t.Errorf("Append() block = %v, want %v", got, want)
		}
		assertZeroBlock(t, manager, block)

		length, err := manager.Length(fileName)
		if err != nil {
			t.Fatalf("Length() after append %d error = %v", wantNumber, err)
		}
		if got, want := length, wantNumber+1; got != want {
			t.Errorf("Length() after append %d = %d, want %d", wantNumber, got, want)
		}
	}

	read0, err := manager.Read(block0)
	if err != nil {
		t.Fatalf("Read(%v) error = %v", block0, err)
	}
	value, err := read0.GetInt(0)
	if err != nil {
		t.Fatalf("read0.GetInt() error = %v", err)
	}
	if value != 42 {
		t.Errorf("block 0 value after later appends = %d, want 42", value)
	}

	info, err := os.Stat(filepath.Join(directory, fileName))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got, want := info.Size(), int64(3*blockSize); got != want {
		t.Errorf("file size = %d, want %d", got, want)
	}
}

func TestManagerRejectsPartialBlockFiles(t *testing.T) {
	t.Parallel()

	const blockSize = 16
	directory := filepath.Join(t.TempDir(), "database")
	manager := newManager(t, directory, blockSize)
	path := filepath.Join(directory, "partial.tbl")
	if err := os.WriteFile(path, make([]byte, blockSize+1), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := manager.Length("partial.tbl"); err == nil {
		t.Error("Length() partial block error = nil, want a corruption error")
	}
	if _, err := manager.Append("partial.tbl"); err == nil {
		t.Error("Append() partial block error = nil, want a corruption error")
	}
	if _, err := manager.Length("../escape.tbl"); err == nil {
		t.Error("Length() path traversal error = nil, want an invalid file name error")
	}
	if _, err := manager.Append("../escape.tbl"); err == nil {
		t.Error("Append() path traversal error = nil, want an invalid file name error")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got, want := info.Size(), int64(blockSize+1); got != want {
		t.Errorf("partial file size after rejected append = %d, want %d", got, want)
	}
}

func assertZeroBlock(t *testing.T, manager *storagefile.Manager, block storage.BlockID) {
	t.Helper()

	page, err := manager.Read(block)
	if err != nil {
		t.Fatalf("Read(%v) error = %v", block, err)
	}
	want := make([]byte, manager.BlockSize())
	if got := page.Snapshot(); !bytes.Equal(got, want) {
		t.Errorf("Read(%v) = %v, want a zero-filled block", block, got)
	}
}

func newManager(t *testing.T, directory string, blockSize int) *storagefile.Manager {
	t.Helper()

	manager, err := storagefile.NewManager(directory, blockSize)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func newStoragePage(t *testing.T, capacity int) *storage.Page {
	t.Helper()

	page, err := storage.NewPage(capacity)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	return page
}
