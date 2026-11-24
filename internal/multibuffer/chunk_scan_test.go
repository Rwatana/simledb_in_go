package multibuffer

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type chunkTransactionProbe struct {
	pins         []storage.BlockID
	unpins       []storage.BlockID
	pinErrors    map[int]error
	unpinErrors  map[int][]error
	blockSize    int
	blockErr     error
	intReads     []chunkRead
	intValues    map[int]map[int]int32
	intErrors    map[int]map[int]error
	stringReads  []chunkRead
	stringValues map[int]map[int]string
	stringErrors map[int]map[int]error
}

type chunkRead struct {
	block  storage.BlockID
	offset int
}

func (transaction *chunkTransactionProbe) Pin(block storage.BlockID) error {
	transaction.pins = append(transaction.pins, block)
	return transaction.pinErrors[block.Number()]
}

func (transaction *chunkTransactionProbe) BlockSize() (int, error) {
	return transaction.blockSize, transaction.blockErr
}

func (transaction *chunkTransactionProbe) GetInt(block storage.BlockID, offset int) (int32, error) {
	transaction.intReads = append(transaction.intReads, chunkRead{block: block, offset: offset})
	if err := transaction.intErrors[block.Number()][offset]; err != nil {
		return 0, err
	}
	return transaction.intValues[block.Number()][offset], nil
}

func (transaction *chunkTransactionProbe) GetString(block storage.BlockID, offset int) (string, error) {
	transaction.stringReads = append(transaction.stringReads, chunkRead{block: block, offset: offset})
	if err := transaction.stringErrors[block.Number()][offset]; err != nil {
		return "", err
	}
	return transaction.stringValues[block.Number()][offset], nil
}

func (*chunkTransactionProbe) SetInt(storage.BlockID, int32, int32, bool) error { return nil }

func (*chunkTransactionProbe) SetString(storage.BlockID, int32, string, bool) error { return nil }

func (transaction *chunkTransactionProbe) Unpin(block storage.BlockID) error {
	transaction.unpins = append(transaction.unpins, block)
	queued := transaction.unpinErrors[block.Number()]
	if len(queued) == 0 {
		return nil
	}
	transaction.unpinErrors[block.Number()] = queued[1:]
	return queued[0]
}

func TestNewChunkScanPinsInclusiveRangeAndCloseUnpinsIt(t *testing.T) {
	transaction := &chunkTransactionProbe{}
	layout := chunkLayout()
	scan, err := NewChunkScan(transaction, "records.tbl", layout, 2, 4)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, want := transaction.pins, chunkBlocks(2, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("pinned blocks = %v, want %v", got, want)
	}
	if scan.fileName != "records.tbl" || scan.layout != layout || scan.startBlock != 2 || scan.endBlock != 4 || len(scan.pages) != 3 {
		t.Fatalf("scan metadata = %+v, want file/range/layout retained with three pages", scan)
	}

	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got, want := transaction.unpins, chunkBlocks(2, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("unpinned blocks = %v, want %v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if got, want := transaction.unpins, chunkBlocks(2, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("second Close() repeated an unpin: got %v, want %v", got, want)
	}
}

func TestNewChunkScanRejectsInvalidConfigurationBeforePinning(t *testing.T) {
	transaction := &chunkTransactionProbe{}
	layout := chunkLayout()
	tests := []struct {
		name        string
		transaction ChunkTransaction
		fileName    string
		layout      *record.Layout
		start       int
		end         int
		wantMessage string
	}{
		{name: "nil transaction", fileName: "records.tbl", layout: layout, start: 0, end: 0, wantMessage: "transaction is nil"},
		{name: "empty filename", transaction: transaction, layout: layout, start: 0, end: 0, wantMessage: "filename is empty"},
		{name: "nil layout", transaction: transaction, fileName: "records.tbl", start: 0, end: 0, wantMessage: "layout is nil"},
		{name: "negative start", transaction: transaction, fileName: "records.tbl", layout: layout, start: -1, end: 0, wantMessage: "range must be non-negative"},
		{name: "negative end", transaction: transaction, fileName: "records.tbl", layout: layout, start: 0, end: -1, wantMessage: "range must be non-negative"},
		{name: "end before start", transaction: transaction, fileName: "records.tbl", layout: layout, start: 2, end: 1, wantMessage: "end block 1 precedes start block 2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewChunkScan(test.transaction, test.fileName, test.layout, test.start, test.end)
			if err == nil || !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("NewChunkScan() error = %v, want message containing %q", err, test.wantMessage)
			}
			if len(transaction.pins) != 0 {
				t.Fatalf("invalid configuration pinned blocks %v", transaction.pins)
			}
		})
	}
}

func TestNewChunkScanPinsSingleMaximumBlock(t *testing.T) {
	transaction := &chunkTransactionProbe{}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), math.MaxInt, math.MaxInt)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	want := []storage.BlockID{storage.NewBlockID("records.tbl", math.MaxInt)}
	if !reflect.DeepEqual(transaction.pins, want) {
		t.Fatalf("pinned blocks = %v, want %v", transaction.pins, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewChunkScanReleasesEarlierPagesAfterPinFailure(t *testing.T) {
	pinErr := errors.New("pin failed")
	transaction := &chunkTransactionProbe{pinErrors: map[int]error{4: pinErr}}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 2, 4)
	if scan != nil || !errors.Is(err, pinErr) {
		t.Fatalf("NewChunkScan() = (%v, %v), want nil scan and wrapped pin failure", scan, err)
	}
	if got, want := transaction.pins, chunkBlocks(2, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("pin attempts = %v, want %v", got, want)
	}
	if got, want := transaction.unpins, chunkBlocks(2, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup unpins = %v, want %v", got, want)
	}
}

func TestNewChunkScanRetainsPagesWhoseCleanupFailed(t *testing.T) {
	pinErr := errors.New("pin failed")
	cleanupErr := errors.New("cleanup failed")
	transaction := &chunkTransactionProbe{
		pinErrors:   map[int]error{4: pinErr},
		unpinErrors: map[int][]error{3: {cleanupErr}},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 2, 4)
	if scan == nil || !errors.Is(err, pinErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("NewChunkScan() = (%v, %v), want retryable scan and both causes", scan, err)
	}
	if got, want := chunkPageNumbers(scan.pages), []int{3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("retained pages = %v, want failed release block [3]", got)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	if got, want := chunkPageNumbers(scan.pages), []int{}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pages after successful retry = %v, want none", got)
	}
	wantUnpins := []storage.BlockID{storage.NewBlockID("records.tbl", 2), storage.NewBlockID("records.tbl", 3), storage.NewBlockID("records.tbl", 3)}
	if !reflect.DeepEqual(transaction.unpins, wantUnpins) {
		t.Fatalf("unpin attempts = %v, want %v", transaction.unpins, wantUnpins)
	}
}

func TestChunkScanCloseAttemptsAllPagesAndRetriesOnlyFailures(t *testing.T) {
	firstErr := errors.New("first release failed")
	lastErr := errors.New("last release failed")
	transaction := &chunkTransactionProbe{unpinErrors: map[int][]error{1: {firstErr}, 3: {lastErr}}}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 1, 3)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if err := scan.Close(); !errors.Is(err, firstErr) || !errors.Is(err, lastErr) {
		t.Fatalf("Close() error = %v, want both unpin causes", err)
	}
	if got, want := chunkPageNumbers(scan.pages), []int{1, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pages retained after close failure = %v, want [1 3]", got)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	wantUnpins := []storage.BlockID{
		storage.NewBlockID("records.tbl", 1), storage.NewBlockID("records.tbl", 2), storage.NewBlockID("records.tbl", 3),
		storage.NewBlockID("records.tbl", 1), storage.NewBlockID("records.tbl", 3),
	}
	if !reflect.DeepEqual(transaction.unpins, wantUnpins) {
		t.Fatalf("unpin attempts = %v, want %v", transaction.unpins, wantUnpins)
	}
	if err := scan.Close(); err != nil || len(transaction.unpins) != len(wantUnpins) {
		t.Fatalf("third Close() = %v with unpins %v, want no more operations", err, transaction.unpins)
	}
}

func TestChunkScanCloseAcceptsNilReceiver(t *testing.T) {
	var scan *ChunkScan
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() on nil scan = %v, want nil", err)
	}
	if err := (&ChunkScan{}).Close(); err != nil {
		t.Fatalf("Close() on zero-value scan = %v, want nil", err)
	}
}

func TestChunkScanNextCrossesBlocksAfterFinishingTheCurrentBlock(t *testing.T) {
	transaction := &chunkTransactionProbe{
		blockSize: 24,
		intValues: map[int]map[int]int32{
			5: {0: 0, 8: 1, 16: 1},
			6: {0: 1},
		},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 5, 6)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if scan.currentPage != 0 || scan.currentSlot != -1 || !scan.ready {
		t.Fatalf("initial cursor = page %d, slot %d, ready %v; want page 0, slot -1, ready", scan.currentPage, scan.currentSlot, scan.ready)
	}
	for _, wantSlot := range []int{1, 2} {
		got, err := scan.Next()
		if err != nil || !got || scan.currentSlot != wantSlot {
			t.Fatalf("Next() at slot %d = (%v, %v) with current slot %d, want (true, nil) at %d", wantSlot, got, err, scan.currentSlot, wantSlot)
		}
	}
	if got, want := transaction.intReads[len(transaction.intReads)-1].block.Number(), 5; got != want {
		t.Fatalf("block read before exhausting active page = %d, want 5", got)
	}
	if got, err := scan.Next(); err != nil || !got || scan.currentPage != 1 || scan.currentSlot != 0 {
		t.Fatalf("Next() after first block = (%v, %v), page %d slot %d; want next block slot 0", got, err, scan.currentPage, scan.currentSlot)
	}
	if got, err := scan.Next(); got || err != nil || !scan.exhausted {
		t.Fatalf("Next() at end of range = (%v, %v) exhausted %v, want (false, nil) and exhausted", got, err, scan.exhausted)
	}
	readsAtExhaustion := len(transaction.intReads)
	if got, err := scan.Next(); got || err != nil || len(transaction.intReads) != readsAtExhaustion {
		t.Fatalf("repeated Next() at end of range = (%v, %v), reads %d; want no row, error, or additional read", got, err, len(transaction.intReads))
	}
	if got, want := transaction.intReads[len(transaction.intReads)-1].block.Number(), 6; got != want {
		t.Fatalf("last read block = %d, want final block %d", got, want)
	}
	if got, want := transaction.pins, chunkBlocks(5, 6); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins after traversal = %v, want unchanged %v", got, want)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if scan.currentPage != 0 || scan.currentSlot != -1 || scan.exhausted {
		t.Fatalf("cursor after rewind = page %d, slot %d, exhausted %v; want page 0, slot -1, not exhausted", scan.currentPage, scan.currentSlot, scan.exhausted)
	}
	if got, err := scan.Next(); err != nil || !got || scan.currentSlot != 1 {
		t.Fatalf("Next() after rewind = (%v, %v) at slot %d, want (true, nil) at 1", got, err, scan.currentSlot)
	}
	if got, want := transaction.pins, chunkBlocks(5, 6); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins after rewind = %v, want unchanged %v", got, want)
	}
}

func TestChunkScanNextSkipsEmptyPagesAndRewindsAcrossTheRange(t *testing.T) {
	transaction := &chunkTransactionProbe{
		blockSize: 16,
		intValues: map[int]map[int]int32{
			7: {0: 0, 8: 0},
			8: {0: 0, 8: 0},
			9: {0: 1, 8: 0},
		},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 7, 9)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); err != nil || !got || scan.currentPage != 2 || scan.currentSlot != 0 {
		t.Fatalf("Next() after empty pages = (%v, %v), page %d slot %d; want page 2 slot 0", got, err, scan.currentPage, scan.currentSlot)
	}
	if got, want := transaction.pins, chunkBlocks(7, 9); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins after rollover = %v, want unchanged %v", got, want)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if scan.currentPage != 0 || scan.currentSlot != -1 {
		t.Fatalf("cursor after rewind = page %d slot %d, want page 0 slot -1", scan.currentPage, scan.currentSlot)
	}
	if got, err := scan.Next(); err != nil || !got || scan.currentPage != 2 || scan.currentSlot != 0 {
		t.Fatalf("Next() after range rewind = (%v, %v), page %d slot %d; want page 2 slot 0", got, err, scan.currentPage, scan.currentSlot)
	}
}

func TestChunkScanNextWrapsReadErrorAfterMovingToLaterBlock(t *testing.T) {
	readErr := errors.New("later record flag read failed")
	transaction := &chunkTransactionProbe{
		blockSize: 8,
		intErrors: map[int]map[int]error{11: {0: readErr}},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 10, 11)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); got || !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 11") {
		t.Fatalf("Next() after rollover read failure = (%v, %v), want wrapped cause with block 11 context", got, err)
	}
	if scan.currentPage != 1 || scan.currentSlot != -1 {
		t.Fatalf("cursor after later-block error = page %d slot %d, want page 1 slot -1", scan.currentPage, scan.currentSlot)
	}
}

func TestChunkScanGetIntReadsCurrentRecordWithoutMovingCursor(t *testing.T) {
	transaction := &chunkTransactionProbe{
		blockSize: 8,
		intValues: map[int]map[int]int32{
			5: {0: 1, 4: 17},
			6: {0: 1, 4: 42},
		},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 5, 6)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	for _, want := range []int32{17, 42} {
		got, err := scan.Next()
		if err != nil || !got {
			t.Fatalf("Next() = (%v, %v), want a current record", got, err)
		}
		currentPage, currentSlot, pins := scan.currentPage, scan.currentSlot, len(transaction.pins)
		value, err := scan.GetInt("id")
		if err != nil || value != want {
			t.Fatalf("GetInt(id) = (%d, %v), want (%d, nil)", value, err, want)
		}
		if scan.currentPage != currentPage || scan.currentSlot != currentSlot || len(transaction.pins) != pins {
			t.Fatalf("GetInt changed cursor/page ownership: page %d slot %d pins %d", scan.currentPage, scan.currentSlot, len(transaction.pins))
		}
	}
}

func TestChunkScanGetIntWrapsReadErrorAndRejectsMissingField(t *testing.T) {
	readErr := errors.New("integer field read failed")
	transaction := &chunkTransactionProbe{
		blockSize: 8,
		intValues: map[int]map[int]int32{4: {0: 1}},
		intErrors: map[int]map[int]error{4: {4: readErr}},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 4, 4)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); err != nil || !got {
		t.Fatalf("Next() = (%v, %v), want one record", got, err)
	}
	if _, err := scan.GetInt("missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("GetInt(missing) error = %v, want a field lookup error", err)
	}
	if _, err := scan.GetInt("id"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 4") {
		t.Fatalf("GetInt(id) error = %v, want wrapped cause and block context", err)
	}
}

func TestChunkScanGetIntRejectsUnavailableCurrentRecordBeforeFieldIO(t *testing.T) {
	validTransaction := func() *chunkTransactionProbe {
		return &chunkTransactionProbe{blockSize: 8, intValues: map[int]map[int]int32{0: {0: 1, 4: 7}}}
	}
	beforeFirst, err := NewChunkScan(validTransaction(), "records.tbl", chunkLayout(), 0, 0)
	if err != nil {
		t.Fatalf("before-first setup error = %v", err)
	}
	closedTransaction := validTransaction()
	closed, err := NewChunkScan(closedTransaction, "records.tbl", chunkLayout(), 0, 0)
	if err != nil {
		t.Fatalf("closed setup error = %v", err)
	}
	if got, err := closed.Next(); err != nil || !got {
		t.Fatalf("closed setup Next() = (%v, %v), want one record", got, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() setup error = %v", err)
	}
	exhaustedTransaction := &chunkTransactionProbe{blockSize: 8}
	exhausted, err := NewChunkScan(exhaustedTransaction, "records.tbl", chunkLayout(), 0, 0)
	if err != nil {
		t.Fatalf("exhausted setup error = %v", err)
	}
	if got, err := exhausted.Next(); err != nil || got {
		t.Fatalf("exhausted setup Next() = (%v, %v), want no row", got, err)
	}
	partialTransaction := &chunkTransactionProbe{
		pinErrors:   map[int]error{1: errors.New("pin failed")},
		unpinErrors: map[int][]error{0: {errors.New("unpin failed")}},
	}
	partial, err := NewChunkScan(partialTransaction, "records.tbl", chunkLayout(), 0, 1)
	if partial == nil || err == nil {
		t.Fatalf("partial setup = (%v, %v), want scan and pin/cleanup errors", partial, err)
	}
	tests := []struct {
		name        string
		scan        *ChunkScan
		transaction *chunkTransactionProbe
	}{
		{name: "nil receiver"},
		{name: "zero value", scan: &ChunkScan{}},
		{name: "before first", scan: beforeFirst, transaction: beforeFirst.transaction.(*chunkTransactionProbe)},
		{name: "exhausted", scan: exhausted, transaction: exhaustedTransaction},
		{name: "closed", scan: closed, transaction: closedTransaction},
		{name: "partial initialization", scan: partial, transaction: partialTransaction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.scan.GetInt("id"); err == nil {
				t.Fatal("GetInt(id) error = nil, want unavailable-record error")
			}
			if test.transaction != nil {
				for _, read := range test.transaction.intReads {
					if read.offset == 4 {
						t.Fatalf("invalid cursor read integer field data: %v", test.transaction.intReads)
					}
				}
			}
		})
	}
	if err := partial.Close(); err != nil {
		t.Fatalf("retry partial cleanup = %v", err)
	}
}

func TestChunkScanGetStringReadsCurrentRecordWithoutMovingCursor(t *testing.T) {
	layout := chunkStringLayout()
	nameOffset, ok := layout.Offset("name")
	if !ok {
		t.Fatal("name field offset is unavailable")
	}
	transaction := &chunkTransactionProbe{
		blockSize: 128,
		intValues: map[int]map[int]int32{5: {0: 1}, 6: {0: 1}},
		stringValues: map[int]map[int]string{
			5: {nameOffset: "first-page"},
			6: {nameOffset: "later-page"},
		},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", layout, 5, 6)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	for _, want := range []string{"first-page", "later-page"} {
		got, err := scan.Next()
		if err != nil || !got {
			t.Fatalf("Next() = (%v, %v), want a current record", got, err)
		}
		currentPage, currentSlot, pins := scan.currentPage, scan.currentSlot, len(transaction.pins)
		value, err := scan.GetString("name")
		if err != nil || value != want {
			t.Fatalf("GetString(name) = (%q, %v), want (%q, nil)", value, err, want)
		}
		if scan.currentPage != currentPage || scan.currentSlot != currentSlot || len(transaction.pins) != pins {
			t.Fatalf("GetString changed cursor/page ownership: page %d slot %d pins %d", scan.currentPage, scan.currentSlot, len(transaction.pins))
		}
	}
}

func TestChunkScanGetStringWrapsReadErrorAndRejectsMissingField(t *testing.T) {
	layout := chunkStringLayout()
	nameOffset, ok := layout.Offset("name")
	if !ok {
		t.Fatal("name field offset is unavailable")
	}
	readErr := errors.New("string field read failed")
	transaction := &chunkTransactionProbe{
		blockSize: 128,
		intValues: map[int]map[int]int32{4: {0: 1}},
		stringErrors: map[int]map[int]error{
			4: {nameOffset: readErr},
		},
	}
	scan, err := NewChunkScan(transaction, "records.tbl", layout, 4, 4)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); err != nil || !got {
		t.Fatalf("Next() = (%v, %v), want one record", got, err)
	}
	if _, err := scan.GetString("missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("GetString(missing) error = %v, want a field lookup error", err)
	}
	if _, err := scan.GetString("name"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 4") {
		t.Fatalf("GetString(name) error = %v, want wrapped cause and block context", err)
	}
}

func TestChunkScanGetStringRejectsUnavailableCurrentRecordBeforeFieldIO(t *testing.T) {
	validTransaction := func() *chunkTransactionProbe {
		return &chunkTransactionProbe{blockSize: 128, intValues: map[int]map[int]int32{0: {0: 1}}}
	}
	layout := chunkStringLayout()
	beforeFirstTransaction := validTransaction()
	beforeFirst, err := NewChunkScan(beforeFirstTransaction, "records.tbl", layout, 0, 0)
	if err != nil {
		t.Fatalf("before-first setup error = %v", err)
	}
	closedTransaction := validTransaction()
	closed, err := NewChunkScan(closedTransaction, "records.tbl", layout, 0, 0)
	if err != nil {
		t.Fatalf("closed setup error = %v", err)
	}
	if got, err := closed.Next(); err != nil || !got {
		t.Fatalf("closed setup Next() = (%v, %v), want one record", got, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() setup error = %v", err)
	}
	exhaustedTransaction := &chunkTransactionProbe{blockSize: 64}
	exhausted, err := NewChunkScan(exhaustedTransaction, "records.tbl", layout, 0, 0)
	if err != nil {
		t.Fatalf("exhausted setup error = %v", err)
	}
	if got, err := exhausted.Next(); err != nil || got {
		t.Fatalf("exhausted setup Next() = (%v, %v), want no row", got, err)
	}
	partialTransaction := &chunkTransactionProbe{
		pinErrors:   map[int]error{1: errors.New("pin failed")},
		unpinErrors: map[int][]error{0: {errors.New("unpin failed")}},
	}
	partial, err := NewChunkScan(partialTransaction, "records.tbl", layout, 0, 1)
	if partial == nil || err == nil {
		t.Fatalf("partial setup = (%v, %v), want scan and pin/cleanup errors", partial, err)
	}
	tests := []struct {
		name        string
		scan        *ChunkScan
		transaction *chunkTransactionProbe
	}{
		{name: "nil receiver"},
		{name: "zero value", scan: &ChunkScan{}},
		{name: "before first", scan: beforeFirst, transaction: beforeFirstTransaction},
		{name: "exhausted", scan: exhausted, transaction: exhaustedTransaction},
		{name: "closed", scan: closed, transaction: closedTransaction},
		{name: "partial initialization", scan: partial, transaction: partialTransaction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.scan.GetString("name"); err == nil {
				t.Fatal("GetString(name) error = nil, want unavailable-record error")
			}
			if test.transaction != nil && len(test.transaction.stringReads) != 0 {
				t.Fatalf("invalid cursor read string field data: %v", test.transaction.stringReads)
			}
		})
	}
	if err := partial.Close(); err != nil {
		t.Fatalf("retry partial cleanup = %v", err)
	}
}

func TestChunkScanHasFieldReportsSchemaWithoutStorageAccess(t *testing.T) {
	layout := chunkStringLayout()
	transaction := &chunkTransactionProbe{blockSize: 128}
	scan, err := NewChunkScan(transaction, "records.tbl", layout, 0, 0)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	pins := append([]storage.BlockID(nil), transaction.pins...)
	for field, want := range map[string]bool{"id": true, "name": true, "missing": false, "": false} {
		if got := scan.HasField(field); got != want {
			t.Errorf("HasField(%q) = %t, want %t", field, got, want)
		}
	}
	if !reflect.DeepEqual(transaction.pins, pins) || len(transaction.intReads) != 0 || len(transaction.stringReads) != 0 {
		t.Fatalf("HasField performed storage I/O: pins=%v intReads=%v stringReads=%v", transaction.pins, transaction.intReads, transaction.stringReads)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for field, want := range map[string]bool{"id": true, "name": true, "missing": false} {
		if got := scan.HasField(field); got != want {
			t.Errorf("closed HasField(%q) = %t, want %t", field, got, want)
		}
	}
	var nilScan *ChunkScan
	if nilScan.HasField("id") {
		t.Error("nil HasField(id) = true, want false")
	}
	if (&ChunkScan{}).HasField("id") {
		t.Error("zero-value HasField(id) = true, want false")
	}
	if len(transaction.intReads) != 0 || len(transaction.stringReads) != 0 {
		t.Fatalf("HasField read field data: integers=%v strings=%v", transaction.intReads, transaction.stringReads)
	}
}

func TestChunkScanNextReportsPageErrorsWithoutMovingCursor(t *testing.T) {
	blockErr := errors.New("block size failed")
	transaction := &chunkTransactionProbe{blockErr: blockErr}
	scan, err := NewChunkScan(transaction, "records.tbl", chunkLayout(), 3, 3)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); got || !errors.Is(err, blockErr) || !strings.Contains(err.Error(), "block 3") {
		t.Fatalf("Next() with block-size error = (%v, %v), want wrapped cause and block context", got, err)
	}
	if scan.currentSlot != -1 {
		t.Fatalf("current slot after block-size error = %d, want -1", scan.currentSlot)
	}

	readErr := errors.New("record flag read failed")
	transaction = &chunkTransactionProbe{blockSize: 8, intErrors: map[int]map[int]error{4: {0: readErr}}}
	scan, err = NewChunkScan(transaction, "records.tbl", chunkLayout(), 4, 4)
	if err != nil {
		t.Fatalf("second NewChunkScan() error = %v", err)
	}
	if got, err := scan.Next(); got || !errors.Is(err, readErr) || !strings.Contains(err.Error(), "block 4") {
		t.Fatalf("Next() with record-read error = (%v, %v), want wrapped cause and block context", got, err)
	}
	if scan.currentSlot != -1 {
		t.Fatalf("current slot after record-read error = %d, want -1", scan.currentSlot)
	}
}

func TestChunkScanBeforeFirstAndNextRejectUnavailableCursor(t *testing.T) {
	cleanupErr := errors.New("cleanup failed")
	partialTransaction := &chunkTransactionProbe{
		pinErrors:   map[int]error{5: errors.New("pin failed")},
		unpinErrors: map[int][]error{4: {cleanupErr}},
	}
	partialScan, err := NewChunkScan(partialTransaction, "records.tbl", chunkLayout(), 4, 5)
	if partialScan == nil || !errors.Is(err, cleanupErr) {
		t.Fatalf("partial NewChunkScan() = (%v, %v), want a scan retaining the failed cleanup", partialScan, err)
	}
	closedScan, err := NewChunkScan(&chunkTransactionProbe{}, "records.tbl", chunkLayout(), 0, 0)
	if err != nil {
		t.Fatalf("closed scan setup error = %v", err)
	}
	if err := closedScan.Close(); err != nil {
		t.Fatalf("close scan setup = %v", err)
	}
	badCursor, err := NewChunkScan(&chunkTransactionProbe{}, "records.tbl", chunkLayout(), 0, 0)
	if err != nil {
		t.Fatalf("bad cursor setup error = %v", err)
	}
	badCursor.currentPage = 1
	tests := []struct {
		name string
		scan *ChunkScan
	}{
		{name: "nil receiver"},
		{name: "zero value", scan: &ChunkScan{}},
		{name: "partial initialization", scan: partialScan},
		{name: "closed", scan: closedScan},
		{name: "invalid page index", scan: badCursor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.scan.BeforeFirst(); err == nil {
				t.Fatal("BeforeFirst() error = nil, want unavailable cursor error")
			}
			if got, err := test.scan.Next(); got || err == nil {
				t.Fatalf("Next() = (%v, %v), want unavailable cursor error", got, err)
			}
		})
	}
	if err := partialScan.Close(); err != nil {
		t.Fatalf("retry partial scan cleanup = %v", err)
	}
}

func chunkLayout() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("id")
	return record.NewLayout(&schema)
}

func chunkStringLayout() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 16)
	return record.NewLayout(&schema)
}

func chunkBlocks(start, end int) []storage.BlockID {
	blocks := make([]storage.BlockID, 0, end-start+1)
	for number := start; number <= end; number++ {
		blocks = append(blocks, storage.NewBlockID("records.tbl", number))
	}
	return blocks
}

func chunkPageNumbers(pages []*record.RecordPage) []int {
	numbers := make([]int, len(pages))
	for index, page := range pages {
		numbers[index] = page.Block().Number()
	}
	return numbers
}
