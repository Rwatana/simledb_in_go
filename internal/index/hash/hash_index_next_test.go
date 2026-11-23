package hash

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type hashIndexNextTransaction struct {
	blockSize      int
	integers       map[int]int32
	strings        map[int]string
	blockSizeErr   error
	getIntErrAt    int
	getIntErr      error
	getStringErrAt int
	getStringErr   error
}

func (transaction *hashIndexNextTransaction) Pin(storage.BlockID) error { return nil }

func (transaction *hashIndexNextTransaction) BlockSize() (int, error) {
	return transaction.blockSize, transaction.blockSizeErr
}

func (transaction *hashIndexNextTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	if offset == transaction.getIntErrAt && transaction.getIntErr != nil {
		return 0, transaction.getIntErr
	}
	return transaction.integers[offset], nil
}

func (transaction *hashIndexNextTransaction) GetString(_ storage.BlockID, offset int) (string, error) {
	if offset == transaction.getStringErrAt && transaction.getStringErr != nil {
		return "", transaction.getStringErr
	}
	return transaction.strings[offset], nil
}

func (*hashIndexNextTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }

func (*hashIndexNextTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func (*hashIndexNextTransaction) Size(string) (int, error) { return 1, nil }

func (*hashIndexNextTransaction) Append(string) (storage.BlockID, error) {
	return storage.BlockID{}, nil
}

func (*hashIndexNextTransaction) Unpin(storage.BlockID) error { return nil }

func newHashIndexNextTestIndex(t *testing.T, schema *record.Schema, values []query.Constant, key query.Constant) (*HashIndex, *hashIndexNextTransaction) {
	t.Helper()
	layout := record.NewLayout(schema)
	transaction := &hashIndexNextTransaction{
		blockSize:      layout.SlotSize() * len(values),
		integers:       make(map[int]int32),
		strings:        make(map[int]string),
		getIntErrAt:    -1,
		getStringErrAt: -1,
	}
	datumOffset, ok := layout.Offset("dataval")
	if !ok && len(values) > 0 {
		t.Fatal("test schema does not contain dataval")
	}
	fieldType, hasField := schema.TypeOf("dataval")
	for slot, value := range values {
		position := slot * layout.SlotSize()
		transaction.integers[position] = 1
		if !hasField {
			continue
		}
		switch fieldType {
		case record.IntegerField:
			integer, valid := value.AsInt()
			if !valid {
				t.Fatalf("test value %v is not an integer", value)
			}
			transaction.integers[position+datumOffset] = integer
		case record.VarcharField:
			text, valid := value.AsString()
			if !valid {
				t.Fatalf("test value %v is not a string", value)
			}
			transaction.strings[position+datumOffset] = text
		}
	}
	index := NewHashIndex(transaction, "student_id", layout)
	if err := index.BeforeFirst(key); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	return index, transaction
}

func TestHashIndexNextSkipsIntegerBucketCollisionsAndReturnsEveryMatch(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("dataval")
	key := query.NewIntConstant(42)
	index, transaction := newHashIndexNextTestIndex(t, schema, []query.Constant{
		query.NewIntConstant(7), query.NewIntConstant(42), query.NewIntConstant(42),
	}, key)

	for position, want := range []bool{true, true, false} {
		got, err := index.Next()
		if err != nil || got != want {
			t.Fatalf("Next() #%d = (%t, %v), want (%t, nil)", position+1, got, err, want)
		}
	}
	if transaction.integers[0] != 1 || transaction.integers[8] != 1 || transaction.integers[16] != 1 {
		t.Fatalf("test records were not marked occupied: %v", transaction.integers)
	}
}

func TestHashIndexNextComparesVarcharEntries(t *testing.T) {
	schema := &record.Schema{}
	schema.AddVarcharField("dataval", 12)
	index, _ := newHashIndexNextTestIndex(t, schema, []query.Constant{
		query.NewStringConstant("collision"), query.NewStringConstant("wanted"),
	}, query.NewStringConstant("wanted"))

	if got, err := index.Next(); err != nil || !got {
		t.Fatalf("Next() matching varchar = (%t, %v), want (true, nil)", got, err)
	}
	if got, err := index.Next(); err != nil || got {
		t.Fatalf("Next() after final varchar = (%t, %v), want (false, nil)", got, err)
	}
}

func TestHashIndexNextTreatsDifferentSearchAndFieldTypesAsNonmatches(t *testing.T) {
	schema := &record.Schema{}
	schema.AddVarcharField("dataval", 12)
	index, _ := newHashIndexNextTestIndex(t, schema, []query.Constant{query.NewStringConstant("42")}, query.NewIntConstant(42))

	if got, err := index.Next(); err != nil || got {
		t.Fatalf("Next() with mixed key types = (%t, %v), want (false, nil)", got, err)
	}
}

func TestHashIndexNextRejectsInvalidSearchStateAndLayout(t *testing.T) {
	validSchema := &record.Schema{}
	validSchema.AddIntegerField("dataval")
	validLayout := record.NewLayout(validSchema)
	tests := []struct {
		name  string
		index *HashIndex
	}{
		{name: "nil receiver"},
		{name: "missing scan", index: NewHashIndex(&hashIndexNextTransaction{}, "idx", validLayout)},
		{name: "unset search key", index: newHashIndexNextIndexWithManualScan(t, validLayout, query.Constant{})},
		{name: "nil layout", index: newHashIndexNextIndexWithManualScan(t, nil, query.NewIntConstant(1))},
		{name: "nil schema", index: newHashIndexNextIndexWithManualScan(t, record.NewLayoutFromMetadata(nil, map[string]int{"dataval": 4}, 8), query.NewIntConstant(1))},
		{name: "missing dataval", index: newHashIndexNextIndexWithManualScan(t, record.NewLayout(&record.Schema{}), query.NewIntConstant(1))},
		{name: "unsupported dataval type", index: newHashIndexNextIndexWithManualScan(t, hashIndexNextUnsupportedLayout(), query.NewIntConstant(1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, err := test.index.Next(); err == nil || got || !strings.Contains(err.Error(), "hash index") {
				t.Fatalf("Next() = (%t, %v), want contextual validation error", got, err)
			}
		})
	}
}

func newHashIndexNextIndexWithManualScan(t *testing.T, layout *record.Layout, key query.Constant) *HashIndex {
	t.Helper()
	transaction := &hashIndexNextTransaction{blockSize: 8, integers: map[int]int32{0: 1}, strings: map[int]string{}}
	scanLayout := layout
	if scanLayout == nil {
		schema := &record.Schema{}
		schema.AddIntegerField("dataval")
		scanLayout = record.NewLayout(schema)
	}
	scan, err := record.NewTableScan(transaction, "test", scanLayout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index := NewHashIndex(transaction, "idx", layout)
	index.scan = scan
	index.searchKey = key
	return index
}

func hashIndexNextUnsupportedLayout() *record.Layout {
	schema := &record.Schema{}
	schema.AddField("dataval", record.FieldType(255), 0)
	return record.NewLayout(schema)
}

func TestHashIndexNextWrapsScanAndFieldReadErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("dataval")
	layout := record.NewLayout(schema)
	scanErr := errors.New("block size failed")
	flagErr := errors.New("slot flag failed")
	valueErr := errors.New("integer value failed")
	tests := []struct {
		name        string
		transaction *hashIndexNextTransaction
		cause       error
	}{
		{name: "scan advance", transaction: &hashIndexNextTransaction{blockSizeErr: scanErr, integers: map[int]int32{}, getIntErrAt: -1}, cause: scanErr},
		{name: "slot flag", transaction: &hashIndexNextTransaction{blockSize: 8, integers: map[int]int32{}, getIntErrAt: 0, getIntErr: flagErr}, cause: flagErr},
		{name: "integer field", transaction: &hashIndexNextTransaction{blockSize: 8, integers: map[int]int32{0: 1}, getIntErrAt: 4, getIntErr: valueErr}, cause: valueErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := NewHashIndex(test.transaction, "students", layout)
			index.searchKey = query.NewIntConstant(1)
			scan, err := record.NewTableScan(test.transaction, "bucket", layout)
			if err != nil {
				t.Fatalf("NewTableScan() error = %v", err)
			}
			index.scan = scan
			if got, err := index.Next(); !errors.Is(err, test.cause) || got || !strings.Contains(err.Error(), "students") {
				t.Fatalf("Next() = (%t, %v), want contextual wrapped cause", got, err)
			}
		})
	}

	stringSchema := &record.Schema{}
	stringSchema.AddVarcharField("dataval", 8)
	stringLayout := record.NewLayout(stringSchema)
	stringOffset, _ := stringLayout.Offset("dataval")
	stringErr := errors.New("string value failed")
	transaction := &hashIndexNextTransaction{
		blockSize:      stringLayout.SlotSize(),
		integers:       map[int]int32{0: 1},
		strings:        map[int]string{},
		getIntErrAt:    -1,
		getStringErrAt: stringOffset,
		getStringErr:   stringErr,
	}
	index := NewHashIndex(transaction, "students", stringLayout)
	index.searchKey = query.NewStringConstant("one")
	scan, err := record.NewTableScan(transaction, "bucket", stringLayout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index.scan = scan
	if got, err := index.Next(); !errors.Is(err, stringErr) || got || !strings.Contains(err.Error(), "students") {
		t.Fatalf("Next() string field = (%t, %v), want contextual wrapped cause", got, err)
	}
}
