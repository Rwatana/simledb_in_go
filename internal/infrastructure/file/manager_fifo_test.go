//go:build darwin || linux

package file_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagerAppendPreservesFIFOWriteFailure(t *testing.T) {
	t.Parallel()

	const (
		fileName  = "student.tbl"
		blockSize = 16
	)
	directory := filepath.Join(t.TempDir(), "database")
	manager := newManager(t, directory, blockSize)
	target := filepath.Join(directory, fileName)
	if err := syscall.Mkfifo(target, 0o600); err != nil {
		t.Fatalf("Mkfifo(%q) error = %v", target, err)
	}

	block, err := manager.Append(fileName)
	if block != (storage.BlockID{}) {
		t.Errorf("Append(%q) block = %v, want zero BlockID", fileName, block)
	}
	if err == nil || !errors.Is(err, syscall.ESPIPE) {
		t.Fatalf("Append(%q) error = %v, want a positioned-write ESPIPE cause", fileName, err)
	}
	for _, context := range []string{fileName, "wrote 0 of 16 bytes"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append(%q) error = %q, want context %q", fileName, err, context)
		}
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatalf("Stat(%q) error = %v, want the FIFO to remain", target, statErr)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("%q has mode %v, want a named pipe", target, info.Mode())
	}
}
