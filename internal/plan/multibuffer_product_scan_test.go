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

type multibufferProductLeftRows struct {
	values            []int32
	position          int
	beforeFirstCalls  int
	nextCalls         int
	beforeFirstErr    error
	beforeFirstErrors []error
	nextErrors        []error
}

func (scan *multibufferProductLeftRows) BeforeFirst() error {
	scan.beforeFirstCalls++
	if len(scan.beforeFirstErrors) != 0 {
		err := scan.beforeFirstErrors[0]
		scan.beforeFirstErrors = scan.beforeFirstErrors[1:]
		if err != nil {
			return err
		}
	}
	if scan.beforeFirstErr != nil {
		return scan.beforeFirstErr
	}
	scan.position = -1
	return nil
}

func (scan *multibufferProductLeftRows) Next() (bool, error) {
	scan.nextCalls++
	if len(scan.nextErrors) != 0 {
		err := scan.nextErrors[0]
		scan.nextErrors = scan.nextErrors[1:]
		if err != nil {
			return false, err
		}
	}
	scan.position++
	return scan.position < len(scan.values), nil
}

func (scan *multibufferProductLeftRows) GetInt(string) (int32, error) { return 0, nil }

func (scan *multibufferProductLeftRows) GetString(string) (string, error) { return "", nil }

func (scan *multibufferProductLeftRows) GetVal(fieldName string) (query.Constant, error) {
	if fieldName != "left" || scan.position < 0 || scan.position >= len(scan.values) {
		return query.Constant{}, errors.New("left value is unavailable")
	}
	return query.NewIntConstant(scan.values[scan.position]), nil
}

func (*multibufferProductLeftRows) HasField(fieldName string) bool { return fieldName == "left" }

func (*multibufferProductLeftRows) Close() error { return nil }

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

func TestMultibufferProductScanAdvancesLeftMajorPairsWithinCurrentChunk(t *testing.T) {
	layout := chunkValueLayout(false)
	flagOffset, _ := layout.Offset("id")
	slotSize := layout.SlotSize()
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			intValues: map[int]map[int]int32{
				0: {0: 1, slotSize: 1, flagOffset: 100, slotSize + flagOffset: 200},
			},
		},
		fileSize:  1,
		available: 4,
	}
	left := &multibufferProductLeftRows{values: []int32{10, 20}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	initialPins := append([]storage.BlockID(nil), transaction.pins...)
	wantPairs := [][2]int32{{10, 100}, {10, 200}, {20, 100}, {20, 200}}
	for i, want := range wantPairs {
		ok, err := scan.Next()
		if err != nil || !ok {
			t.Fatalf("Next() pair %d = (%v, %v), want a row", i, ok, err)
		}
		leftValue, err := scan.product.GetVal("left")
		if err != nil {
			t.Fatalf("left value for pair %d: %v", i, err)
		}
		rightValue, err := scan.product.GetVal("id")
		if err != nil {
			t.Fatalf("right value for pair %d: %v", i, err)
		}
		leftInteger, leftIsInt := leftValue.AsInt()
		rightInteger, rightIsInt := rightValue.AsInt()
		if !leftIsInt || !rightIsInt {
			t.Fatalf("pair %d has non-integer values (%v, %v)", i, leftValue, rightValue)
		}
		got := [2]int32{leftInteger, rightInteger}
		if got != want {
			t.Fatalf("pair %d = %v, want %v", i, got, want)
		}
	}
	readsAtExhaustion := len(transaction.intReads)
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after current chunk = (%v, %v), want exhaustion", ok, err)
	}
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("repeated Next() after exhaustion = (%v, %v), want stable exhaustion", ok, err)
	}
	if len(transaction.intReads) != readsAtExhaustion {
		t.Fatalf("exhausted Next() read %d more values, want no reads", len(transaction.intReads)-readsAtExhaustion)
	}
	if !reflect.DeepEqual(transaction.pins, initialPins) {
		t.Fatalf("traversal changed pins: got %v, want %v", transaction.pins, initialPins)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() after exhaustion: %v", err)
	}
	if left.beforeFirstCalls != 2 {
		t.Fatalf("left rewind calls = %d, want constructor and explicit rewind", left.beforeFirstCalls)
	}
	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() after rewind = (%v, %v), want first pair", ok, err)
	}
	if !reflect.DeepEqual(transaction.pins, initialPins) {
		t.Fatalf("rewind changed pins: got %v, want %v", transaction.pins, initialPins)
	}
}

func TestMultibufferProductScanEmptyRightRelationDoesNotReadLeft(t *testing.T) {
	transaction := &multibufferProductTransaction{fileSize: 0, available: 4}
	left := &multibufferProductLeftRows{values: []int32{10}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "empty", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() on empty relation: %v", err)
	}
	for i := 0; i < 2; i++ {
		if ok, err := scan.Next(); err != nil || ok {
			t.Fatalf("Next() on empty relation = (%v, %v), want no rows", ok, err)
		}
	}
	if left.beforeFirstCalls != 0 || left.nextCalls != 0 || transaction.availableCalls != 0 || len(transaction.pins) != 0 {
		t.Fatalf("empty relation touched inputs: left rewinds=%d reads=%d available=%d pins=%v", left.beforeFirstCalls, left.nextCalls, transaction.availableCalls, transaction.pins)
	}
}

func TestMultibufferProductScanPreservesProductFactoryAndInputErrors(t *testing.T) {
	newScan := func(t *testing.T) (*MultibufferProductScan, *multibufferProductTransaction, *multibufferProductLeftRows) {
		t.Helper()
		layout := chunkValueLayout(false)
		idOffset, _ := layout.Offset("id")
		transaction := &multibufferProductTransaction{
			chunkValueTransaction: chunkValueTransaction{
				blockSize: 128,
				intValues: map[int]map[int]int32{0: {0: 1, idOffset: 100}},
			},
			fileSize:  1,
			available: 4,
		}
		left := &multibufferProductLeftRows{values: []int32{10}, position: -1}
		scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
		if err != nil {
			t.Fatalf("NewMultibufferProductScan() error = %v", err)
		}
		return scan, transaction, left
	}

	t.Run("factory error can be retried", func(t *testing.T) {
		scan, transaction, left := newScan(t)
		factoryErr := errors.New("product constructor failed")
		scan.newProductScan = func(query.Scan, query.Scan) (*query.ProductScan, error) {
			return nil, factoryErr
		}
		if err := scan.BeforeFirst(); !errors.Is(err, factoryErr) || !strings.Contains(err.Error(), "student.tbl") {
			t.Fatalf("BeforeFirst() = %v, want contextual product-construction error", err)
		}
		if ok, err := scan.Next(); ok || !errors.Is(err, factoryErr) || !strings.Contains(err.Error(), "student.tbl") {
			t.Fatalf("Next() = (%v, %v), want contextual product-construction error", ok, err)
		}
		if left.nextCalls != 0 || len(transaction.pins) != 1 {
			t.Fatalf("failed product construction advanced or repinned inputs: reads=%d pins=%v", left.nextCalls, transaction.pins)
		}
		scan.newProductScan = query.NewProductScan
		if ok, err := scan.Next(); err != nil || !ok {
			t.Fatalf("Next() after factory retry = (%v, %v), want first row", ok, err)
		}
	})

	t.Run("nil factory and nil product are rejected", func(t *testing.T) {
		scan, _, _ := newScan(t)
		scan.newProductScan = nil
		if ok, err := scan.Next(); ok || err == nil || !strings.Contains(err.Error(), "factory is nil") {
			t.Fatalf("Next() with nil factory = (%v, %v), want factory error", ok, err)
		}
		scan.newProductScan = func(query.Scan, query.Scan) (*query.ProductScan, error) { return nil, nil }
		if ok, err := scan.Next(); ok || err == nil || !strings.Contains(err.Error(), "returned nil") {
			t.Fatalf("Next() with nil product = (%v, %v), want nil product error", ok, err)
		}
	})

	t.Run("closed chunk is reported as unavailable", func(t *testing.T) {
		scan, _, _ := newScan(t)
		if err := scan.Close(); err != nil {
			t.Fatalf("Close() = %v", err)
		}
		if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "current chunk is unavailable") {
			t.Fatalf("BeforeFirst() after close = %v, want unavailable chunk error", err)
		}
		if ok, err := scan.Next(); ok || err == nil || !strings.Contains(err.Error(), "current chunk is unavailable") {
			t.Fatalf("Next() after close = (%v, %v), want unavailable chunk error", ok, err)
		}
	})

	t.Run("missing current chunk is reported", func(t *testing.T) {
		scan, _, _ := newScan(t)
		retained := scan.right
		scan.right = nil
		if ok, err := scan.Next(); ok || err == nil || !strings.Contains(err.Error(), "current chunk is unavailable") {
			t.Fatalf("Next() without a current chunk = (%v, %v), want unavailable chunk error", ok, err)
		}
		scan.right = retained
		if err := scan.Close(); err != nil {
			t.Fatalf("Close() retained test chunk: %v", err)
		}
	})

	t.Run("left next error", func(t *testing.T) {
		scan, _, left := newScan(t)
		leftErr := errors.New("left next failed")
		left.nextErrors = []error{leftErr, nil}
		if ok, err := scan.Next(); ok || !errors.Is(err, leftErr) || !strings.Contains(err.Error(), "left") {
			t.Fatalf("Next() = (%v, %v), want contextual left error", ok, err)
		}
		if ok, err := scan.Next(); err != nil || !ok {
			t.Fatalf("Next() after left retry = (%v, %v), want first row", ok, err)
		}
	})

	t.Run("right next error", func(t *testing.T) {
		scan, _, _ := newScan(t)
		rightErr := errors.New("right record flag failed")
		scan.transaction.(*multibufferProductTransaction).intErrors = map[int]map[int]error{0: {0: rightErr}}
		if ok, err := scan.Next(); ok || !errors.Is(err, rightErr) || !strings.Contains(err.Error(), "right") {
			t.Fatalf("Next() = (%v, %v), want contextual right error", ok, err)
		}
	})

	t.Run("left rewind error then retry", func(t *testing.T) {
		scan, _, left := newScan(t)
		leftErr := errors.New("left rewind failed")
		left.beforeFirstErr = leftErr
		if err := scan.BeforeFirst(); !errors.Is(err, leftErr) || !strings.Contains(err.Error(), "left") {
			t.Fatalf("BeforeFirst() error = %v, want contextual left cause", err)
		}
		left.beforeFirstErr = nil
		if err := scan.BeforeFirst(); err != nil {
			t.Fatalf("BeforeFirst() retry error = %v", err)
		}
		if ok, err := scan.Next(); err != nil || !ok {
			t.Fatalf("Next() after rewind retry = (%v, %v), want first row", ok, err)
		}
	})

	t.Run("right rewind error", func(t *testing.T) {
		scan, _, _ := newScan(t)
		if err := scan.right.ChunkScan.Close(); err != nil {
			t.Fatalf("close right test scan: %v", err)
		}
		if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "right") {
			t.Fatalf("BeforeFirst() error = %v, want contextual right rewind failure", err)
		}
	})

	t.Run("nil and zero value", func(t *testing.T) {
		var nilScan *MultibufferProductScan
		if _, err := nilScan.Next(); err == nil {
			t.Fatal("nil Next() error = nil, want validation error")
		}
		if err := nilScan.BeforeFirst(); err == nil {
			t.Fatal("nil BeforeFirst() error = nil, want validation error")
		}
		zero := &MultibufferProductScan{}
		if _, err := zero.Next(); err == nil {
			t.Fatal("zero Next() error = nil, want validation error")
		}
		if err := zero.BeforeFirst(); err == nil {
			t.Fatal("zero BeforeFirst() error = nil, want validation error")
		}
	})
}

func TestMultibufferProductScanMovesAcrossBoundedChunksInLeftMajorOrder(t *testing.T) {
	layout := chunkValueLayout(false)
	idOffset, _ := layout.Offset("id")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			intValues: map[int]map[int]int32{
				0: {0: 1, idOffset: 100},
				1: {0: 1, idOffset: 101},
				2: {0: 1, idOffset: 102},
				3: {0: 1, idOffset: 103},
				4: {0: 1, idOffset: 104},
			},
		},
		fileSize:  5,
		available: 4,
	}
	left := &multibufferProductLeftRows{values: []int32{10, 20}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if scan.chunkSize != 2 {
		t.Fatalf("chunk size = %d, want 2", scan.chunkSize)
	}
	wantPairs := [][2]int32{
		{10, 100}, {10, 101}, {20, 100}, {20, 101},
		{10, 102}, {10, 103}, {20, 102}, {20, 103},
		{10, 104}, {20, 104},
	}
	for i, want := range wantPairs {
		ok, err := scan.Next()
		if err != nil || !ok {
			t.Fatalf("Next() pair %d = (%v, %v), want a row", i, ok, err)
		}
		pair := multibufferProductPair(t, scan)
		if pair != want {
			t.Fatalf("pair %d = %v, want %v", i, pair, want)
		}
		if i == 3 {
			if got, want := transaction.pins, multibufferProductBlocks(0, 1); !reflect.DeepEqual(got, want) {
				t.Fatalf("current-chunk rows changed pins: got %v, want %v", got, want)
			}
			if len(transaction.unpins) != 0 {
				t.Fatalf("current chunk was released before exhaustion: %v", transaction.unpins)
			}
		}
	}
	wantPins := multibufferProductBlocks(0, 4)
	if !reflect.DeepEqual(transaction.pins, wantPins) {
		t.Fatalf("pins = %v, want block order and bounded final chunk %v", transaction.pins, wantPins)
	}
	wantUnpins := append(multibufferProductBlocks(0, 1), multibufferProductBlocks(2, 3)...)
	if !reflect.DeepEqual(transaction.unpins, wantUnpins) {
		t.Fatalf("intermediate chunk releases = %v, want %v", transaction.unpins, wantUnpins)
	}
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after final row = (%v, %v), want exhaustion", ok, err)
	}
	readsAtEnd := len(transaction.intReads)
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("repeated Next() after final chunk = (%v, %v), want stable exhaustion", ok, err)
	}
	if len(transaction.intReads) != readsAtEnd || !reflect.DeepEqual(transaction.pins, wantPins) || !reflect.DeepEqual(transaction.unpins, wantUnpins) {
		t.Fatalf("stable exhaustion changed reads or pins: reads=%v pins=%v unpins=%v", transaction.intReads, transaction.pins, transaction.unpins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() final chunk: %v", err)
	}
}

func TestMultibufferProductScanSkipsEmptyEarlierChunks(t *testing.T) {
	layout := chunkValueLayout(false)
	idOffset, _ := layout.Offset("id")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			intValues: map[int]map[int]int32{4: {0: 1, idOffset: 104}},
		},
		fileSize:  5,
		available: 4,
	}
	left := &multibufferProductLeftRows{values: []int32{20}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	ok, err := scan.Next()
	if err != nil || !ok {
		t.Fatalf("Next() after empty earlier chunks = (%v, %v), want final-chunk row", ok, err)
	}
	if got, want := multibufferProductPair(t, scan), [2]int32{20, 104}; got != want {
		t.Fatalf("final-chunk pair = %v, want %v", got, want)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins across empty chunks = %v, want %v", got, want)
	}
	if got, want := transaction.unpins, multibufferProductBlocks(0, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("releases before final chunk = %v, want %v", got, want)
	}
	if left.beforeFirstCalls != 3 {
		t.Fatalf("left rewind calls = %d, want initial plus both chunk transitions", left.beforeFirstCalls)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() final chunk: %v", err)
	}
}

func TestMultibufferProductScanRetriesLaterChunkPinFailure(t *testing.T) {
	pinErr := errors.New("next chunk pin failed")
	layout := chunkValueLayout(false)
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			pinErrors: map[int]error{2: pinErr},
		},
		fileSize:  4,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftRows{position: -1}, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, pinErr) {
		t.Fatalf("Next() at first chunk boundary = (%v, %v), want wrapped next-chunk pin error", ok, err)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins after failed transition = %v, want no block beyond failed block 2", got)
	}
	if scan.nextBlock != 2 {
		t.Fatalf("next block after failed pin = %d, want retryable block 2", scan.nextBlock)
	}
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "current chunk is unavailable") {
		t.Fatalf("BeforeFirst() while a later chunk is unavailable = %v, want unavailable chunk error", err)
	}
	delete(transaction.pinErrors, 2)
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after pin retry with empty left input = (%v, %v), want exhaustion", ok, err)
	}
	wantPins := append(multibufferProductBlocks(0, 2), multibufferProductBlocks(2, 3)...)
	if !reflect.DeepEqual(transaction.pins, wantPins) {
		t.Fatalf("pins after retry = %v, want failed block 2 retried before block 3: %v", transaction.pins, wantPins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() after retry: %v", err)
	}
}

func TestMultibufferProductScanRetriesOldChunkReleaseBeforePinningNext(t *testing.T) {
	releaseErr := errors.New("old chunk release failed")
	layout := chunkValueLayout(false)
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   128,
			unpinErrors: map[int][]error{0: {releaseErr}},
		},
		fileSize:  4,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftRows{position: -1}, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, releaseErr) {
		t.Fatalf("Next() at first chunk boundary = (%v, %v), want release error", ok, err)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("failed release pinned the next chunk: got %v, want %v", got, want)
	}
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after release retry and empty left input = (%v, %v), want exhaustion", ok, err)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("pins after release retry = %v, want later chunk after old releases", got)
	}
	wantUnpinsBeforeClose := []storage.BlockID{
		storage.NewBlockID("student.tbl", 0),
		storage.NewBlockID("student.tbl", 1),
		storage.NewBlockID("student.tbl", 0),
	}
	if !reflect.DeepEqual(transaction.unpins, wantUnpinsBeforeClose) {
		t.Fatalf("unpin attempts before final Close = %v, want failed-page-only retry and retained final chunk %v", transaction.unpins, wantUnpinsBeforeClose)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() after release retry: %v", err)
	}
	wantUnpins := append(wantUnpinsBeforeClose, multibufferProductBlocks(2, 3)...)
	if !reflect.DeepEqual(transaction.unpins, wantUnpins) {
		t.Fatalf("unpins after final Close = %v, want %v", transaction.unpins, wantUnpins)
	}
}

func TestMultibufferProductScanRetainsPartiallyPinnedLaterChunk(t *testing.T) {
	pinErr := errors.New("later chunk pin failed")
	releaseErr := errors.New("partial later chunk release failed")
	layout := chunkValueLayout(false)
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   128,
			pinErrors:   map[int]error{3: pinErr},
			unpinErrors: map[int][]error{2: {releaseErr}},
		},
		fileSize:  4,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftRows{position: -1}, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, pinErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("Next() at partial chunk failure = (%v, %v), want both pin and cleanup causes", ok, err)
	}
	if scan.right == nil || scan.nextBlock != 2 {
		t.Fatalf("partial chunk state = right %v, next block %d; want retained cleanup handle and unchanged next block", scan.right, scan.nextBlock)
	}
	if err := scan.BeforeFirst(); !errors.Is(err, pinErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("BeforeFirst() with partial later chunk = %v, want both causes", err)
	}
	pinsAtFailure := append([]storage.BlockID(nil), transaction.pins...)
	if ok, err := scan.Next(); ok || !errors.Is(err, pinErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("Next() with retained partial chunk = (%v, %v), want original causes without retrying it", ok, err)
	}
	if !reflect.DeepEqual(transaction.pins, pinsAtFailure) {
		t.Fatalf("Next() retried pins before cleanup: got %v, want %v", transaction.pins, pinsAtFailure)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() retry of partial later chunk: %v", err)
	}
	if got, want := transaction.unpins[len(transaction.unpins)-1], storage.NewBlockID("student.tbl", 2); got != want {
		t.Fatalf("last cleanup retry = %v, want retained block %v", got, want)
	}
}

func TestMultibufferProductScanDoesNotReplaceChunkPastFileEnd(t *testing.T) {
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{blockSize: 128},
		fileSize:              1,
		available:             4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftRows{position: -1}, "student", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if err := scan.useNextChunk(); err != nil {
		t.Fatalf("useNextChunk() after file end = %v, want nil", err)
	}
	if got, want := transaction.pins, multibufferProductBlocks(0, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("pin attempts after file end = %v, want unchanged current chunk %v", transaction.pins, want)
	}
	if len(transaction.unpins) != 0 {
		t.Fatalf("useNextChunk() released the final chunk: %v", transaction.unpins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() final chunk: %v", err)
	}
}

func TestMultibufferProductScanRetriesLaterChunkAfterLeftRewindFailure(t *testing.T) {
	leftErr := errors.New("later left rewind failed")
	layout := chunkValueLayout(false)
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{blockSize: 128},
		fileSize:              4,
		available:             4,
	}
	left := &multibufferProductLeftRows{position: -1, beforeFirstErrors: []error{nil, leftErr, nil}}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, leftErr) {
		t.Fatalf("Next() at left reset failure = (%v, %v), want left rewind cause", ok, err)
	}
	wantAfterResetFailure := append(multibufferProductBlocks(0, 1), multibufferProductBlocks(2, 3)...)
	if !reflect.DeepEqual(transaction.unpins, wantAfterResetFailure) {
		t.Fatalf("failed left reset releases = %v, want current then failed new range %v", transaction.unpins, wantAfterResetFailure)
	}
	if scan.nextBlock != 2 || scan.right != nil {
		t.Fatalf("failed rewind state = next block %d, right %v; want unchanged block and no new chunk", scan.nextBlock, scan.right)
	}
	if ok, err := scan.Next(); ok || err != nil {
		t.Fatalf("Next() after rewind retry with empty left input = (%v, %v), want eventual exhaustion", ok, err)
	}
	wantPins := append(multibufferProductBlocks(0, 1), multibufferProductBlocks(2, 3)...)
	wantPins = append(wantPins, multibufferProductBlocks(2, 3)...)
	if !reflect.DeepEqual(transaction.pins, wantPins) {
		t.Fatalf("pins after left rewind retry = %v, want %v", transaction.pins, wantPins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() after left rewind retry: %v", err)
	}
}

func TestMultibufferProductScanRetainsLaterChunkWhenRewindAndCleanupFail(t *testing.T) {
	leftErr := errors.New("later left rewind failed")
	releaseErr := errors.New("later chunk release failed")
	layout := chunkValueLayout(false)
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:   128,
			unpinErrors: map[int][]error{2: {releaseErr}},
		},
		fileSize:  4,
		available: 4,
	}
	left := &multibufferProductLeftRows{position: -1, beforeFirstErrors: []error{nil, leftErr}}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, leftErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("Next() at rewind cleanup failure = (%v, %v), want both causes", ok, err)
	}
	if scan.right == nil || scan.nextBlock != 2 {
		t.Fatalf("retained later chunk state = right %v next block %d, want cleanup handle and unchanged range", scan.right, scan.nextBlock)
	}
	if ok, err := scan.Next(); ok || !errors.Is(err, leftErr) || !errors.Is(err, releaseErr) {
		t.Fatalf("Next() with retained rewind failure = (%v, %v), want stored causes", ok, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() retry after left rewind failure: %v", err)
	}
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("BeforeFirst() after cleanup = %v, want closed scan error", err)
	}
}

func TestMultibufferProductScanReleasesFinalChunkOnCloseAndRejectsReuse(t *testing.T) {
	layout := chunkValueLayout(false)
	idOffset, _ := layout.Offset("id")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			intValues: map[int]map[int]int32{0: {0: 1, idOffset: 100}},
		},
		fileSize:  1,
		available: 4,
	}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftRows{values: []int32{10}, position: -1}, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if got, want := transaction.unpins, multibufferProductBlocks(0, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("Close() final page releases = %v, want %v", got, want)
	}
	if ok, err := scan.Next(); ok || err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Next() after Close() = (%v, %v), want closed scan error", ok, err)
	}
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("BeforeFirst() after Close() = %v, want closed scan error", err)
	}
}

func TestMultibufferProductScanReadsValuesFromCurrentProductRow(t *testing.T) {
	layout := chunkValueLayout(false)
	idOffset, _ := layout.Offset("id")
	nameOffset, _ := layout.Offset("name")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize:    128,
			intValues:    map[int]map[int]int32{0: {0: 1, idOffset: 100}},
			stringValues: map[int]map[int]string{0: {nameOffset: "Ada"}},
		},
		fileSize:  1,
		available: 4,
	}
	left := &multibufferProductLeftRows{values: []int32{10}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if _, err := scan.GetVal("left"); err == nil || !strings.Contains(err.Error(), "current product row") {
		t.Fatalf("GetVal() before Next = %v, want no-current-row error", err)
	}
	if scan.product != nil || left.nextCalls != 0 || len(transaction.intReads) != 0 || len(transaction.stringReads) != 0 {
		t.Fatalf("GetVal() before Next touched scan inputs: product=%v left reads=%d int reads=%v string reads=%v", scan.product, left.nextCalls, transaction.intReads, transaction.stringReads)
	}
	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() = (%v, %v), want a current product row", ok, err)
	}
	pins := append([]storage.BlockID(nil), transaction.pins...)
	leftReads := left.nextCalls
	checks := []struct {
		field string
		want  query.Constant
	}{
		{"left", query.NewIntConstant(10)},
		{"id", query.NewIntConstant(100)},
		{"name", query.NewStringConstant("Ada")},
	}
	for _, check := range checks {
		for attempt := 0; attempt < 2; attempt++ {
			got, err := scan.GetVal(check.field)
			if err != nil || !got.Equal(check.want) {
				t.Fatalf("GetVal(%q) attempt %d = (%v, %v), want %v", check.field, attempt, got, err, check.want)
			}
		}
	}
	if left.nextCalls != leftReads || !reflect.DeepEqual(transaction.pins, pins) {
		t.Fatalf("GetVal moved the product or changed pins: left reads %d -> %d pins %v -> %v", leftReads, left.nextCalls, pins, transaction.pins)
	}
	if _, err := scan.GetVal("missing"); err == nil || !strings.Contains(err.Error(), "missing") || !strings.Contains(err.Error(), "multibuffer product scan") {
		t.Fatalf("GetVal(missing) error = %v, want field and product-scan context", err)
	}
	if left.nextCalls != leftReads || !reflect.DeepEqual(transaction.pins, pins) {
		t.Fatalf("failed field lookup moved the product or changed pins: left reads=%d pins=%v", left.nextCalls, transaction.pins)
	}
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after one pair = (%v, %v), want exhaustion", ok, err)
	}
	if _, err := scan.GetVal("left"); err == nil || !strings.Contains(err.Error(), "current product row") {
		t.Fatalf("GetVal() after exhaustion = %v, want no-current-row error", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() = %v", err)
	}
	if _, err := scan.GetVal("left"); err == nil || !strings.Contains(err.Error(), "current product row") {
		t.Fatalf("GetVal() after BeforeFirst = %v, want no-current-row error", err)
	}
	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() after rewind = (%v, %v), want row", ok, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if _, err := scan.GetVal("left"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("GetVal() after Close = %v, want closed-scan error", err)
	}
}

func TestMultibufferProductScanReportsFieldMembershipWithoutReadingRows(t *testing.T) {
	layout := chunkValueLayout(false)
	idOffset, _ := layout.Offset("id")
	transaction := &multibufferProductTransaction{
		chunkValueTransaction: chunkValueTransaction{
			blockSize: 128,
			intValues: map[int]map[int]int32{0: {0: 1, idOffset: 100}},
		},
		fileSize:  1,
		available: 4,
	}
	left := &multibufferProductLeftRows{values: []int32{10}, position: -1}
	scan, err := NewMultibufferProductScan(transaction, left, "student", layout)
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	initialPins := append([]storage.BlockID(nil), transaction.pins...)
	if !scan.HasField("left") || !scan.HasField("id") || !scan.HasField("name") || scan.HasField("missing") {
		t.Fatal("HasField() did not report the union of left and right schema fields")
	}
	if scan.product != nil || left.nextCalls != 0 || left.beforeFirstCalls != 1 || len(transaction.intReads) != 0 || len(transaction.stringReads) != 0 {
		t.Fatalf("HasField() accessed rows: product=%v left next=%d rewind=%d int reads=%v string reads=%v", scan.product, left.nextCalls, left.beforeFirstCalls, transaction.intReads, transaction.stringReads)
	}
	if !reflect.DeepEqual(transaction.pins, initialPins) || len(transaction.unpins) != 0 {
		t.Fatalf("HasField() changed pins: pins=%v unpins=%v", transaction.pins, transaction.unpins)
	}
	if ok, err := scan.Next(); err != nil || !ok {
		t.Fatalf("Next() = (%v, %v), want a product row", ok, err)
	}
	if !scan.HasField("left") || !scan.HasField("id") {
		t.Fatal("HasField() lost schema membership on a current row")
	}
	if ok, err := scan.Next(); err != nil || ok {
		t.Fatalf("Next() after one product row = (%v, %v), want exhaustion", ok, err)
	}
	if !scan.HasField("left") || !scan.HasField("id") {
		t.Fatal("HasField() changed after exhaustion")
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() after exhaustion: %v", err)
	}
	if !scan.HasField("left") || !scan.HasField("id") {
		t.Fatal("HasField() changed after rewind")
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	closedPins := append([]storage.BlockID(nil), transaction.unpins...)
	if !scan.HasField("left") || !scan.HasField("id") || scan.HasField("missing") {
		t.Fatal("HasField() should remain schema-based after Close")
	}
	if !reflect.DeepEqual(transaction.unpins, closedPins) || left.nextCalls != 2 {
		t.Fatalf("HasField() changed closed scan state: unpins=%v left next=%d", transaction.unpins, left.nextCalls)
	}
}

func TestMultibufferProductScanReportsRightFieldsForEmptyRelation(t *testing.T) {
	transaction := &multibufferProductTransaction{fileSize: 0}
	scan, err := NewMultibufferProductScan(transaction, &multibufferProductLeftProbe{}, "empty", chunkValueLayout(false))
	if err != nil {
		t.Fatalf("NewMultibufferProductScan() error = %v", err)
	}
	if !scan.HasField("id") || !scan.HasField("name") || scan.HasField("missing") {
		t.Fatal("HasField() did not use declared schema fields for an empty right relation")
	}
	if len(transaction.pins) != 0 || len(transaction.unpins) != 0 || transaction.availableCalls != 0 {
		t.Fatalf("HasField() touched empty relation resources: pins=%v unpins=%v buffer checks=%d", transaction.pins, transaction.unpins, transaction.availableCalls)
	}
}

func TestMultibufferProductScanHasFieldAcceptsNilZeroAndSchemaLessScans(t *testing.T) {
	var nilScan *MultibufferProductScan
	if nilScan.HasField("id") {
		t.Fatal("nil HasField() = true, want false")
	}
	if (&MultibufferProductScan{}).HasField("id") {
		t.Fatal("zero-value HasField() = true, want false")
	}
	left := &multibufferProductLeftRows{values: []int32{10}, position: -1}
	scan := &MultibufferProductScan{transaction: &multibufferProductTransaction{}, left: left}
	if scan.HasField("left") {
		t.Fatal("HasField() on a schema-less partial scan = true, want false")
	}
}

func TestMultibufferProductScanGetValRejectsNilAndZeroValue(t *testing.T) {
	var nilScan *MultibufferProductScan
	if _, err := nilScan.GetVal("left"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("nil GetVal() error = %v, want initialization error", err)
	}
	if _, err := (&MultibufferProductScan{}).GetVal("left"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value GetVal() error = %v, want initialization error", err)
	}
}

func multibufferProductPair(t *testing.T, scan *MultibufferProductScan) [2]int32 {
	t.Helper()
	left, err := scan.product.GetVal("left")
	if err != nil {
		t.Fatalf("read product left value: %v", err)
	}
	right, err := scan.product.GetVal("id")
	if err != nil {
		t.Fatalf("read product right value: %v", err)
	}
	leftValue, leftIsInt := left.AsInt()
	rightValue, rightIsInt := right.AsInt()
	if !leftIsInt || !rightIsInt {
		t.Fatalf("product values = (%v, %v), want integer values", left, right)
	}
	return [2]int32{leftValue, rightValue}
}

func multibufferProductBlocks(start, end int) []storage.BlockID {
	blocks := make([]storage.BlockID, 0, end-start+1)
	for number := start; number <= end; number++ {
		blocks = append(blocks, storage.NewBlockID("student.tbl", number))
	}
	return blocks
}
