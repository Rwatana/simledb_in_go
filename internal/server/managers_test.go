package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewManagersUsesConfiguredFileLogAndBufferManagers(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 2)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	if managers == nil {
		t.Fatal("NewManagers() = nil, want initialized managers")
	}
	if managers.Files == nil || managers.Logs == nil || managers.Buffers == nil {
		t.Fatal("NewManagers() returned a nil manager")
	}
	if managers.Files.BlockSize() != 64 {
		t.Errorf("file block size = %d, want 64", managers.Files.BlockSize())
	}
	if managers.Buffers.Capacity() != 2 {
		t.Errorf("buffer capacity = %d, want 2", managers.Buffers.Capacity())
	}
	if managers.Buffers.Available() != 2 {
		t.Errorf("available buffers = %d, want 2", managers.Buffers.Available())
	}
	length, err := managers.Files.Length("simpledb.log")
	if err != nil {
		t.Fatalf("log file length error = %v", err)
	}
	if length != 1 {
		t.Errorf("log file length = %d, want the initial block", length)
	}
	page, err := managers.Files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("read initial log block error = %v", err)
	}
	boundary, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("read log boundary error = %v", err)
	}
	if boundary != 64 {
		t.Errorf("log boundary = %d, want 64", boundary)
	}
}

func TestNewManagersReturnsNoManagersWhenFileManagerFails(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0o600); err != nil {
		t.Fatalf("write blocker file error = %v", err)
	}
	managers, err := NewManagers(filePath, 64, 2)
	if err == nil || !strings.Contains(err.Error(), "file manager") {
		t.Fatalf("NewManagers() error = %v, want file-manager context", err)
	}
	if managers != nil {
		t.Errorf("NewManagers() = %#v, want nil managers on failure", managers)
	}
}

func TestNewManagersReturnsNoManagersWhenLogManagerFails(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 3, 2)
	if err == nil || !strings.Contains(err.Error(), "log manager") {
		t.Fatalf("NewManagers() error = %v, want log-manager context", err)
	}
	if managers != nil {
		t.Errorf("NewManagers() = %#v, want nil managers on failure", managers)
	}
}

func TestNewManagersReturnsNoManagersWhenBufferManagerFails(t *testing.T) {
	t.Parallel()

	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 64, 0)
	if err == nil || !strings.Contains(err.Error(), "buffer manager") {
		t.Fatalf("NewManagers() error = %v, want buffer-manager context", err)
	}
	if managers != nil {
		t.Errorf("NewManagers() = %#v, want nil managers on failure", managers)
	}
}
