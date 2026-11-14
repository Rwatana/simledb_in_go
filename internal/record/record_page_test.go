package record

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingRecordTransaction struct {
	pins           []storage.BlockID
	pinErr         error
	reads          []integerRead
	value          int32
	readErr        error
	stringReads    []integerRead
	stringValue    string
	stringErr      error
	writes         []integerWrite
	writeErr       error
	stringWrites   []stringWrite
	stringWriteErr error
}

type integerRead struct {
	block  storage.BlockID
	offset int
}

type integerWrite struct {
	block     storage.BlockID
	offset    int32
	value     int32
	logUpdate bool
}

type stringWrite struct {
	block     storage.BlockID
	offset    int32
	value     string
	logUpdate bool
}

func (t *recordingRecordTransaction) Pin(block storage.BlockID) error {
	t.pins = append(t.pins, block)
	return t.pinErr
}

func (t *recordingRecordTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	t.reads = append(t.reads, integerRead{block: block, offset: offset})
	return t.value, t.readErr
}

func (t *recordingRecordTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	t.stringReads = append(t.stringReads, integerRead{block: block, offset: offset})
	return t.stringValue, t.stringErr
}

func (t *recordingRecordTransaction) SetInt(block storage.BlockID, offset, value int32, logUpdate bool) error {
	t.writes = append(t.writes, integerWrite{block: block, offset: offset, value: value, logUpdate: logUpdate})
	return t.writeErr
}

func (t *recordingRecordTransaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	t.stringWrites = append(t.stringWrites, stringWrite{block: block, offset: offset, value: value, logUpdate: logUpdate})
	return t.stringWriteErr
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

func TestRecordPageGetStringReadsCalculatedFieldPosition(t *testing.T) {
	transaction := &recordingRecordTransaction{stringValue: "東京"}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 20)
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	got, err := page.GetString(2, "name")
	if err != nil || got != "東京" {
		t.Fatalf("GetString(2, name) = (%q, %v), want (東京, nil)", got, err)
	}
	wantRead := integerRead{block: block, offset: 192}
	if len(transaction.stringReads) != 1 || transaction.stringReads[0] != wantRead {
		t.Fatalf("GetString calls = %v, want [%v]", transaction.stringReads, wantRead)
	}
}

func TestRecordPageGetStringReportsLookupAndReadFailures(t *testing.T) {
	readErr := errors.New("string read failed")
	transaction := &recordingRecordTransaction{stringErr: readErr}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddVarcharField("name", 20)
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if _, err := page.GetString(1, "missing"); err == nil || len(transaction.stringReads) != 0 {
		t.Fatalf("GetString(missing) = %v with reads %v, want lookup failure before read", err, transaction.stringReads)
	}
	_, err = page.GetString(1, "name")
	if !errors.Is(err, readErr) || !strings.Contains(err.Error(), "slot 1 field \"name\"") || !strings.Contains(err.Error(), "offset 92") {
		t.Fatalf("GetString read error = %v, want contextual wrapped failure", err)
	}
}

func TestRecordPageSetIntWritesCalculatedFieldPositionWithLogging(t *testing.T) {
	transaction := &recordingRecordTransaction{}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.SetInt(2, "id", 42); err != nil {
		t.Fatalf("SetInt(2, id) error = %v", err)
	}
	want := integerWrite{block: block, offset: 20, value: 42, logUpdate: true}
	if len(transaction.writes) != 1 || transaction.writes[0] != want {
		t.Fatalf("SetInt calls = %v, want [%v]", transaction.writes, want)
	}
}

func TestRecordPageSetIntRejectsInvalidPositionAndWrapsFailures(t *testing.T) {
	writeErr := errors.New("write failed")
	transaction := &recordingRecordTransaction{writeErr: writeErr}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.SetInt(1, "missing", 1); err == nil || len(transaction.writes) != 0 {
		t.Fatalf("SetInt(missing) = %v with writes %v, want lookup failure", err, transaction.writes)
	}
	if err := page.SetInt(1, "id", 1); !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "offset 12") {
		t.Fatalf("SetInt write error = %v, want contextual wrapped failure", err)
	}

	overflow := NewLayoutFromMetadata(&schema, map[string]int{"id": math.MaxInt32}, 1)
	overflowPage, err := NewRecordPage(&recordingRecordTransaction{}, block, overflow)
	if err != nil {
		t.Fatalf("NewRecordPage(overflow) error = %v", err)
	}
	if err := overflowPage.SetInt(1, "id", 1); err == nil {
		t.Fatal("SetInt with overflowing position returned nil error")
	}
}

func TestRecordPageSetStringWritesUTF8AtCalculatedPositionWithLogging(t *testing.T) {
	transaction := &recordingRecordTransaction{}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddVarcharField("name", 20)
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.SetString(2, "name", "東京"); err != nil {
		t.Fatalf("SetString(2, name) error = %v", err)
	}
	want := stringWrite{block: block, offset: 180, value: "東京", logUpdate: true}
	if len(transaction.stringWrites) != 1 || transaction.stringWrites[0] != want {
		t.Fatalf("SetString calls = %v, want [%v]", transaction.stringWrites, want)
	}
}

func TestRecordPageSetStringRejectsInvalidPositionAndWrapsFailures(t *testing.T) {
	writeErr := errors.New("string write failed")
	transaction := &recordingRecordTransaction{stringWriteErr: writeErr}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddVarcharField("name", 20)
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.SetString(1, "missing", "x"); err == nil || len(transaction.stringWrites) != 0 {
		t.Fatalf("SetString(missing) = %v with writes %v, want lookup failure", err, transaction.stringWrites)
	}
	if err := page.SetString(1, "name", "x"); !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "offset 92") {
		t.Fatalf("SetString write error = %v, want contextual wrapped failure", err)
	}

	overflow := NewLayoutFromMetadata(&schema, map[string]int{"name": math.MaxInt32}, 1)
	overflowPage, err := NewRecordPage(&recordingRecordTransaction{}, block, overflow)
	if err != nil {
		t.Fatalf("NewRecordPage(overflow) error = %v", err)
	}
	if err := overflowPage.SetString(1, "name", "x"); err == nil {
		t.Fatal("SetString with overflowing position returned nil error")
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
