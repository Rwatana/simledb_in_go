package file

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagerAppendPreservesFileStatFailure(t *testing.T) {
	directory := t.TempDir()
	manager, err := NewManager(directory, 16)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	wantErr := errors.New("stat unavailable")
	handle := &failedStatFile{err: wantErr}
	var openedPath string
	manager.openFile = func(path string, _ int, _ os.FileMode) (blockFile, error) {
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
	manager.openFile = func(path string, _ int, _ os.FileMode) (blockFile, error) {
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

type failedSyncFile struct {
	info       os.FileInfo
	err        error
	written    []byte
	writeCalls int
	syncCalls  int
	closeCalls int
}

func (f *failedSyncFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

func (f *failedSyncFile) WriteAt(contents []byte, _ int64) (int, error) {
	f.writeCalls++
	f.written = append(f.written[:0], contents...)
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
