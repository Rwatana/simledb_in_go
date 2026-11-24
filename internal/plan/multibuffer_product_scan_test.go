package plan

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/multibuffer"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type multibufferProductTransaction struct {
	chunkValueTransaction
	fileSize       int
	fileSizeErr    error
	available      int
	sizeFileNames  []string
	availableCalls int
}

func (transaction *multibufferProductTransaction) Size(fileName string) (int, error) {
	transaction.sizeFileNames = append(transaction.sizeFileNames, fileName)
	return transaction.fileSize, transaction.fileSizeErr
}

func (transaction *multibufferProductTransaction) AvailableBuffers() int {
	transaction.availableCalls++
	return transaction.available
}

type multibufferProductLeftProbe struct {
	beforeFirstCalls int
	beforeFirstErr   error
}

func (scan *multibufferProductLeftProbe) BeforeFirst() error {
	scan.beforeFirstCalls++
	return scan.beforeFirstErr
}

func (*multibufferProductLeftProbe) Next() (bool, error)              { return false, nil }
func (*multibufferProductLeftProbe) GetInt(string) (int32, error)     { return 0, nil }
func (*multibufferProductLeftProbe) GetString(string) (string, error) { return "", nil }
func (*multibufferProductLeftProbe) GetVal(string) (query.Constant, error) {
	return query.Constant{}, nil
}
func (*multibufferProductLeftProbe) HasField(string) bool { return false }
func (*multibufferProductLeftProbe) Close() error         { return nil }

func TestNewMultibufferProductScanInitializesFirstBoundedChunk(t *testing.T) {
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{blockSize: 64},
		fileSize:              10,
		available:             6,
	}
	left := &multibufferProductLeftProbe{}
	layout := chunkValueLayout(false)
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if got, want := transaction.sizeFileNames, []string{"student.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("file size requests = %v, want %v", got, want)
	}
	if transaction.availableCalls != 1 {
		t.Fatalf("AvailableBuffers() calls = %d, want one", transaction.availableCalls)
	}
	if got, want := scan.chunkSize, multibuffer.BestFactor(6, 10); got != want || got != 4 {
		t.Fatalf("chunk size = %d, want BestFactor(6, 10) = 4", got)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("first chunk pins = %v, want %v", got, want)
	}
	if scan.nextBlock != 4 || scan.right == nil {
		t.Fatalf("initial scan state = next block %d, right %v; want next block 4 and an initialized right scan", scan.nextBlock, scan.right)
	}
	if left.beforeFirstCalls != 1 {
		t.Fatalf("left BeforeFirst() calls = %d, want one", left.beforeFirstCalls)
	}
}

func TestNewMultibufferProductScanLeavesEmptyRightRelationUnpinned(t *testing.T) {
	transaction := &multibufferProductTransaction{fileSize: 0, available: 4}
	left := &multibufferProductLeftProbe{}
	scan, err := NewMultibufferProductScan(transaction, left, "empty", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if len(transaction.pins) != 0 || scan.right != nil || scan.nextBlock != 0 {
		t.Fatalf("empty right initialization: pins=%v scan=%+v, want no chunk", transaction.pins, scan)
	}
	if left.beforeFirstCalls != 0 {
		t.Fatalf("empty right reset left scan %d times, want zero", left.beforeFirstCalls)
	}
	if got, want := transaction.sizeFileNames, []string{"empty.tbl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("file size requests = %v, want %v", got, want)
	}
}

func TestNewMultibufferProductScanValidatesInputsAndFileSize(t *testing.T) {
	transaction := &multibufferProductTransaction{fileSize: 1, available: 4}
	left := &multibufferProductLeftProbe{}
	layout := chunkValueLayout(false)
	schemaNil := record.NewLayoutFromMetadata(nil, nil, 0)
	tests := []struct {
		name        string
		transaction MultibufferProductTransaction
		left        query.Scan
		tableName   string
		layout      *record.Layout
		wantError   string
	}{
		{"nil transaction", nil, left, "student", layout, "transaction is nil"},
		{"nil left scan", transaction, nil, "student", layout, "left scan is nil"},
		{"empty table name", transaction, left, "", layout, "table name is empty"},
		{"nil layout", transaction, left, "student", nil, "layout is nil"},
		{"nil schema", transaction, left, "student", schemaNil, "layout schema is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := NewMultibufferProductScan(test.transaction, test.left, test.tableName, test.layout)
			if scan != nil || err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("NewMultibufferProductScan() = (%v, %v), want error containing %q", scan, err, test.wantError)
			}
		})
	}
	sizeErr := errors.New("file size failed")
	transaction.fileSizeErr = sizeErr
	if _, err := NewMultibufferProductScan(transaction, left, "student", layout); !errors.Is(err, sizeErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("file size error = %v, want wrapped cause and filename", err)
	}
	transaction.fileSizeErr = nil
	transaction.fileSize = -1
	if _, err := NewMultibufferProductScan(transaction, left, "student", layout); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative file size error = %v, want validation error", err)
	}
	transaction.fileSize = 1
	transaction.available = -1
	if _, err := NewMultibufferProductScan(transaction, left, "student", layout); err == nil || !strings.Contains(err.Error(), "negative buffer") {
		t.Fatalf("negative available buffers error = %v, want validation error", err)
	}
}

func TestNewMultibufferProductScanReleasesFirstChunkAfterLeftRewindFailure(t *testing.T) {
	leftErr := errors.New("left rewind failed")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{blockSize: 64},
		fileSize:              2,
		available:             4,
	}
	left := &multibufferProductLeftProbe{beforeFirstErr: leftErr}
	if scan, err := NewMultibufferProductScan(transaction, left, "student", chunkValueLayout(false)); scan != nil || !errors.Is(err, leftErr) {
		t.Fatalf("NewMultibufferProductScan() = (%v, %v), want rewind error after cleanup", scan, err)
	}
	if got, want := transaction.unpins, multibufferProductBlocks(0, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup unpins = %v, want %v", got, want)
	}
}

func TestNewMultibufferProductScanRetainsChunkWhenCleanupFails(t *testing.T) {
	pinErr := errors.New("chunk pin failed")
	unpinErr := errors.New("chunk cleanup failed")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   64,
			pinErrors:   map[int]error{2: pinErr},
			unpinErrors: map[int][]error{0: {unpinErr}},
		},
		fileSize:  10,
		available: 6,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftProbe{}, "student", chunkValueLayout(false))
	if scan == nil || err == nil || !errors.Is(err, pinErr) || !errors.Is(err, unpinErr) {
		t.Fatalf("NewMultibufferProductScan() = (%v, %v), want partial scan and both causes", scan, err)
	}
	if scan.right == nil {
		t.Fatal("partial product scan did not retain its chunk cleanup handle")
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("retry retained chunk cleanup = %v", err)
	}
	if got, want := transaction.unpins, []storage.BlockID{
		storage.NewBlockID("student.tbl", 0),
		storage.NewBlockID("student.tbl", 1),
		storage.NewBlockID("student.tbl", 0),
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partial chunk unpins = %v, want initial cleanup followed by retry of only the retained block %v", transaction.unpins, want)
	}
}

func TestNewMultibufferProductScanReturnsPinErrorAfterSuccessfulCleanup(t *testing.T) {
	pinErr := errors.New("first chunk pin failed")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{pinErrors: map[int]error{0: pinErr}},
		fileSize:              2,
		available:             4,
	}
	left := &multibufferProductLeftProbe{}
	scan, err := NewMultibufferProductScan(transaction, left, "student", chunkValueLayout(false))
	if scan != nil || !errors.Is(err, pinErr) || !strings.Contains(err.Error(), "student.tbl") {
		t.Fatalf("NewMultibufferProductScan() = (%v, %v), want contextual pin error", scan, err)
	}
	if len(transaction.unpins) != 0 || left.beforeFirstCalls != 0 {
		t.Fatalf("failed first pin caused cleanup or left rewind: unpins=%v rewinds=%d", transaction.unpins, left.beforeFirstCalls)
	}
}

func TestMultibufferProductScanCloseReleasesInitialChunk(t *testing.T) {
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{blockSize: 64},
		fileSize:              3,
		available:             4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftProbe{}, "student", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got, want := transaction.unpins, multibufferProductBlocks(0, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("Close() unpins = %v, want %v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
	if got, want := transaction.unpins, multibufferProductBlocks(0, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("repeated Close() unpins = %v, want no additional releases", got)
	}
}

func TestMultibufferProductScanCloseRetriesChunkCleanupFailure(t *testing.T) {
	cleanupErr := errors.New("right chunk release failed")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   64,
			unpinErrors: map[int][]error{1: {cleanupErr}},
		},
		fileSize:  2,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftProbe{}, "student", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if err := scan.Close(); !errors.Is(err, cleanupErr) {
		t.Fatalf("Close() error = %v, want wrapped release error", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	want := []storage.BlockID{
		storage.NewBlockID("student.tbl", 0),
		storage.NewBlockID("student.tbl", 1),
		storage.NewBlockID("student.tbl", 1),
	}
	if !reflect.DeepEqual(transaction.unpins, want) {
		t.Fatalf("Close() unpins = %v, want failed-page-only retry sequence %v", transaction.unpins, want)
	}
}

func TestMultibufferProductScanRetainsChunkWhenRewindCleanupFails(t *testing.T) {
	leftErr := errors.New("left rewind failed")
	cleanupErr := errors.New("right chunk release failed")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   64,
			unpinErrors: map[int][]error{0: {cleanupErr}},
		},
		fileSize:  1,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftProbe{beforeFirstErr: leftErr}, "student", chunkValueLayout(false))
	if scan == nil || !errors.Is(err, leftErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("NewMultibufferProductScan() = (%v, %v), want retryable scan and rewind/cleanup causes", scan, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	if got, want := transaction.unpins, []storage.BlockID{
		storage.NewBlockID("student.tbl", 0),
		storage.NewBlockID("student.tbl", 0),
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup retry unpins = %v, want %v", got, want)
	}
}

func TestMultibufferProductScanCloseAcceptsNilAndZeroValue(t *testing.T) {
	var nilScan *MultibufferProductScan
	if err := nilScan.Close(); err != nil {
		t.Fatalf("nil Close() error = %v, want nil", err)
	}
	if err := (&MultibufferProductScan{}).Close(); err != nil {
		t.Fatalf("zero-value Close() error = %v, want nil", err)
	}
}

func multibufferProductBlocks(start, end int) []storage.BlockID {
	blocks := make([]storage.BlockID, 0, end-start+1)
	for number := start; number <= end; number++ {
		blocks = append(blocks, storage.NewBlockID("student.tbl", number))
	}
	return blocks
}
