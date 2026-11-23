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

func (f readFailureFileManager) Read(storage.BlockID) (*storage.Page, error) {
	return nil, f.err
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
