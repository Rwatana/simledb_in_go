package indexquery

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

type indexSelectCloseIndex struct {
	events   *[]string
	closeErr error
}

func (*indexSelectCloseIndex) BeforeFirst(query.Constant) error { return nil }
func (*indexSelectCloseIndex) Next() (bool, error)              { return false, nil }
func (*indexSelectCloseIndex) GetDataRID() (record.RID, error)  { return record.RID{}, nil }
func (*indexSelectCloseIndex) Insert(query.Constant, record.RID) error {
	return nil
}
func (*indexSelectCloseIndex) Delete(query.Constant, record.RID) error { return nil }
func (index *indexSelectCloseIndex) Close() error {
	if index.events != nil {
		*index.events = append(*index.events, "index")
	}
	return index.closeErr
}

func TestIndexSelectScanCloseClosesIndexBeforeDataScan(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	var events []string
	transaction.events = &events
	index := &indexSelectCloseIndex{events: &events}
	scan, err := NewIndexSelectScan(dataScan, index, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}

	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !reflect.DeepEqual(events, []string{"index", "data"}) {
		t.Fatalf("close order = %v, want index then data scan", events)
	}
}

func TestIndexSelectScanCloseAttemptsBothResourcesAndPreservesErrors(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	var events []string
	transaction.events = &events
	indexErr := errors.New("index close failed")
	dataErr := errors.New("table scan close failed")
	transaction.unpinErr = dataErr
	index := &indexSelectCloseIndex{events: &events, closeErr: indexErr}
	scan, err := NewIndexSelectScan(dataScan, index, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}

	err = scan.Close()
	if !errors.Is(err, indexErr) || !errors.Is(err, dataErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("Close() error = %v, want both wrapped close causes", err)
	}
	if !reflect.DeepEqual(events, []string{"index", "data"}) {
		t.Fatalf("close order after error = %v, want both resources attempted in order", events)
	}
}

func TestIndexSelectScanCloseHandlesNilAndPartialResources(t *testing.T) {
	var nilScan *IndexSelectScan
	if err := nilScan.Close(); err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("Close() on nil receiver error = %v, want contextual error", err)
	}
	if err := (&IndexSelectScan{}).Close(); err != nil {
		t.Fatalf("Close() without resources error = %v, want nil", err)
	}

	var indexEvents []string
	if err := (&IndexSelectScan{index: &indexSelectCloseIndex{events: &indexEvents}}).Close(); err != nil {
		t.Fatalf("Close() with only index error = %v", err)
	}
	if !reflect.DeepEqual(indexEvents, []string{"index"}) {
		t.Fatalf("index-only close events = %v, want one index close", indexEvents)
	}

	dataScan, transaction := newIndexSelectNextDataScan(t)
	var dataEvents []string
	transaction.events = &dataEvents
	if err := (&IndexSelectScan{dataScan: dataScan}).Close(); err != nil {
		t.Fatalf("Close() with only data scan error = %v", err)
	}
	if !reflect.DeepEqual(dataEvents, []string{"data"}) {
		t.Fatalf("data-only close events = %v, want one data close", dataEvents)
	}
	if err := (&IndexSelectScan{dataScan: &query.RecordScan{}}).Close(); err != nil {
		t.Fatalf("Close() with absent embedded table scan error = %v, want nil", err)
	}
}
