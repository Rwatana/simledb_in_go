package record

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingTableTransaction struct {
	recordingRecordTransaction
	sizeCalls   []string
	size        int
	sizeErr     error
	appendCalls []string
	appendBlock storage.BlockID
	appendErr   error
	unpinCalls  []storage.BlockID
	unpinErr    error
}

func (t *recordingTableTransaction) Size(fileName string) (int, error) {
	t.sizeCalls = append(t.sizeCalls, fileName)
	return t.size, t.sizeErr
}

func (t *recordingTableTransaction) Append(fileName string) (storage.BlockID, error) {
	t.appendCalls = append(t.appendCalls, fileName)
	return t.appendBlock, t.appendErr
}

func (t *recordingTableTransaction) Unpin(block storage.BlockID) error {
	t.unpinCalls = append(t.unpinCalls, block)
	return t.unpinErr
}

func TestNewTableScanPinsFirstBlockOfExistingTable(t *testing.T) {
	transaction := &recordingTableTransaction{size: 3}
	layout := NewLayout(&Schema{})

	scan, err := NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if scan == nil {
		t.Fatal("NewTableScan() returned nil scan")
	}
	if len(transaction.sizeCalls) != 1 || transaction.sizeCalls[0] != "student.tbl" {
		t.Fatalf("Size calls = %v, want [student.tbl]", transaction.sizeCalls)
	}
	wantBlock := storage.NewBlockID("student.tbl", 0)
	if len(transaction.pins) != 1 || transaction.pins[0] != wantBlock {
		t.Fatalf("Pin calls = %v, want [%v]", transaction.pins, wantBlock)
	}
	if scan.fileName != "student.tbl" || scan.layout != layout || scan.currentSlot != -1 || scan.recordPage.Block() != wantBlock {
		t.Fatalf("scan state = %+v, want first block before slot zero", scan)
	}
}

func TestNewTableScanReportsSizeAndPinFailures(t *testing.T) {
	sizeErr := errors.New("size failed")
	sizeTransaction := &recordingTableTransaction{sizeErr: sizeErr}
	if scan, err := NewTableScan(sizeTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, sizeErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NewTableScan(size failure) = (%v, %v), want contextual failure", scan, err)
	}

	pinErr := errors.New("pin failed")
	pinTransaction := &recordingTableTransaction{size: 1, recordingRecordTransaction: recordingRecordTransaction{pinErr: pinErr}}
	if scan, err := NewTableScan(pinTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, pinErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NewTableScan(pin failure) = (%v, %v), want contextual failure", scan, err)
	}
}

func TestNewTableScanAppendsPinsAndFormatsEmptyTable(t *testing.T) {
	block := storage.NewBlockID("student.tbl", 0)
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{blockSize: 16},
		appendBlock:                block,
	}
	var schema Schema
	schema.AddIntegerField("id")
	layout := NewLayout(&schema)

	scan, err := NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan(empty) error = %v", err)
	}
	if len(transaction.appendCalls) != 1 || transaction.appendCalls[0] != "student.tbl" {
		t.Fatalf("Append calls = %v, want [student.tbl]", transaction.appendCalls)
	}
	if len(transaction.pins) != 1 || transaction.pins[0] != block {
		t.Fatalf("Pin calls = %v, want [%v]", transaction.pins, block)
	}
	wantWrites := []integerWrite{
		{block: block, offset: 0, value: recordSlotEmpty, logUpdate: false},
		{block: block, offset: 4, value: 0, logUpdate: false},
		{block: block, offset: 8, value: recordSlotEmpty, logUpdate: false},
		{block: block, offset: 12, value: 0, logUpdate: false},
	}
	if len(transaction.writes) != len(wantWrites) {
		t.Fatalf("format writes = %v, want %v", transaction.writes, wantWrites)
	}
	for index := range wantWrites {
		if transaction.writes[index] != wantWrites[index] {
			t.Fatalf("format write %d = %v, want %v", index, transaction.writes[index], wantWrites[index])
		}
	}
	if scan.currentSlot != -1 || scan.recordPage.Block() != block {
		t.Fatalf("scan state = %+v, want appended block before slot zero", scan)
	}
}

func TestNewTableScanReportsEmptyTableAppendPinAndFormatFailures(t *testing.T) {
	block := storage.NewBlockID("student.tbl", 0)
	appendErr := errors.New("append failed")
	appendTransaction := &recordingTableTransaction{appendErr: appendErr}
	if scan, err := NewTableScan(appendTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, appendErr) || len(appendTransaction.pins) != 0 {
		t.Fatalf("NewTableScan(append failure) = (%v, %v), pins %v", scan, err, appendTransaction.pins)
	}

	pinErr := errors.New("empty pin failed")
	pinTransaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{pinErr: pinErr},
		appendBlock:                block,
	}
	if scan, err := NewTableScan(pinTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, pinErr) {
		t.Fatalf("NewTableScan(empty pin failure) = (%v, %v), want contextual failure", scan, err)
	}

	formatErr := errors.New("format failed")
	formatTransaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{blockSize: 8, writeErr: formatErr},
		appendBlock:                block,
	}
	if scan, err := NewTableScan(formatTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, formatErr) || len(formatTransaction.writes) != 1 {
		t.Fatalf("NewTableScan(format failure) = (%v, %v), writes %v", scan, err, formatTransaction.writes)
	}
}

func TestTableScanCloseUnpinsCurrentBlockOnce(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	wantBlock := storage.NewBlockID("student.tbl", 0)
	if len(transaction.unpinCalls) != 1 || transaction.unpinCalls[0] != wantBlock {
		t.Fatalf("Unpin calls = %v, want [%v]", transaction.unpinCalls, wantBlock)
	}
	if scan.recordPage != nil {
		t.Fatal("Close() retained record page after successful unpin")
	}
	if err := scan.Close(); err != nil || len(transaction.unpinCalls) != 1 {
		t.Fatalf("second Close() = %v with calls %v, want idempotent success", err, transaction.unpinCalls)
	}
}

func TestTableScanCloseRetainsPageForUnpinRetry(t *testing.T) {
	unpinErr := errors.New("unpin failed")
	transaction := &recordingTableTransaction{size: 1, unpinErr: unpinErr}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if err := scan.Close(); !errors.Is(err, unpinErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("Close() error = %v, want contextual wrapped failure", err)
	}
	if scan.recordPage == nil {
		t.Fatal("Close() cleared record page after failed unpin")
	}
	transaction.unpinErr = nil
	if err := scan.Close(); err != nil || len(transaction.unpinCalls) != 2 || scan.recordPage != nil {
		t.Fatalf("retry Close() = %v, calls %v, page %v", err, transaction.unpinCalls, scan.recordPage)
	}
}

func TestTableScanCloseWithoutCurrentPageDoesNothing(t *testing.T) {
	transaction := &recordingTableTransaction{}
	scan := &TableScan{transaction: transaction, fileName: "student.tbl"}
	if err := scan.Close(); err != nil || len(transaction.unpinCalls) != 0 {
		t.Fatalf("Close() = %v with calls %v, want no-op success", err, transaction.unpinCalls)
	}
}
