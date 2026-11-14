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

func TestTableScanBeforeFirstRepinsBlockZeroAndResetsCursor(t *testing.T) {
	transaction := &recordingTableTransaction{size: 2}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 3

	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	wantBlock := storage.NewBlockID("student.tbl", 0)
	if len(transaction.unpinCalls) != 1 || transaction.unpinCalls[0] != wantBlock {
		t.Fatalf("Unpin calls = %v, want [%v]", transaction.unpinCalls, wantBlock)
	}
	if len(transaction.pins) != 2 || transaction.pins[1] != wantBlock {
		t.Fatalf("Pin calls = %v, want initial and rewind block zero", transaction.pins)
	}
	if scan.recordPage == nil || scan.recordPage.Block() != wantBlock || scan.currentSlot != -1 {
		t.Fatalf("scan state after BeforeFirst = %+v, want block zero before first slot", scan)
	}
}

func TestTableScanBeforeFirstStopsOnCloseFailure(t *testing.T) {
	unpinErr := errors.New("unpin failed")
	transaction := &recordingTableTransaction{size: 2, unpinErr: unpinErr}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 3

	if err := scan.BeforeFirst(); !errors.Is(err, unpinErr) {
		t.Fatalf("BeforeFirst() error = %v, want close failure", err)
	}
	if len(transaction.pins) != 1 || scan.recordPage == nil || scan.currentSlot != 3 {
		t.Fatalf("state after close failure: pins=%v page=%v slot=%d", transaction.pins, scan.recordPage, scan.currentSlot)
	}
}

func TestTableScanBeforeFirstReportsRepinFailureAfterClose(t *testing.T) {
	pinErr := errors.New("repin failed")
	transaction := &recordingTableTransaction{size: 2}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 3
	transaction.pinErr = pinErr

	if err := scan.BeforeFirst(); !errors.Is(err, pinErr) || !strings.Contains(err.Error(), "block 0") {
		t.Fatalf("BeforeFirst() error = %v, want contextual repin failure", err)
	}
	if scan.recordPage != nil || scan.currentSlot != 3 || len(transaction.unpinCalls) != 1 || len(transaction.pins) != 2 {
		t.Fatalf("state after repin failure: page=%v slot=%d unpins=%v pins=%v", scan.recordPage, scan.currentSlot, transaction.unpinCalls, transaction.pins)
	}
}

func TestTableScanNextInBlockAdvancesToOccupiedSlotsAndReportsExhaustion(t *testing.T) {
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{
			blockSize:  32,
			readValues: []int32{recordSlotUsed, recordSlotEmpty, recordSlotUsed, recordSlotEmpty},
		},
		size: 1,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if ok, err := scan.NextInBlock(); err != nil || !ok || scan.currentSlot != 0 {
		t.Fatalf("first NextInBlock() = (%t, %v), slot %d; want (true, nil), slot 0", ok, err, scan.currentSlot)
	}
	if ok, err := scan.NextInBlock(); err != nil || !ok || scan.currentSlot != 2 {
		t.Fatalf("second NextInBlock() = (%t, %v), slot %d; want (true, nil), slot 2", ok, err, scan.currentSlot)
	}
	if ok, err := scan.NextInBlock(); err != nil || ok || scan.currentSlot != -1 {
		t.Fatalf("third NextInBlock() = (%t, %v), slot %d; want (false, nil), slot -1", ok, err, scan.currentSlot)
	}
	wantOffsets := []int{0, 8, 16, 24}
	if len(transaction.reads) != len(wantOffsets) {
		t.Fatalf("flag reads = %v, want offsets %v", transaction.reads, wantOffsets)
	}
	for index, offset := range wantOffsets {
		if transaction.reads[index].offset != offset {
			t.Fatalf("flag read %d offset = %d, want %d", index, transaction.reads[index].offset, offset)
		}
	}
}

func TestTableScanNextInBlockPreservesCursorOnReadFailure(t *testing.T) {
	readErr := errors.New("flag read failed")
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{blockSize: 16, readErr: readErr},
		size:                       1,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 0

	if ok, err := scan.NextInBlock(); ok || !errors.Is(err, readErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NextInBlock() = (%t, %v), want contextual read failure", ok, err)
	}
	if scan.currentSlot != 0 {
		t.Fatalf("cursor after failure = %d, want 0", scan.currentSlot)
	}
}

func TestTableScanNextInBlockRejectsClosedScan(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if ok, err := scan.NextInBlock(); ok || err == nil {
		t.Fatalf("NextInBlock() on closed scan = (%t, %v), want explicit error", ok, err)
	}
}

func TestTableScanAtLastBlockComparesCurrentBlockWithFileSize(t *testing.T) {
	transaction := &recordingTableTransaction{size: 3}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if last, err := scan.AtLastBlock(); err != nil || last {
		t.Fatalf("AtLastBlock() on block 0 = (%t, %v), want (false, nil)", last, err)
	}
	scan.recordPage = &RecordPage{transaction: transaction, block: storage.NewBlockID("student.tbl", 2), layout: scan.layout}
	if last, err := scan.AtLastBlock(); err != nil || !last {
		t.Fatalf("AtLastBlock() on block 2 = (%t, %v), want (true, nil)", last, err)
	}
	if len(transaction.sizeCalls) != 3 {
		t.Fatalf("Size calls = %v, want initialization plus one per AtLastBlock call", transaction.sizeCalls)
	}
}

func TestTableScanAtLastBlockRejectsUnavailableStateAndWrapsSizeFailure(t *testing.T) {
	sizeErr := errors.New("size failed")
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	transaction.sizeErr = sizeErr
	if last, err := scan.AtLastBlock(); last || !errors.Is(err, sizeErr) || !strings.Contains(err.Error(), "block 0") {
		t.Fatalf("AtLastBlock() size failure = (%t, %v), want contextual failure", last, err)
	}

	transaction.sizeErr = nil
	transaction.size = 0
	if last, err := scan.AtLastBlock(); last || err == nil {
		t.Fatalf("AtLastBlock() with zero size = (%t, %v), want explicit error", last, err)
	}

	sizeCallCount := len(transaction.sizeCalls)
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if last, err := scan.AtLastBlock(); last || err == nil || len(transaction.sizeCalls) != sizeCallCount {
		t.Fatalf("AtLastBlock() after close = (%t, %v), size calls %v", last, err, transaction.sizeCalls)
	}
}

func TestTableScanMoveToNextBlockReleasesCurrentPageAndResetsCursor(t *testing.T) {
	transaction := &recordingTableTransaction{size: 3}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 2

	if err := scan.MoveToNextBlock(); err != nil {
		t.Fatalf("MoveToNextBlock() error = %v", err)
	}
	blockZero := storage.NewBlockID("student.tbl", 0)
	blockOne := storage.NewBlockID("student.tbl", 1)
	if len(transaction.unpinCalls) != 1 || transaction.unpinCalls[0] != blockZero {
		t.Fatalf("Unpin calls = %v, want [%v]", transaction.unpinCalls, blockZero)
	}
	if len(transaction.pins) != 2 || transaction.pins[1] != blockOne {
		t.Fatalf("Pin calls = %v, want initial block zero then block one", transaction.pins)
	}
	if scan.recordPage == nil || scan.recordPage.Block() != blockOne || scan.currentSlot != -1 {
		t.Fatalf("scan state after MoveToNextBlock = %+v, want block one before first slot", scan)
	}
}

func TestTableScanMoveToNextBlockStopsOnCloseFailure(t *testing.T) {
	unpinErr := errors.New("unpin failed")
	transaction := &recordingTableTransaction{size: 3, unpinErr: unpinErr}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 2

	if err := scan.MoveToNextBlock(); !errors.Is(err, unpinErr) {
		t.Fatalf("MoveToNextBlock() error = %v, want close failure", err)
	}
	if len(transaction.pins) != 1 || scan.recordPage == nil || scan.recordPage.Block().Number() != 0 || scan.currentSlot != 2 {
		t.Fatalf("state after close failure: pins=%v page=%v slot=%d", transaction.pins, scan.recordPage, scan.currentSlot)
	}
}

func TestTableScanMoveToNextBlockReportsPinFailureAfterClose(t *testing.T) {
	pinErr := errors.New("next pin failed")
	transaction := &recordingTableTransaction{size: 3}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 2
	transaction.pinErr = pinErr

	if err := scan.MoveToNextBlock(); !errors.Is(err, pinErr) || !strings.Contains(err.Error(), "block 1") {
		t.Fatalf("MoveToNextBlock() error = %v, want contextual next-block pin failure", err)
	}
	blockOne := storage.NewBlockID("student.tbl", 1)
	if scan.recordPage != nil || scan.currentSlot != 2 || len(transaction.unpinCalls) != 1 || len(transaction.pins) != 2 || transaction.pins[1] != blockOne {
		t.Fatalf("state after pin failure: page=%v slot=%d unpins=%v pins=%v", scan.recordPage, scan.currentSlot, transaction.unpinCalls, transaction.pins)
	}
}

func TestTableScanMoveToNextBlockRejectsClosedScan(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	pinCount := len(transaction.pins)
	if err := scan.MoveToNextBlock(); err == nil || len(transaction.pins) != pinCount {
		t.Fatalf("MoveToNextBlock() on closed scan = %v with pins %v, want explicit error without pin", err, transaction.pins)
	}
}

func TestTableScanNextSkipsEmptyBlocksAndFindsTheNextRecord(t *testing.T) {
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{
			blockSize:  16,
			readValues: []int32{recordSlotEmpty, recordSlotEmpty, recordSlotEmpty, recordSlotEmpty, recordSlotUsed, recordSlotEmpty},
		},
		size: 3,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", ok, err)
	}
	if scan.recordPage.Block().Number() != 2 || scan.currentSlot != 0 {
		t.Fatalf("scan position = block %d slot %d, want block 2 slot 0", scan.recordPage.Block().Number(), scan.currentSlot)
	}
	wantPins := []storage.BlockID{
		storage.NewBlockID("student.tbl", 0),
		storage.NewBlockID("student.tbl", 1),
		storage.NewBlockID("student.tbl", 2),
	}
	if len(transaction.pins) != len(wantPins) {
		t.Fatalf("Pin calls = %v, want %v", transaction.pins, wantPins)
	}
	for index := range wantPins {
		if transaction.pins[index] != wantPins[index] {
			t.Fatalf("Pin call %d = %v, want %v", index, transaction.pins[index], wantPins[index])
		}
	}
	if len(transaction.unpinCalls) != 2 || transaction.unpinCalls[0] != wantPins[0] || transaction.unpinCalls[1] != wantPins[1] {
		t.Fatalf("Unpin calls = %v, want first two blocks", transaction.unpinCalls)
	}
	if ok, err := scan.Next(); err != nil || ok || scan.recordPage.Block().Number() != 2 || scan.currentSlot != -1 {
		t.Fatalf("Next() after final record = (%t, %v), block %d slot %d; want final exhaustion", ok, err, scan.recordPage.Block().Number(), scan.currentSlot)
	}
}

func TestTableScanNextStopsAtAnEmptyLastBlock(t *testing.T) {
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{
			blockSize:  8,
			readValues: []int32{recordSlotEmpty},
		},
		size: 1,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}

	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() = (%t, %v), want (false, nil)", ok, err)
	}
	if len(transaction.pins) != 1 || len(transaction.unpinCalls) != 0 || scan.recordPage.Block().Number() != 0 || scan.currentSlot != -1 {
		t.Fatalf("state after final exhaustion: pins=%v unpins=%v page=%v slot=%d", transaction.pins, transaction.unpinCalls, scan.recordPage.Block(), scan.currentSlot)
	}
}

func TestTableScanNextWrapsTraversalFailure(t *testing.T) {
	sizeErr := errors.New("size failed")
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{blockSize: 8, readValues: []int32{recordSlotEmpty}},
		size:                       2,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	transaction.sizeErr = sizeErr

	if ok, err := scan.Next(); ok || !errors.Is(err, sizeErr) || !strings.Contains(err.Error(), "continue after block 0") {
		t.Fatalf("Next() = (%t, %v), want contextual traversal failure", ok, err)
	}
	if len(transaction.pins) != 1 || len(transaction.unpinCalls) != 0 || scan.currentSlot != -1 {
		t.Fatalf("state after failure: pins=%v unpins=%v slot=%d", transaction.pins, transaction.unpinCalls, scan.currentSlot)
	}
}

func TestTableScanNextRejectsClosedScan(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if ok, err := scan.Next(); ok || err == nil {
		t.Fatalf("Next() on closed scan = (%t, %v), want explicit error", ok, err)
	}
}

func TestTableScanGetIntReadsTheCurrentRecordField(t *testing.T) {
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{value: 42},
		size:                       1,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 1

	got, err := scan.GetInt("id")
	if err != nil || got != 42 {
		t.Fatalf("GetInt(id) = (%d, %v), want (42, nil)", got, err)
	}
	wantRead := integerRead{block: storage.NewBlockID("student.tbl", 0), offset: 12}
	if len(transaction.reads) != 1 || transaction.reads[0] != wantRead {
		t.Fatalf("integer reads = %v, want [%v]", transaction.reads, wantRead)
	}
	if scan.currentSlot != 1 || scan.recordPage.Block().Number() != 0 {
		t.Fatalf("scan state changed after read: page=%v slot=%d", scan.recordPage.Block(), scan.currentSlot)
	}
}

func TestTableScanGetIntWrapsFieldAndReadFailures(t *testing.T) {
	readErr := errors.New("read failed")
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{readErr: readErr},
		size:                       1,
	}
	var schema Schema
	schema.AddIntegerField("id")
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 1

	if _, err := scan.GetInt("missing"); err == nil || !strings.Contains(err.Error(), "student.tbl") || len(transaction.reads) != 0 {
		t.Fatalf("GetInt(missing) = %v with reads %v, want contextual lookup failure", err, transaction.reads)
	}
	if _, err := scan.GetInt("id"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 0 slot 1 field \"id\"") {
		t.Fatalf("GetInt(id) error = %v, want contextual wrapped read failure", err)
	}
	if scan.currentSlot != 1 || scan.recordPage.Block().Number() != 0 {
		t.Fatalf("scan state changed after failed read: page=%v slot=%d", scan.recordPage.Block(), scan.currentSlot)
	}
}

func TestTableScanGetIntRequiresACurrentRecord(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if _, err := scan.GetInt("id"); err == nil || len(transaction.reads) != 0 {
		t.Fatalf("GetInt() before first = %v with reads %v, want explicit error", err, transaction.reads)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := scan.GetInt("id"); err == nil || len(transaction.reads) != 0 {
		t.Fatalf("GetInt() after close = %v with reads %v, want explicit error", err, transaction.reads)
	}
}

func TestTableScanGetStringReadsTheCurrentRecordField(t *testing.T) {
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{stringValue: "東京"},
		size:                       1,
	}
	var schema Schema
	schema.AddVarcharField("name", 20)
	layout := NewLayout(&schema)
	scan, err := NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 1

	got, err := scan.GetString("name")
	if err != nil || got != "東京" {
		t.Fatalf("GetString(name) = (%q, %v), want (東京, nil)", got, err)
	}
	fieldOffset, _ := layout.Offset("name")
	wantRead := integerRead{block: storage.NewBlockID("student.tbl", 0), offset: layout.SlotSize() + fieldOffset}
	if len(transaction.stringReads) != 1 || transaction.stringReads[0] != wantRead {
		t.Fatalf("string reads = %v, want [%v]", transaction.stringReads, wantRead)
	}
	if scan.currentSlot != 1 || scan.recordPage.Block().Number() != 0 {
		t.Fatalf("scan state changed after read: page=%v slot=%d", scan.recordPage.Block(), scan.currentSlot)
	}
}

func TestTableScanGetStringWrapsFieldAndReadFailures(t *testing.T) {
	readErr := errors.New("string read failed")
	transaction := &recordingTableTransaction{
		recordingRecordTransaction: recordingRecordTransaction{stringErr: readErr},
		size:                       1,
	}
	var schema Schema
	schema.AddVarcharField("name", 20)
	scan, err := NewTableScan(transaction, "student", NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	scan.currentSlot = 1

	if _, err := scan.GetString("missing"); err == nil || !strings.Contains(err.Error(), "student.tbl") || len(transaction.stringReads) != 0 {
		t.Fatalf("GetString(missing) = %v with reads %v, want contextual lookup failure", err, transaction.stringReads)
	}
	if _, err := scan.GetString("name"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 0 slot 1 field \"name\"") {
		t.Fatalf("GetString(name) error = %v, want contextual wrapped read failure", err)
	}
	if scan.currentSlot != 1 || scan.recordPage.Block().Number() != 0 {
		t.Fatalf("scan state changed after failed read: page=%v slot=%d", scan.recordPage.Block(), scan.currentSlot)
	}
}

func TestTableScanGetStringRequiresACurrentRecord(t *testing.T) {
	transaction := &recordingTableTransaction{size: 1}
	scan, err := NewTableScan(transaction, "student", NewLayout(&Schema{}))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if _, err := scan.GetString("name"); err == nil || len(transaction.stringReads) != 0 {
		t.Fatalf("GetString() before first = %v with reads %v, want explicit error", err, transaction.stringReads)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := scan.GetString("name"); err == nil || len(transaction.stringReads) != 0 {
		t.Fatalf("GetString() after close = %v with reads %v, want explicit error", err, transaction.stringReads)
	}
}
