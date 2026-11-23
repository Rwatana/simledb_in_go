package log

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
)

func TestAppendPreservesRolloverBlockAppendFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	directory := filepath.Join(t.TempDir(), "database")
	files, err := storagefile.NewManager(directory, 64)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	manager, err := NewManager(files, fileName)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if lsn, err := manager.Append(make([]byte, 56)); err != nil || lsn != 1 {
		t.Fatalf("Append(full-tail record) = (%d, %v), want (1, nil)", lsn, err)
	}
	if err := manager.Flush(1); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	beforePage := manager.page.Snapshot()
	beforeBlock := manager.block

	path := filepath.Join(directory, fileName)
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(%q) error = %v", path, err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", path, err)
	}

	lsn, err := manager.Append([]byte("x"))
	if lsn != 0 {
		t.Errorf("Append(rollover) LSN = %d, want 0 after block append failure", lsn)
	}
	if err == nil || !errors.Is(err, syscall.EISDIR) {
		t.Fatalf("Append(rollover) error = %v, want wrapped EISDIR", err)
	}
	for _, context := range []string{
		"append log record of 1 bytes",
		"roll over full tail [file simpledb.log, block 0]",
		"append new tail block",
	} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Append(rollover) error = %q, want context %q", err, context)
		}
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, beforePage) {
		t.Errorf("tail Page changed after failed rollover: got %v, want %v", got, beforePage)
	}
	if manager.block != beforeBlock {
		t.Errorf("tail block = %v, want unchanged %v", manager.block, beforeBlock)
	}
	if manager.latestLSN != 1 {
		t.Errorf("latest LSN = %d, want unchanged value 1", manager.latestLSN)
	}
}
