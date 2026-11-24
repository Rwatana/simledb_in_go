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

type chunkValueRead struct {
	block  storage.BlockID
	offset int
}

type chunkValueTransaction struct {
	blockSize    int
	pins         []storage.BlockID
	unpins       []storage.BlockID
	pinErrors    map[int]error
	unpinErrors  map[int][]error
	intValues    map[int]map[int]int32
	intErrors    map[int]map[int]error
	stringValues map[int]map[int]string
	stringErrors map[int]map[int]error
	intReads     []chunkValueRead
	stringReads  []chunkValueRead
}

func (transaction *chunkValueTransaction) Pin(block storage.BlockID) error {
	transaction.pins = append(transaction.pins, block)
	return transaction.pinErrors[block.Number()]
}

func (transaction *chunkValueTransaction) BlockSize() (int, error) {
	return transaction.blockSize, nil
}

func (transaction *chunkValueTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	transaction.intReads = append(transaction.intReads, chunkValueRead{block: block, offset: offset})
	if err := transaction.intErrors[block.Number()][offset]; err != nil {
		return 0, err
	}
	return transaction.intValues[block.Number()][offset], nil
}

func (transaction *chunkValueTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	transaction.stringReads = append(transaction.stringReads, chunkValueRead{block: block, offset: offset})
	if err := transaction.stringErrors[block.Number()][offset]; err != nil {
		return "", err
	}
	return transaction.stringValues[block.Number()][offset], nil
}

func (*chunkValueTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }

func (*chunkValueTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func (transaction *chunkValueTransaction) Unpin(block storage.BlockID) error {
	transaction.unpins = append(transaction.unpins, block)
	queued := transaction.unpinErrors[block.Number()]
	if len(queued) == 0 {
		return nil
	}
	transaction.unpinErrors[block.Number()] = queued[1:]
	return queued[0]
}

func TestChunkValueScanReadsTypedValuesAcrossBlocksWithoutAdvancing(t *testing.T) {
	layout := chunkValueLayout(false)
	nameOffset, ok := layout.Offset("name")
	if !ok {
		t.Fatal("name field offset is unavailable")
	}
	transaction := &chunkValueTransaction{
		blockSize: 64,
		intValues: map[int]map[int]int32{
			0: {0: 1, 4: 7},
			1: {0: 1, 4: 9},
		},
		stringValues: map[int]map[int]string{
			0: {nameOffset: "Ada"},
			1: {nameOffset: "Bob"},
		},
	}
	chunk, err := multibuffer.NewChunkScan(transaction, "records.tbl", layout, 0, 1)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	scan, err := NewChunkValueScan(chunk, layout)
	if err != nil {
		t.Fatalf("NewChunkValueScan() error = %v", err)
	}
	var _ query.Scan = scan
	pins := append([]storage.BlockID(nil), transaction.pins...)
	for _, want := range []struct {
		integer int32
		text    string
	}{{7, "Ada"}, {9, "Bob"}} {
		got, err := scan.Next()
		if err != nil || !got {
			t.Fatalf("Next() = (%v, %v), want current record", got, err)
		}
		value, err := scan.GetVal("id")
		if err != nil || !value.Equal(query.NewIntConstant(want.integer)) {
			t.Fatalf("GetVal(id) = (%v, %v), want integer %d", value, err, want.integer)
		}
		if again, err := scan.GetVal("id"); err != nil || !again.Equal(value) {
			t.Fatalf("repeated GetVal(id) = (%v, %v), want unchanged %v", again, err, value)
		}
		value, err = scan.GetVal("name")
		if err != nil || !value.Equal(query.NewStringConstant(want.text)) {
			t.Fatalf("GetVal(name) = (%v, %v), want string %q", value, err, want.text)
		}
		if !reflect.DeepEqual(transaction.pins, pins) {
			t.Fatalf("GetVal changed page pins: got %v, want %v", transaction.pins, pins)
		}
	}
	if got, err := scan.Next(); err != nil || got {
		t.Fatalf("Next() after final block = (%v, %v), want exhaustion", got, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestChunkValueScanPreservesFieldAndStorageErrors(t *testing.T) {
	layout := chunkValueLayout(false)
	nameOffset, _ := layout.Offset("name")
	integerErr := errors.New("integer read failed")
	stringErr := errors.New("string read failed")
	transaction := &chunkValueTransaction{
		blockSize: 64,
		intValues: map[int]map[int]int32{3: {0: 1}},
		intErrors: map[int]map[int]error{3: {4: integerErr}},
		stringErrors: map[int]map[int]error{
			3: {nameOffset: stringErr},
		},
	}
	scan := newChunkValueScanForTest(t, transaction, layout, 3, 3)
	if got, err := scan.Next(); err != nil || !got {
		t.Fatalf("Next() = (%v, %v), want one record", got, err)
	}
	if _, err := scan.GetVal("missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("GetVal(missing) error = %v, want field context", err)
	}
	if _, err := scan.GetVal("id"); !errors.Is(err, integerErr) || !strings.Contains(err.Error(), "block 3") {
		t.Fatalf("GetVal(id) error = %v, want wrapped integer cause and block context", err)
	}
	if _, err := scan.GetVal("name"); !errors.Is(err, stringErr) || !strings.Contains(err.Error(), "block 3") {
		t.Fatalf("GetVal(name) error = %v, want wrapped string cause and block context", err)
	}
}

func TestChunkValueScanRejectsUnsupportedFieldTypeBeforeDataIO(t *testing.T) {
	layout := chunkValueLayout(true)
	transaction := &chunkValueTransaction{
		blockSize: 64,
		intValues: map[int]map[int]int32{0: {0: 1}},
	}
	scan := newChunkValueScanForTest(t, transaction, layout, 0, 0)
	if got, err := scan.Next(); err != nil || !got {
		t.Fatalf("Next() = (%v, %v), want one record", got, err)
	}
	readsBefore := len(transaction.intReads)
	if _, err := scan.GetVal("opaque"); err == nil || !strings.Contains(err.Error(), "unsupported") || !strings.Contains(err.Error(), "opaque") {
		t.Fatalf("GetVal(opaque) error = %v, want unsupported type error", err)
	}
	if len(transaction.intReads) != readsBefore || len(transaction.stringReads) != 0 {
		t.Fatalf("unsupported field caused data read: integer=%v string=%v", transaction.intReads, transaction.stringReads)
	}
}

func TestChunkValueScanRejectsUnpositionedAndClosedScansBeforeFieldIO(t *testing.T) {
	layout := chunkValueLayout(false)
	newTransaction := func(withRow bool) *chunkValueTransaction {
		transaction := &chunkValueTransaction{blockSize: 64}
		if withRow {
			transaction.intValues = map[int]map[int]int32{0: {0: 1, 4: 7}}
		}
		return transaction
	}
	beforeFirstTransaction := newTransaction(true)
	beforeFirst := newChunkValueScanForTest(t, beforeFirstTransaction, layout, 0, 0)
	exhaustedTransaction := newTransaction(false)
	exhausted := newChunkValueScanForTest(t, exhaustedTransaction, layout, 0, 0)
	if got, err := exhausted.Next(); err != nil || got {
		t.Fatalf("exhausted Next() = (%v, %v), want no row", got, err)
	}
	closedTransaction := newTransaction(true)
	closed := newChunkValueScanForTest(t, closedTransaction, layout, 0, 0)
	if got, err := closed.Next(); err != nil || !got {
		t.Fatalf("closed setup Next() = (%v, %v), want one row", got, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() setup error = %v", err)
	}
	partialTransaction := &chunkValueTransaction{
		pinErrors:   map[int]error{1: errors.New("pin failed")},
		unpinErrors: map[int][]error{0: {errors.New("unpin failed")}},
	}
	partialChunk, err := multibuffer.NewChunkScan(partialTransaction, "records.tbl", layout, 0, 1)
	if partialChunk == nil || err == nil {
		t.Fatalf("partial NewChunkScan() = (%v, %v), want retryable partial scan", partialChunk, err)
	}
	partial, err := NewChunkValueScan(partialChunk, layout)
	if err != nil {
		t.Fatalf("wrap partial scan error = %v", err)
	}
	tests := []struct {
		name        string
		scan        *ChunkValueScan
		transaction *chunkValueTransaction
	}{
		{name: "nil adapter"},
		{name: "zero adapter", scan: &ChunkValueScan{}},
		{name: "nil embedded scan", scan: &ChunkValueScan{schema: layout.Schema()}},
		{name: "uninitialized embedded scan", scan: newChunkValueScanForTest(t, &chunkValueTransaction{}, layout, 0, 0)},
		{name: "before first", scan: beforeFirst, transaction: beforeFirstTransaction},
		{name: "exhausted", scan: exhausted, transaction: exhaustedTransaction},
		{name: "closed", scan: closed, transaction: closedTransaction},
		{name: "partial initialization", scan: partial, transaction: partialTransaction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.scan.GetVal("id"); err == nil {
				t.Fatal("GetVal(id) error = nil, want unavailable scan error")
			}
			if test.transaction != nil {
				for _, read := range test.transaction.intReads {
					if read.offset == 4 {
						t.Fatalf("unavailable scan read integer field: %v", test.transaction.intReads)
					}
				}
				if len(test.transaction.stringReads) != 0 {
					t.Fatalf("unavailable scan read string field: %v", test.transaction.stringReads)
				}
			}
		})
	}
	if err := partial.Close(); err != nil {
		t.Fatalf("retry partial cleanup = %v", err)
	}
}

func TestNewChunkValueScanValidatesAdapterInputs(t *testing.T) {
	layout := chunkValueLayout(false)
	if _, err := NewChunkValueScan(nil, layout); err == nil || !strings.Contains(err.Error(), "scan is nil") {
		t.Fatalf("nil scan error = %v, want validation error", err)
	}
	transaction := &chunkValueTransaction{blockSize: 64}
	chunk, err := multibuffer.NewChunkScan(transaction, "records.tbl", layout, 0, 0)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	if _, err := NewChunkValueScan(chunk, nil); err == nil || !strings.Contains(err.Error(), "layout is nil") {
		t.Fatalf("nil layout error = %v, want validation error", err)
	}
	if _, err := NewChunkValueScan(chunk, record.NewLayoutFromMetadata(nil, nil, 0)); err == nil || !strings.Contains(err.Error(), "schema is nil") {
		t.Fatalf("nil schema error = %v, want validation error", err)
	}
	if _, err := NewChunkValueScan(chunk, layout); err != nil {
		t.Fatalf("valid adapter error = %v", err)
	}
	if err := chunk.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func newChunkValueScanForTest(t *testing.T, transaction *chunkValueTransaction, layout *record.Layout, start, end int) *ChunkValueScan {
	t.Helper()
	chunk, err := multibuffer.NewChunkScan(transaction, "records.tbl", layout, start, end)
	if err != nil {
		t.Fatalf("NewChunkScan() error = %v", err)
	}
	scan, err := NewChunkValueScan(chunk, layout)
	if err != nil {
		t.Fatalf("NewChunkValueScan() error = %v", err)
	}
	return scan
}

func chunkValueLayout(unsupported bool) *record.Layout {
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 8)
	if unsupported {
		schema.AddField("opaque", record.FieldType(99), 1)
	}
	return record.NewLayout(&schema)
}
