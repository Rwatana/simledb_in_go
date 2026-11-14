package record

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingTableTransaction struct {
	recordingRecordTransaction
	sizeCalls []string
	size      int
	sizeErr   error
}

func (t *recordingTableTransaction) Size(fileName string) (int, error) {
	t.sizeCalls = append(t.sizeCalls, fileName)
	return t.size, t.sizeErr
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

func TestNewTableScanReportsSizeEmptyAndPinFailures(t *testing.T) {
	sizeErr := errors.New("size failed")
	sizeTransaction := &recordingTableTransaction{sizeErr: sizeErr}
	if scan, err := NewTableScan(sizeTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, sizeErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NewTableScan(size failure) = (%v, %v), want contextual failure", scan, err)
	}

	emptyTransaction := &recordingTableTransaction{}
	if scan, err := NewTableScan(emptyTransaction, "student", NewLayout(&Schema{})); scan != nil || err == nil || len(emptyTransaction.pins) != 0 {
		t.Fatalf("NewTableScan(empty) = (%v, %v), pins %v, want explicit error without pin", scan, err, emptyTransaction.pins)
	}

	pinErr := errors.New("pin failed")
	pinTransaction := &recordingTableTransaction{size: 1, recordingRecordTransaction: recordingRecordTransaction{pinErr: pinErr}}
	if scan, err := NewTableScan(pinTransaction, "student", NewLayout(&Schema{})); scan != nil || !errors.Is(err, pinErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NewTableScan(pin failure) = (%v, %v), want contextual failure", scan, err)
	}
}
