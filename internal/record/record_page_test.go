package record

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingRecordTransaction struct {
	pins            []storage.BlockID
	pinErr          error
	blockSize       int
	blockSizeErr    error
	reads           []integerRead
	value           int32
	readValues      []int32
	readErr         error
	stringReads     []integerRead
	stringValue     string
	stringErr       error
	writes          []integerWrite
	writeErr        error
	stringWrites    []stringWrite
	stringWriteErr  error
	writeOperations []writeOperation
}

type writeOperation struct {
	kind      string
	offset    int32
	integer   int32
	text      string
	logUpdate bool
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

func (t *recordingRecordTransaction) BlockSize() (int, error) {
	return t.blockSize, t.blockSizeErr
}

func (t *recordingRecordTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	t.reads = append(t.reads, integerRead{block: block, offset: offset})
	if len(t.readValues) >= len(t.reads) {
		return t.readValues[len(t.reads)-1], t.readErr
	}
	return t.value, t.readErr
}

func (t *recordingRecordTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	t.stringReads = append(t.stringReads, integerRead{block: block, offset: offset})
	return t.stringValue, t.stringErr
}

func (t *recordingRecordTransaction) SetInt(block storage.BlockID, offset, value int32, logUpdate bool) error {
	t.writes = append(t.writes, integerWrite{block: block, offset: offset, value: value, logUpdate: logUpdate})
	t.writeOperations = append(t.writeOperations, writeOperation{kind: "int", offset: offset, integer: value, logUpdate: logUpdate})
	return t.writeErr
}

func (t *recordingRecordTransaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	t.stringWrites = append(t.stringWrites, stringWrite{block: block, offset: offset, value: value, logUpdate: logUpdate})
	t.writeOperations = append(t.writeOperations, writeOperation{kind: "string", offset: offset, text: value, logUpdate: logUpdate})
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

func TestRecordPageDeleteWritesEmptyFlagWithLogging(t *testing.T) {
	transaction := &recordingRecordTransaction{}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.Delete(2); err != nil {
		t.Fatalf("Delete(2) error = %v", err)
	}
	want := integerWrite{block: block, offset: 16, value: 0, logUpdate: true}
	if len(transaction.writes) != 1 || transaction.writes[0] != want {
		t.Fatalf("Delete calls = %v, want [%v]", transaction.writes, want)
	}
}

func TestRecordPageDeleteRejectsInvalidPositionAndWrapsFailures(t *testing.T) {
	writeErr := errors.New("delete failed")
	transaction := &recordingRecordTransaction{writeErr: writeErr}
	block := storage.NewBlockID("records.tbl", 3)
	page, err := NewRecordPage(transaction, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}
	if err := page.Delete(2); !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "offset 8") {
		t.Fatalf("Delete write error = %v, want contextual wrapped failure", err)
	}

	overflow := NewLayoutFromMetadata(&Schema{}, nil, math.MaxInt32)
	overflowPage, err := NewRecordPage(&recordingRecordTransaction{}, block, overflow)
	if err != nil {
		t.Fatalf("NewRecordPage(overflow) error = %v", err)
	}
	if err := overflowPage.Delete(2); err == nil {
		t.Fatal("Delete with overflowing position returned nil error")
	}
}

func TestRecordPageFormatInitializesCompleteSlotsWithoutLogging(t *testing.T) {
	transaction := &recordingRecordTransaction{blockSize: 34}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("code", 1)
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if err := page.Format(); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	want := []writeOperation{
		{kind: "int", offset: 0},
		{kind: "int", offset: 4},
		{kind: "string", offset: 8},
		{kind: "int", offset: 16},
		{kind: "int", offset: 20},
		{kind: "string", offset: 24},
	}
	if len(transaction.writeOperations) != len(want) {
		t.Fatalf("Format operations = %v, want %v", transaction.writeOperations, want)
	}
	for index := range want {
		if transaction.writeOperations[index] != want[index] {
			t.Fatalf("Format operation %d = %v, want %v", index, transaction.writeOperations[index], want[index])
		}
	}
}

func TestRecordPageFormatRejectsInvalidLayoutAndWrapsFailures(t *testing.T) {
	block := storage.NewBlockID("records.tbl", 3)
	invalidPage, err := NewRecordPage(&recordingRecordTransaction{blockSize: 32}, block, NewLayoutFromMetadata(&Schema{}, nil, 0))
	if err != nil {
		t.Fatalf("NewRecordPage(invalid layout) error = %v", err)
	}
	if err := invalidPage.Format(); err == nil {
		t.Fatal("Format() with zero slot size returned nil error")
	}

	blockSizeErr := errors.New("block size failed")
	blockSizePage, err := NewRecordPage(&recordingRecordTransaction{blockSizeErr: blockSizeErr}, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage(block size failure) error = %v", err)
	}
	if err := blockSizePage.Format(); !errors.Is(err, blockSizeErr) || !strings.Contains(err.Error(), block.String()) {
		t.Fatalf("Format() block size error = %v, want contextual wrapped failure", err)
	}

	writeErr := errors.New("format failed")
	transaction := &recordingRecordTransaction{blockSize: 8, writeErr: writeErr}
	writePage, err := NewRecordPage(transaction, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage(write failure) error = %v", err)
	}
	if err := writePage.Format(); !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "slot 0 flag at offset 0") {
		t.Fatalf("Format() write error = %v, want contextual wrapped failure", err)
	}
	if len(transaction.writeOperations) != 1 {
		t.Fatalf("Format() operations after failure = %v, want one", transaction.writeOperations)
	}

	stringWriteErr := errors.New("string format failed")
	stringTransaction := &recordingRecordTransaction{blockSize: 12, stringWriteErr: stringWriteErr}
	var schema Schema
	schema.AddVarcharField("code", 1)
	stringPage, err := NewRecordPage(stringTransaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage(string failure) error = %v", err)
	}
	if err := stringPage.Format(); !errors.Is(err, stringWriteErr) || !strings.Contains(err.Error(), "slot 0 field \"code\" at offset 4") {
		t.Fatalf("Format() string write error = %v, want contextual wrapped failure", err)
	}
	if len(stringTransaction.writeOperations) != 2 {
		t.Fatalf("Format() operations after string failure = %v, want flag and field", stringTransaction.writeOperations)
	}
}

func TestRecordPageNextAfterFindsOccupiedSlotWithinCompleteSlots(t *testing.T) {
	transaction := &recordingRecordTransaction{blockSize: 34, readValues: []int32{recordSlotEmpty, recordSlotEmpty, 1}}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	got, err := page.NextAfter(-1)
	if err != nil || got != 2 {
		t.Fatalf("NextAfter(-1) = (%d, %v), want (2, nil)", got, err)
	}
	wantReads := []integerRead{{block: block, offset: 0}, {block: block, offset: 8}, {block: block, offset: 16}}
	if len(transaction.reads) != len(wantReads) {
		t.Fatalf("NextAfter reads = %v, want %v", transaction.reads, wantReads)
	}
	for index := range wantReads {
		if transaction.reads[index] != wantReads[index] {
			t.Fatalf("NextAfter read %d = %v, want %v", index, transaction.reads[index], wantReads[index])
		}
	}

	emptyTransaction := &recordingRecordTransaction{blockSize: 34, readValues: []int32{recordSlotEmpty}}
	emptyPage, err := NewRecordPage(emptyTransaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage(empty) error = %v", err)
	}
	got, err = emptyPage.NextAfter(2)
	if err != nil || got != -1 {
		t.Fatalf("NextAfter(2) = (%d, %v), want (-1, nil)", got, err)
	}
	if len(emptyTransaction.reads) != 1 || emptyTransaction.reads[0].offset != 24 {
		t.Fatalf("NextAfter(2) reads = %v, want only complete slot offset 24", emptyTransaction.reads)
	}
}

func TestRecordPageNextAfterRejectsInvalidInputAndWrapsFailures(t *testing.T) {
	block := storage.NewBlockID("records.tbl", 3)
	invalidPage, err := NewRecordPage(&recordingRecordTransaction{blockSize: 32}, block, NewLayoutFromMetadata(&Schema{}, nil, 0))
	if err != nil {
		t.Fatalf("NewRecordPage(invalid layout) error = %v", err)
	}
	if _, err := invalidPage.NextAfter(-1); err == nil {
		t.Fatal("NextAfter() with zero slot size returned nil error")
	}

	page, err := NewRecordPage(&recordingRecordTransaction{blockSize: 32}, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}
	if _, err := page.NextAfter(-2); err == nil {
		t.Fatal("NextAfter(-2) returned nil error")
	}

	blockSizeErr := errors.New("block size failed")
	blockSizePage, err := NewRecordPage(&recordingRecordTransaction{blockSizeErr: blockSizeErr}, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage(block size failure) error = %v", err)
	}
	if _, err := blockSizePage.NextAfter(-1); !errors.Is(err, blockSizeErr) || !strings.Contains(err.Error(), block.String()) {
		t.Fatalf("NextAfter() block size error = %v, want contextual wrapped failure", err)
	}

	readErr := errors.New("flag read failed")
	readTransaction := &recordingRecordTransaction{blockSize: 8, readErr: readErr}
	readPage, err := NewRecordPage(readTransaction, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage(read failure) error = %v", err)
	}
	if _, err := readPage.NextAfter(-1); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "slot 0 flag at offset 0") {
		t.Fatalf("NextAfter() read error = %v, want contextual wrapped failure", err)
	}
}

func TestRecordPageNextEmptyAfterFindsEmptySlotWithoutWriting(t *testing.T) {
	transaction := &recordingRecordTransaction{blockSize: 34, readValues: []int32{recordSlotUsed, recordSlotEmpty}}
	block := storage.NewBlockID("records.tbl", 3)
	var schema Schema
	schema.AddIntegerField("id")
	page, err := NewRecordPage(transaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	got, err := page.NextEmptyAfter(-1)
	if err != nil || got != 1 {
		t.Fatalf("NextEmptyAfter(-1) = (%d, %v), want (1, nil)", got, err)
	}
	wantReads := []integerRead{{block: block, offset: 0}, {block: block, offset: 8}}
	if len(transaction.reads) != len(wantReads) {
		t.Fatalf("NextEmptyAfter reads = %v, want %v", transaction.reads, wantReads)
	}
	for index := range wantReads {
		if transaction.reads[index] != wantReads[index] {
			t.Fatalf("NextEmptyAfter read %d = %v, want %v", index, transaction.reads[index], wantReads[index])
		}
	}
	if len(transaction.writeOperations) != 0 {
		t.Fatalf("NextEmptyAfter writes = %v, want none", transaction.writeOperations)
	}

	fullTransaction := &recordingRecordTransaction{blockSize: 17, readValues: []int32{recordSlotUsed, recordSlotUsed}}
	fullPage, err := NewRecordPage(fullTransaction, block, NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordPage(full) error = %v", err)
	}
	got, err = fullPage.NextEmptyAfter(-1)
	if err != nil || got != -1 {
		t.Fatalf("NextEmptyAfter(-1) = (%d, %v), want (-1, nil)", got, err)
	}
	if len(fullTransaction.reads) != 2 {
		t.Fatalf("NextEmptyAfter full-page reads = %v, want two complete slots", fullTransaction.reads)
	}
}

func TestRecordPageNextEmptyAfterWrapsFlagReadFailure(t *testing.T) {
	readErr := errors.New("empty flag read failed")
	transaction := &recordingRecordTransaction{blockSize: 8, readErr: readErr}
	block := storage.NewBlockID("records.tbl", 3)
	page, err := NewRecordPage(transaction, block, NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewRecordPage() error = %v", err)
	}

	if _, err := page.NextEmptyAfter(-1); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "slot 0 flag at offset 0") {
		t.Fatalf("NextEmptyAfter() read error = %v, want contextual wrapped failure", err)
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
