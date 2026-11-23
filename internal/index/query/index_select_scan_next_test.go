package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type indexSelectNextResult struct {
	found   bool
	rid     record.RID
	nextErr error
	ridErr  error
}

type indexSelectNextIndex struct {
	results []indexSelectNextResult
	current int
	nexts   int
}

func (index *indexSelectNextIndex) BeforeFirst(query.Constant) error {
	index.current = -1
	return nil
}

func (index *indexSelectNextIndex) Next() (bool, error) {
	index.nexts++
	if index.current+1 >= len(index.results) {
		return false, nil
	}
	index.current++
	result := index.results[index.current]
	return result.found, result.nextErr
}

func (index *indexSelectNextIndex) GetDataRID() (record.RID, error) {
	if index.current < 0 || index.current >= len(index.results) {
		return record.RID{}, errors.New("index has no current RID")
	}
	result := index.results[index.current]
	return result.rid, result.ridErr
}

func (*indexSelectNextIndex) Insert(query.Constant, record.RID) error { return nil }
func (*indexSelectNextIndex) Delete(query.Constant, record.RID) error { return nil }
func (*indexSelectNextIndex) Close() error                            { return nil }

type indexSelectNextTransaction struct {
	pinErr      error
	unpinErr    error
	intValue    int32
	intErr      error
	stringValue string
	stringErr   error
	events      *[]string
	pinned      []storage.BlockID
	unpinned    []storage.BlockID
}

func (transaction *indexSelectNextTransaction) Pin(block storage.BlockID) error {
	transaction.pinned = append(transaction.pinned, block)
	return transaction.pinErr
}
func (*indexSelectNextTransaction) BlockSize() (int, error) { return 32, nil }
func (transaction *indexSelectNextTransaction) GetInt(storage.BlockID, int) (int32, error) {
	return transaction.intValue, transaction.intErr
}
func (transaction *indexSelectNextTransaction) GetString(storage.BlockID, int) (string, error) {
	return transaction.stringValue, transaction.stringErr
}
func (*indexSelectNextTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*indexSelectNextTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}
func (*indexSelectNextTransaction) Size(string) (int, error) { return 1, nil }
func (*indexSelectNextTransaction) Append(fileName string) (storage.BlockID, error) {
	return storage.NewBlockID(fileName, 0), nil
}
func (transaction *indexSelectNextTransaction) Unpin(block storage.BlockID) error {
	transaction.unpinned = append(transaction.unpinned, block)
	if transaction.events != nil {
		*transaction.events = append(*transaction.events, "data")
	}
	return transaction.unpinErr
}

func newIndexSelectNextDataScan(t *testing.T) (*query.RecordScan, *indexSelectNextTransaction) {
	t.Helper()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 12)
	transaction := &indexSelectNextTransaction{}
	scan, err := query.NewRecordScan(transaction, "students", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	return scan, transaction
}

func TestIndexSelectScanNextMovesDataScanToMatchingRIDs(t *testing.T) {
	index := &indexSelectNextIndex{results: []indexSelectNextResult{
		{found: true, rid: record.NewRID(0, 2)},
		{found: true, rid: record.NewRID(1, 1)},
		{found: false},
	}}
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, index, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}

	for _, want := range []record.RID{record.NewRID(0, 2), record.NewRID(1, 1)} {
		found, err := scan.Next()
		if err != nil || !found {
			t.Fatalf("Next() = (%t, %v), want matching row", found, err)
		}
		if got, err := dataScan.GetRID(); err != nil || !got.Equal(want) {
			t.Fatalf("data scan RID = (%v, %v), want %v", got, err, want)
		}
	}
	found, err := scan.Next()
	if err != nil || found {
		t.Fatalf("Next() at exhaustion = (%t, %v), want (false, nil)", found, err)
	}
	if len(transaction.pinned) != 3 || len(transaction.unpinned) != 2 || transaction.pinned[0].Number() != 0 || transaction.pinned[1].Number() != 0 || transaction.pinned[2].Number() != 1 || transaction.unpinned[0].Number() != 0 || transaction.unpinned[1].Number() != 0 {
		t.Errorf("data page movement pins/unpins = (%v, %v), want block 0 initial/first match, block 1 second match, and two old-page unpins", transaction.pinned, transaction.unpinned)
	}
	if index.nexts != 3 {
		t.Errorf("index Next() calls = %d, want three", index.nexts)
	}
}

func TestIndexSelectScanNextWrapsIndexAdvanceAndRIDErrors(t *testing.T) {
	advanceErr := errors.New("index advance failed")
	ridErr := errors.New("index RID failed")
	tests := []struct {
		name   string
		result indexSelectNextResult
		cause  error
	}{
		{name: "advance", result: indexSelectNextResult{nextErr: advanceErr}, cause: advanceErr},
		{name: "RID", result: indexSelectNextResult{found: true, ridErr: ridErr}, cause: ridErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := &indexSelectNextIndex{results: []indexSelectNextResult{test.result}}
			dataScan, transaction := newIndexSelectNextDataScan(t)
			scan, err := NewIndexSelectScan(dataScan, index, query.NewIntConstant(42))
			if err != nil {
				t.Fatalf("NewIndexSelectScan() error = %v", err)
			}
			found, err := scan.Next()
			if found || !errors.Is(err, test.cause) || !strings.Contains(err.Error(), "index select scan") {
				t.Fatalf("Next() = (%t, %v), want wrapped %s error", found, err, test.name)
			}
			if len(transaction.pinned) != 1 || len(transaction.unpinned) != 0 {
				t.Errorf("failed Next() moved data scan: pins=%v unpins=%v", transaction.pinned, transaction.unpinned)
			}
		})
	}
}

func TestIndexSelectScanNextWrapsDataScanMovementError(t *testing.T) {
	moveErr := errors.New("data scan unpin failed")
	index := &indexSelectNextIndex{results: []indexSelectNextResult{{found: true, rid: record.NewRID(1, 2)}}}
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, index, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	transaction.unpinErr = moveErr
	found, err := scan.Next()
	if found || !errors.Is(err, moveErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("Next() = (%t, %v), want wrapped data movement error", found, err)
	}
	if scan.dataScan != dataScan || scan.index != index {
		t.Errorf("scan dependencies after movement error = %#v, want both retained", scan)
	}
}

func TestIndexSelectScanNextRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if found, err := nilScan.Next(); found || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("Next() on nil receiver = (%t, %v), want contextual error", found, err)
	}
	for _, scan := range []*IndexSelectScan{
		{},
		{dataScan: &query.RecordScan{}, searchKey: query.NewIntConstant(42)},
		{index: &indexSelectNextIndex{}, searchKey: query.NewIntConstant(42)},
		{dataScan: &query.RecordScan{}, index: &indexSelectNextIndex{}},
	} {
		if found, err := scan.Next(); found || err == nil || !strings.Contains(err.Error(), "index select scan") {
			t.Errorf("Next() on invalid scan %#v = (%t, %v), want contextual error", scan, found, err)
		}
	}
}
