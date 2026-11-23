package transaction

import (
	"errors"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestSetIntRecoveryLSNPreservesRecorderFailure(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 4)
	wantErr := errors.New("recovery LSN rejected")
	var gotLSN logmanager.LSN
	err := recordRecoveryLSN(func(lsn logmanager.LSN) error {
		gotLSN = lsn
		return wantErr
	}, 17, "set integer", block, 8)

	if gotLSN != 17 {
		t.Errorf("recorded LSN = %d, want 17", gotLSN)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("recordRecoveryLSN() error = %v, want wrapped recorder failure", err)
	}
	for _, context := range []string{"set integer", block.String(), "offset 8", "LSN 17"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("recordRecoveryLSN() error = %q, want context %q", err, context)
		}
	}
}
