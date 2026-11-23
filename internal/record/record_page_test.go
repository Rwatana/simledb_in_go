package record

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingRecordTransaction struct {
	pins    []storage.BlockID
	pinErr  error
	reads   []integerRead
	value   int32
	readErr error
}

type integerRead struct {
	block  storage.BlockID
	offset int
}

func (t *recordingRecordTransaction) Pin(block storage.BlockID) error {
	t.pins = append(t.pins, block)
	return t.pinErr
}

func (t *recordingRecordTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	t.reads = append(t.reads, integerRead{block: block, offset: offset})
	return t.value, t.readErr
}

func TestRecordPageGetIntReadsCalculatedFieldPosition(t *testing.T) {
	transaction := &recordingRecordTransaction{value: 42}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	got, err := page.GetInt(2, "id")
	if err != nil || got != 42 {
		t.Fatalf("GetInt(2, id) = (%d, %v), want (42, nil)", got, err)
	}
	wantRead := integerRead{block: block, offset: 20}
	if len(transaction.reads) != 1 || transaction.reads[0] != wantRead {
		t.Fatalf("GetInt calls = %v, want [%v]", transaction.reads, wantRead)
	}
}

func TestRecordPageGetIntReportsLookupAndReadFailures(t *testing.T) {
	readErr := errors.New("read failed")
	transaction := &recordingRecordTransaction{readErr: readErr}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if _, err := page.GetInt(1, "missing"); err == nil || len(transaction.reads) != 0 {
		t.Fatalf("GetInt(missing) = %v with reads %v, want lookup failure before read", err, transaction.reads)
	}
	_, err = page.GetInt(1, "id")
	if !errors.Is(err, readErr) || !strings.Contains(err.Error(), "slot 1 field \"id\"") || !strings.Contains(err.Error(), "offset 12") {
		t.Fatalf("GetInt read error = %v, want contextual wrapped failure", err)
	}
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
