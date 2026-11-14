package record

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingRecordTransaction struct {
	pins   []storage.BlockID
	pinErr error
}

func (t *recordingRecordTransaction) Pin(block storage.BlockID) error {
	t.pins = append(t.pins, block)
	return t.pinErr
}

func (t *recordingRecordTransaction) GetInt(storage.BlockID, int) (int32, error) {
	return 0, nil
}

func TestNewRecordPagePinsTargetBlock(t *testing.T) {
	transaction := &recordingRecordTransaction{}
	block := storage.NewBlockID("records.tbl", 3)
	layout := NewLayout(&Schema{})

	page, err := NewRecordPage(transaction, block, layout)
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}
	if page == nil {
		t.Fatal("NewRecordPage() returned nil page")
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != block {
		t.Fatalf("Pin calls = %v, want [%v]", transaction.pins, block)
	}
}

func TestNewRecordPageReturnsPinFailure(t *testing.T) {
	pinErr := errors.New("pin failed")
	transaction := &recordingRecordTransaction{pinErr: pinErr}
	block := storage.NewBlockID("records.tbl", 4)

	page, err := NewRecordPage(transaction, block, NewLayout(&Schema{}))
	if page != nil || !errors.Is(err, pinErr) || !strings.Contains(err.Error(), block.String()) {
		t.Fatalf("NewRecordPage() = (%v, %v), want nil contextual pin failure", page, err)
	}
}
