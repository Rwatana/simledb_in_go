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

func TestNewManagerPreservesInitialBlockAppendFailure(t *testing.T) {
	t.Parallel()

	const (
		blockSize = 16
		fileName  = "simpledb.log"
	)
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	target := filepath.Join(directory, fileName)
	if err := syscall.Mkfifo(target, 0o600); err != nil {
		t.Fatalf("Mkfifo(%q) error = %v", target, err)
	}

	manager, err := logmanager.NewManager(files, fileName)
	if manager != nil {
		t.Errorf("NewManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, syscall.ESPIPE) {
		t.Fatalf("NewManager() error = %v, want initial append ESPIPE", err)
	}
	for _, context := range []string{fileName, "append first block"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("NewManager() error = %q, want context %q", err, context)
		}
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatalf("Stat(%q) error = %v, want FIFO to remain", target, statErr)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("%q mode = %v, want a named pipe", target, info.Mode())
	}
}
