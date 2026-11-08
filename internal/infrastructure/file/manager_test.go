package file

import (
	"bytes"
	"errors"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		t.Error("Write() with nil Page error = nil, want error")
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

func assertZeroBlock(t *testing.T, manager *Manager, block storage.BlockID) {
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

func TestManagerCoversFilesystemFailures(t *testing.T) {
	boom := errors.New("injected filesystem failure")
	validInfo := coverageFileInfo{size: 4}
	block := storage.NewBlockID("data", 0)
	page, err := storage.NewPage(4)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		run  func(*Manager) error
		ops  coverageFileOperations
	}{
		{
			name: "length stat failure",
			run:  func(manager *Manager) error { _, err := manager.Length("data"); return err },
			ops:  coverageFileOperations{statErr: boom},
		},
		{
			name: "length invalid file size",
			run:  func(manager *Manager) error { _, err := manager.Length("data"); return err },
			ops:  coverageFileOperations{statInfo: coverageFileInfo{size: 5}},
		},
		{
			name: "append open failure",
			run:  func(manager *Manager) error { _, err := manager.Append("data"); return err },
			ops:  coverageFileOperations{openFileErr: boom},
		},
		{
			name: "append stat failure",
			run:  func(manager *Manager) error { _, err := manager.Append("data"); return err },
			ops:  coverageFileOperations{file: &coverageFile{statErr: boom}},
		},
		{
			name: "append invalid file size",
			run:  func(manager *Manager) error { _, err := manager.Append("data"); return err },
			ops:  coverageFileOperations{file: &coverageFile{info: coverageFileInfo{size: 5}}},
		},
		{
			name: "append write failure",
			run:  func(manager *Manager) error { _, err := manager.Append("data"); return err },
			ops:  coverageFileOperations{file: &coverageFile{info: validInfo, writeErr: boom}},
		},
		{
			name: "append sync failure",
			run:  func(manager *Manager) error { _, err := manager.Append("data"); return err },
			ops:  coverageFileOperations{file: &coverageFile{info: validInfo, syncErr: boom}},
		},
		{
			name: "read open failure",
			run:  func(manager *Manager) error { _, err := manager.Read(block); return err },
			ops:  coverageFileOperations{openErr: boom},
		},
		{
			name: "read failure",
			run:  func(manager *Manager) error { _, err := manager.Read(block); return err },
			ops:  coverageFileOperations{file: &coverageFile{readErr: boom}},
		},
		{
			name: "write nil page",
			run:  func(manager *Manager) error { return manager.Write(block, nil) },
			ops:  coverageFileOperations{},
		},
		{
			name: "write open failure",
			run:  func(manager *Manager) error { return manager.Write(block, page) },
			ops:  coverageFileOperations{openFileErr: boom},
		},
		{
			name: "write failure",
			run:  func(manager *Manager) error { return manager.Write(block, page) },
			ops:  coverageFileOperations{file: &coverageFile{writeErr: boom}},
		},
		{
			name: "write sync failure",
			run:  func(manager *Manager) error { return manager.Write(block, page) },
			ops:  coverageFileOperations{file: &coverageFile{syncErr: boom}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{directory: t.TempDir(), blockSize: 4, ops: tt.ops}
			if err := tt.run(manager); err == nil {
				t.Fatal("operation error = nil, want injected failure")
			}
		})
	}
}

func TestManagerRejectsDirectoryCreationFailure(t *testing.T) {
	if _, err := NewManager("", 4); err == nil {
		t.Fatal("NewManager() error = nil, want empty-directory error")
	}
	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(filepath.Join(filePath, "database"), 4); err == nil {
		t.Fatal("NewManager() error = nil, want directory creation failure")
	}
}

func TestBlocksForSizeRejectsLimitOverflow(t *testing.T) {
	manager := &Manager{blockSize: 1}
	if _, err := manager.blocksForSizeWithLimit("data", 3, 2); err == nil {
		t.Fatal("blocksForSizeWithLimit() error = nil, want platform-limit error")
	}
}

type coverageFileOperations struct {
	statInfo    os.FileInfo
	statErr     error
	openErr     error
	openFileErr error
	file        *coverageFile
}

func (ops coverageFileOperations) Stat(string) (os.FileInfo, error) {
	return ops.statInfo, ops.statErr
}

func (ops coverageFileOperations) Open(string) (fileHandle, error) {
	if ops.openErr != nil {
		return nil, ops.openErr
	}
	return ops.fileForOperation(), nil
}

func (ops coverageFileOperations) OpenFile(string, int, os.FileMode) (fileHandle, error) {
	if ops.openFileErr != nil {
		return nil, ops.openFileErr
	}
	return ops.fileForOperation(), nil
}

func (ops coverageFileOperations) fileForOperation() *coverageFile {
	if ops.file == nil {
		return &coverageFile{info: coverageFileInfo{size: 0}}
	}
	return ops.file
}

type coverageFile struct {
	info     os.FileInfo
	statErr  error
	readErr  error
	writeErr error
	syncErr  error
}

func (file *coverageFile) Stat() (os.FileInfo, error) {
	if file.statErr != nil {
		return nil, file.statErr
	}
	if file.info == nil {
		return coverageFileInfo{}, nil
	}
	return file.info, nil
}

func (file *coverageFile) ReadAt(contents []byte, _ int64) (int, error) {
	if file.readErr != nil {
		return 0, file.readErr
	}
	return len(contents), nil
}

func (file *coverageFile) WriteAt(contents []byte, _ int64) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return len(contents), nil
}

func (file *coverageFile) Sync() error  { return file.syncErr }
func (file *coverageFile) Close() error { return nil }

type coverageFileInfo struct{ size int64 }

func (info coverageFileInfo) Name() string  { return "data" }
func (info coverageFileInfo) Size() int64   { return info.size }
func (coverageFileInfo) Mode() fs.FileMode  { return 0o600 }
func (coverageFileInfo) ModTime() time.Time { return time.Time{} }
func (coverageFileInfo) IsDir() bool        { return false }
func (coverageFileInfo) Sys() any           { return nil }

func TestManagerAppendPreservesFileStatFailure(t *testing.T) {
	directory := t.TempDir()
	manager, err := NewManager(directory, 16)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	wantErr := errors.New("stat unavailable")
	handle := &failedStatFile{err: wantErr}
	var openedPath string
	manager.openFile = func(path string, _ int, _ os.FileMode) (fileHandle, error) {
		openedPath = path
		return handle, nil
	}

	block, err := manager.Append("student.tbl")
	if block != (storage.BlockID{}) {
		t.Errorf("Append() block = %v, want zero BlockID", block)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Append() error = %v, want wrapped stat failure", err)
	}
	for _, context := range []string{"append block to \"student.tbl\"", "stat", filepath.Join(directory, "student.tbl")} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append() error = %q, want context %q", err, context)
		}
	}
	if handle.writeCalls != 0 || handle.syncCalls != 0 {
		t.Errorf("file operations = (%d writes, %d syncs), want none after Stat failure", handle.writeCalls, handle.syncCalls)
	}
	if handle.closeCalls != 1 {
		t.Errorf("close calls = %d, want 1 after Stat failure", handle.closeCalls)
	}
	if openedPath != filepath.Join(directory, "student.tbl") {
		t.Errorf("opened path = %q, want %q", openedPath, filepath.Join(directory, "student.tbl"))
	}
}

func TestManagerAppendPreservesFileSyncFailure(t *testing.T) {
	directory := t.TempDir()
	manager, err := NewManager(directory, 16)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	statSource, err := os.CreateTemp(t.TempDir(), "empty-file")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	info, err := statSource.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if err := statSource.Close(); err != nil {
		t.Fatalf("Close(stat source) error = %v", err)
	}

	wantErr := errors.New("sync unavailable")
	handle := &failedSyncFile{info: info, err: wantErr}
	var openedPath string
	manager.openFile = func(path string, _ int, _ os.FileMode) (fileHandle, error) {
		openedPath = path
		return handle, nil
	}

	block, err := manager.Append("student.tbl")
	if block != (storage.BlockID{}) {
		t.Errorf("Append() block = %v, want zero BlockID", block)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Append() error = %v, want wrapped sync failure", err)
	}
	for _, context := range []string{"append block 0", "student.tbl", "sync", openedPath} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append() error = %q, want context %q", err, context)
		}
	}
	if handle.writeCalls != 1 || handle.syncCalls != 1 || handle.closeCalls != 1 {
		t.Errorf("file operations = (%d writes, %d syncs, %d closes), want (1, 1, 1)", handle.writeCalls, handle.syncCalls, handle.closeCalls)
	}
	if len(handle.written) != 16 {
		t.Fatalf("written block size = %d, want 16", len(handle.written))
	}
	for offset, value := range handle.written {
		if value != 0 {
			t.Errorf("written byte at offset %d = %d, want zero", offset, value)
		}
	}
}

type failedStatFile struct {
	err        error
	writeCalls int
	syncCalls  int
	closeCalls int
}

func (f *failedStatFile) Stat() (os.FileInfo, error) {
	return nil, f.err
}

func (f *failedStatFile) WriteAt([]byte, int64) (int, error) {
	f.writeCalls++
	return 0, nil
}

func (f *failedStatFile) Sync() error {
	f.syncCalls++
	return nil
}

func (f *failedStatFile) Close() error {
	f.closeCalls++
	return nil
}

func TestManagerWritePreservesFileSyncFailure(t *testing.T) {
	directory := t.TempDir()
	manager, err := NewManager(directory, 16)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	wantErr := errors.New("sync unavailable")
	handle := &failedSyncFile{err: wantErr}
	var openedPath string
	manager.openFile = func(path string, _ int, _ os.FileMode) (fileHandle, error) {
		openedPath = path
		return handle, nil
	}
	page, err := storage.NewPage(16)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if err := page.SetInt(0, 42); err != nil {
		t.Fatalf("SetInt() error = %v", err)
	}
	block := storage.NewBlockID("student.tbl", 2)

	err = manager.Write(block, page)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Write() error = %v, want wrapped sync failure", err)
	}
	for _, context := range []string{block.String(), "sync", openedPath} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Write() error = %q, want context %q", err, context)
		}
	}
	if handle.writeCalls != 1 || handle.syncCalls != 1 || handle.closeCalls != 1 {
		t.Errorf("file operations = (%d writes, %d syncs, %d closes), want (1, 1, 1)", handle.writeCalls, handle.syncCalls, handle.closeCalls)
	}
	if !bytes.Equal(handle.written, page.Snapshot()) {
		t.Errorf("written bytes = %v, want complete page %v", handle.written, page.Snapshot())
	}
	if handle.writtenOffset != 32 {
		t.Errorf("write offset = %d, want 32", handle.writtenOffset)
	}
}

type failedSyncFile struct {
	info          os.FileInfo
	err           error
	written       []byte
	writtenOffset int64
	writeCalls    int
	syncCalls     int
	closeCalls    int
}

func (f *failedSyncFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

func (f *failedSyncFile) WriteAt(contents []byte, offset int64) (int, error) {
	f.writeCalls++
	f.written = append(f.written[:0], contents...)
	f.writtenOffset = offset
	return len(contents), nil
}

func (f *failedSyncFile) Sync() error {
	f.syncCalls++
	return f.err
}

func (f *failedSyncFile) Close() error {
	f.closeCalls++
	return nil
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

func (f testFileOperations) Stat(string) (os.FileInfo, error) { return nil, nil }
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

func (f *fakeBlockFile) Stat() (os.FileInfo, error) { return nil, nil }
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
