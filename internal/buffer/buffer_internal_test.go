package buffer

import (
	"errors"
	"strings"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewWithPageFactoryPreservesPageCreationError(t *testing.T) {
	t.Parallel()

	files, err := storagefile.NewManager(t.TempDir(), 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	wantErr := errors.New("page allocation failed")

	frame, err := newWithPageFactory(files, logs, func(size int) (*storage.Page, error) {
		if size != 32 {
			t.Errorf("page factory size = %d, want 32", size)
		}
		return nil, wantErr
	})
	if frame != nil {
		t.Errorf("newWithPageFactory() frame = %v, want nil", frame)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("newWithPageFactory() error = %v, want wrapped %v", err, wantErr)
	}
	if err == nil || !strings.Contains(err.Error(), "create buffer contents") {
		t.Errorf("newWithPageFactory() error = %v, want page-creation context", err)
	}
}
