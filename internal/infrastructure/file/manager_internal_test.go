package file

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type fakeBlockFile struct {
	readCount  int
	readErr    error
	writeCount int
	writeErr   error
	syncErr    error
}

func (f *fakeBlockFile) ReadAt(buffer []byte, _ int64) (int, error) {
	for i := range buffer {
		buffer[i] = byte(i)
	}
	return f.readCount, f.readErr
}

func (f *fakeBlockFile) WriteAt(_ []byte, _ int64) (int, error) {
	return f.writeCount, f.writeErr
}

func (f *fakeBlockFile) Sync() error { return f.syncErr }

func (f *fakeBlockFile) Close() error { return nil }

func TestManagerReadWrapsOpenAndReadFailures(t *testing.T) {
	openErr := errors.New("open failure")
	manager := &Manager{
		directory: "/database",
		blockSize: 4,
		openRead:  func(string) (blockFile, error) { return nil, openErr },
	}
	_, err := manager.Read(storage.NewBlockID("student.tbl", 0))
	if !errors.Is(err, openErr) {
		t.Fatalf("Read() error = %v, want wrapped open failure", err)
	}

	readErr := io.ErrUnexpectedEOF
	manager.openRead = func(string) (blockFile, error) {
		return &fakeBlockFile{readCount: 2, readErr: readErr}, nil
	}
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
		openWrite: func(string, int, os.FileMode) (blockFile, error) { return nil, openErr },
	}
	if err := manager.Write(block, page); !errors.Is(err, openErr) {
		t.Fatalf("Write() error = %v, want wrapped open failure", err)
	}

	writeErr := io.ErrShortWrite
	manager.openWrite = func(string, int, os.FileMode) (blockFile, error) {
		return &fakeBlockFile{writeCount: 2, writeErr: writeErr}, nil
	}
	if err := manager.Write(block, page); !errors.Is(err, writeErr) {
		t.Fatalf("Write() error = %v, want wrapped write failure", err)
	}

	syncErr := errors.New("sync failure")
	manager.openWrite = func(string, int, os.FileMode) (blockFile, error) {
		return &fakeBlockFile{writeCount: page.Capacity(), syncErr: syncErr}, nil
	}
	if err := manager.Write(block, page); !errors.Is(err, syncErr) {
		t.Fatalf("Write() error = %v, want wrapped sync failure", err)
	}
}
