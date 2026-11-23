//go:build darwin || linux

package log_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

func TestIteratorReturnsTailFlushFailure(t *testing.T) {
	t.Parallel()

	const (
		blockSize = 64
		fileName  = "simpledb.log"
	)
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	manager, err := logmanager.NewManager(files, fileName)
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	if _, err := manager.Append([]byte("pending")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	path := filepath.Join(directory, fileName)
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(%q) error = %v", path, err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("Mkfifo(%q) error = %v", path, err)
	}

	iterator, err := manager.Iterator()
	if iterator != nil {
		t.Errorf("Iterator() = %v, want nil after tail flush failure", iterator)
	}
	if err == nil || !errors.Is(err, syscall.ESPIPE) {
		t.Fatalf("Iterator() error = %v, want wrapped ESPIPE", err)
	}
	for _, context := range []string{"create iterator", "flush current log tail", fileName} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Iterator() error = %q, want context %q", err, context)
		}
	}
}
