package buffer_test

import (
	"bytes"
	"path/filepath"
	"testing"

	buffer "github.com/Rwatana/simledb_in_go/internal/buffer"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

func TestNewCreatesAnUnassignedZeroFilledFrame(t *testing.T) {
	t.Parallel()

	files, logs := newDependencies(t, 32)
	frame, err := buffer.New(files, logs)
	if err != nil {
		t.Fatalf("buffer.New() error = %v", err)
	}

	if _, assigned := frame.Block(); assigned {
		t.Error("Block() assigned = true, want a fresh unassigned frame")
	}
	contents := frame.Contents()
	if contents.Capacity() != 32 {
		t.Errorf("Contents().Capacity() = %d, want 32", contents.Capacity())
	}
	if got, want := contents.Snapshot(), make([]byte, 32); !bytes.Equal(got, want) {
		t.Errorf("fresh contents = %v, want zero-filled Page", got)
	}

	if err := contents.SetInt(0, 42); err != nil {
		t.Fatalf("Contents().SetInt() error = %v", err)
	}
	value, err := frame.Contents().GetInt(0)
	if err != nil {
		t.Fatalf("Contents().GetInt() error = %v", err)
	}
	if value != 42 {
		t.Errorf("Contents().GetInt() = %d, want caller-written value 42", value)
	}
}

func TestNewRejectsNilDependencies(t *testing.T) {
	t.Parallel()

	files, logs := newDependencies(t, 32)
	if _, err := buffer.New(nil, logs); err == nil {
		t.Error("buffer.New(nil, logs) error = nil, want a FileManager dependency error")
	}
	if _, err := buffer.New(files, nil); err == nil {
		t.Error("buffer.New(files, nil) error = nil, want a LogManager dependency error")
	}
}

func newDependencies(t *testing.T, blockSize int) (*storagefile.Manager, *logmanager.Manager) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), blockSize)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	return files, logs
}
