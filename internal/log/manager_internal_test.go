package log

import (
	"errors"
	"strings"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type readFailureFileManager struct {
	*storagefile.Manager
	err error
}

type shortPageFileManager struct {
	*storagefile.Manager
	page *storage.Page
}

type lengthFailureFileManager struct {
	*storagefile.Manager
	err error
}

func (f readFailureFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return nil, f.err
}

func (f shortPageFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return f.page, nil
}

func (f lengthFailureFileManager) Length(string) (int, error) {
	return 0, f.err
}

func TestNewManagerPreservesTailBlockReadFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	readErr := errors.New("tail read failed")

	manager, err := newManager(readFailureFileManager{Manager: files, err: readErr}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, readErr) {
		t.Fatalf("newManager() error = %v, want wrapped tail read failure", err)
	}
	for _, context := range []string{"create log manager for \"simpledb.log\"", "read tail block [file simpledb.log, block 0]"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %q, want context %q", err, context)
		}
	}
}

func TestNewManagerPreservesTailBoundaryReadFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	shortPage, err := storage.NewPage(3)
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}

	manager, err := newManager(shortPageFileManager{Manager: files, page: shortPage}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	for _, context := range []string{
		"create log manager for \"simpledb.log\"",
		"read tail boundary",
		"get integer at offset 0",
		"byte range [0:4] exceeds page capacity 3",
	} {
		if err == nil || !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %v, want context %q", err, context)
		}
	}
}

func TestNewManagerPreservesFileLengthFailure(t *testing.T) {
	t.Parallel()

	const fileName = "simpledb.log"
	_, files := newLogManager(t, 32)
	lengthErr := errors.New("file length failed")

	manager, err := newManager(lengthFailureFileManager{Manager: files, err: lengthErr}, fileName)
	if manager != nil {
		t.Errorf("newManager() manager = %v, want nil", manager)
	}
	if err == nil || !errors.Is(err, lengthErr) {
		t.Fatalf("newManager() error = %v, want wrapped file-length error", err)
	}
	for _, context := range []string{"create log manager for \"simpledb.log\"", "file length failed"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("newManager() error = %q, want context %q", err, context)
		}
	}
}
