package file

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewManagerCreatesDatabaseDirectory(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "database")
	manager, err := NewManager(directory, 32)
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
		if _, err := NewManager(filepath.Join(t.TempDir(), "invalid"), blockSize); err == nil {
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
	if err := manager.Write(storage.NewBlockID("student.tbl", 0), nil); err == nil {
		t.Error("Write() with nil Page error = nil, want an error")
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

func newManager(t *testing.T, directory string, blockSize int) *Manager {
	t.Helper()

	manager, err := NewManager(directory, blockSize)
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

func TestManagerReadWrapsOpenAndReadFailures(t *testing.T) {
	openErr := errors.New("open failure")
	manager := &Manager{
		directory: "/database",
		blockSize: 4,
		ops:       testFileOperations{openErr: openErr},
	}
	_, err := manager.Read(storage.NewBlockID("student.tbl", 0))
	if !errors.Is(err, openErr) {
		t.Fatalf("Read() error = %v, want wrapped open failure", err)
	}

	readErr := io.ErrUnexpectedEOF
	manager.ops = testFileOperations{readHandle: &fakeBlockFile{readCount: 2, readErr: readErr}}
	_, err = manager.Read(storage.NewBlockID("student.tbl", 0))
	if !errors.Is(err, readErr) {
		t.Fatalf("Read() error = %v, want wrapped read failure", err)
	}
	if err == nil || !strings.Contains(err.Error(), "read 2 of 4 bytes") {
		t.Errorf("Read() error = %v, want short-read context", err)
	}
}

func TestManagerWriteWrapsOpenWriteAndSyncFailures(t *testing.T) {
	page, err := storage.NewPage(4)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	block := storage.NewBlockID("student.tbl", 0)

	openErr := errors.New("open failure")
	manager := &Manager{
		directory: "/database",
		blockSize: 4,
		ops:       testFileOperations{openFileErr: openErr},
	}
	if err := manager.Write(block, page); !errors.Is(err, openErr) {
		t.Fatalf("Write() error = %v, want wrapped open failure", err)
	}

	writeErr := io.ErrShortWrite
	manager.ops = testFileOperations{writeHandle: &fakeBlockFile{writeCount: 2, writeErr: writeErr}}
	if err := manager.Write(block, page); !errors.Is(err, writeErr) {
		t.Fatalf("Write() error = %v, want wrapped write failure", err)
	}

	syncErr := errors.New("sync failure")
	manager.ops = testFileOperations{writeHandle: &fakeBlockFile{writeCount: page.Capacity(), syncErr: syncErr}}
	if err := manager.Write(block, page); !errors.Is(err, syncErr) {
		t.Fatalf("Write() error = %v, want wrapped sync failure", err)
	}
}

type testFileOperations struct {
	openErr     error
	openFileErr error
	readHandle  fileHandle
	writeHandle fileHandle
}

func (f testFileOperations) Open(string) (fileHandle, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.readHandle, nil
}
func (f testFileOperations) OpenFile(string, int, os.FileMode) (fileHandle, error) {
	if f.openFileErr != nil {
		return nil, f.openFileErr
	}
	return f.writeHandle, nil
}

type fakeBlockFile struct {
	readCount  int
	readErr    error
	writeCount int
	writeErr   error
	syncErr    error
}

func (f *fakeBlockFile) ReadAt(p []byte, _ int64) (int, error) {
	for i := range p {
		p[i] = byte(i)
	}
	return f.readCount, f.readErr
}
func (f *fakeBlockFile) WriteAt([]byte, int64) (int, error) { return f.writeCount, f.writeErr }
func (f *fakeBlockFile) Sync() error                        { return f.syncErr }
func (f *fakeBlockFile) Close() error                       { return nil }

func TestNewManagerRejectsInvalidDirectory(t *testing.T) {
	t.Parallel()

	if _, err := NewManager("", 16); err == nil {
		t.Fatal("NewManager(\"\", 16) error = nil, want an empty-directory error")
	}

	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := NewManager(filepath.Join(parentFile, "database"), 16); err == nil {
		t.Fatal("NewManager() error = nil, want a directory-creation error")
	}
}
