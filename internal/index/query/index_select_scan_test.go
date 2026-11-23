package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/index"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type positioningIndex struct {
	keys     []query.Constant
	position error
}

func (index *positioningIndex) BeforeFirst(key query.Constant) error {
	index.keys = append(index.keys, key)
	return index.position
}

func (*positioningIndex) Next() (bool, error)                     { return false, nil }
func (*positioningIndex) GetDataRID() (record.RID, error)         { return record.RID{}, nil }
func (*positioningIndex) Insert(query.Constant, record.RID) error { return nil }
func (*positioningIndex) Delete(query.Constant, record.RID) error { return nil }
func (*positioningIndex) Close() error                            { return nil }

type positioningTransaction struct{}

func (*positioningTransaction) Pin(storage.BlockID) error                  { return nil }
func (*positioningTransaction) BlockSize() (int, error)                    { return 32, nil }
func (*positioningTransaction) GetInt(storage.BlockID, int) (int32, error) { return 0, nil }
func (*positioningTransaction) GetString(storage.BlockID, int) (string, error) {
	return "", nil
}
func (*positioningTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*positioningTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}
func (*positioningTransaction) Size(string) (int, error) { return 1, nil }
func (*positioningTransaction) Append(fileName string) (storage.BlockID, error) {
	return storage.NewBlockID(fileName, 0), nil
}
func (*positioningTransaction) Unpin(storage.BlockID) error { return nil }

func newPositioningTableScan(t *testing.T) *record.TableScan {
	t.Helper()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	scan, err := record.NewTableScan(&positioningTransaction{}, "students", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	return scan
}

func TestNewIndexSelectScanPositionsIndexInitially(t *testing.T) {
	dataScan := newPositioningTableScan(t)
	index := &positioningIndex{}
	key := query.NewIntConstant(42)

	scan, err := NewIndexSelectScan(dataScan, index, key)
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	if scan == nil || scan.dataScan != dataScan || scan.index != index || !scan.searchKey.Equal(key) {
		t.Fatalf("constructed scan = %#v, want retained data scan, index, and key", scan)
	}
	if len(index.keys) != 1 || !index.keys[0].Equal(key) {
		t.Fatalf("index positions = %v, want initial key %v exactly once", index.keys, key)
	}
}

func TestIndexSelectScanBeforeFirstRepositionsStoredKey(t *testing.T) {
	index := &positioningIndex{}
	key := query.NewStringConstant("A-1")
	scan, err := NewIndexSelectScan(newPositioningTableScan(t), index, key)
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if len(index.keys) != 2 || !index.keys[0].Equal(key) || !index.keys[1].Equal(key) {
		t.Fatalf("index positions = %v, want constructor and BeforeFirst calls with %v", index.keys, key)
	}
}

func TestNewIndexSelectScanRejectsInvalidInputsBeforePositioning(t *testing.T) {
	validScan := newPositioningTableScan(t)
	validIndex := &positioningIndex{}
	key := query.NewIntConstant(42)
	tests := []struct {
		name string
		scan *record.TableScan
		idx  index.Index
		key  query.Constant
	}{
		{name: "nil table scan", idx: validIndex, key: key},
		{name: "nil index", scan: validScan, key: key},
		{name: "unset key", scan: validScan, idx: validIndex},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := NewIndexSelectScan(test.scan, test.idx, test.key)
			if scan != nil || err == nil || !strings.Contains(err.Error(), "index select scan") {
				t.Fatalf("NewIndexSelectScan() = (%v, %v), want nil scan and contextual error", scan, err)
			}
		})
	}
	if len(validIndex.keys) != 0 {
		t.Errorf("invalid inputs positioned index with keys %v", validIndex.keys)
	}
}

func TestNewIndexSelectScanWrapsInitialPositionError(t *testing.T) {
	positionErr := errors.New("position failed")
	index := &positioningIndex{position: positionErr}

	scan, err := NewIndexSelectScan(newPositioningTableScan(t), index, query.NewIntConstant(42))
	if scan != nil || !errors.Is(err, positionErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("NewIndexSelectScan() = (%v, %v), want nil scan and wrapped position error", scan, err)
	}
}

func TestIndexSelectScanBeforeFirstWrapsPositionError(t *testing.T) {
	index := &positioningIndex{}
	scan, err := NewIndexSelectScan(newPositioningTableScan(t), index, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	positionErr := errors.New("reposition failed")
	index.position = positionErr
	if err := scan.BeforeFirst(); !errors.Is(err, positionErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("BeforeFirst() error = %v, want wrapped position error", err)
	}
	if scan.index != index || !scan.searchKey.Equal(query.NewIntConstant(42)) {
		t.Errorf("scan state after positioning failure = %#v, want retained index and key", scan)
	}
}

func TestIndexSelectScanBeforeFirstRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if err := nilScan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("BeforeFirst() on nil receiver error = %v, want contextual error", err)
	}
	tests := []struct {
		name string
		scan *IndexSelectScan
	}{
		{name: "nil index", scan: &IndexSelectScan{searchKey: query.NewIntConstant(42)}},
		{name: "unset key", scan: &IndexSelectScan{index: &positioningIndex{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "index select scan") {
				t.Fatalf("BeforeFirst() error = %v, want contextual error", err)
			}
		})
	}
}
