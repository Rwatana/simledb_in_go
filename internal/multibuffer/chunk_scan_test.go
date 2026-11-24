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
	pins        []storage.BlockID
	unpins      []storage.BlockID
	pinErrors   map[int]error
	unpinErrors map[int][]error
}

func (transaction *chunkTransactionProbe) Pin(block storage.BlockID) error {
	transaction.pins = append(transaction.pins, block)
	return transaction.pinErrors[block.Number()]
}

func (transaction *chunkTransactionProbe) BlockSize() (int, error) { return 0, nil }

func (*chunkTransactionProbe) GetInt(storage.BlockID, int) (int32, error) { return 0, nil }

func (*chunkTransactionProbe) GetString(storage.BlockID, int) (string, error) { return "", nil }

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

func chunkLayout() *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("id")
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
