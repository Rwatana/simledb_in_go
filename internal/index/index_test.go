package index

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

var _ Index = (*indexContractProbe)(nil)

type indexContractProbe struct {
	searchKey query.Constant
	dataValue query.Constant
	dataRID   record.RID
	current   record.RID
	advance   bool
	err       error
	closed    bool
}

func (probe *indexContractProbe) BeforeFirst(searchKey query.Constant) error {
	probe.searchKey = searchKey
	return probe.err
}

func (probe *indexContractProbe) Next() (bool, error) {
	return probe.advance, probe.err
}

func (probe *indexContractProbe) GetDataRID() (record.RID, error) {
	return probe.current, probe.err
}

func (probe *indexContractProbe) Insert(dataValue query.Constant, dataRID record.RID) error {
	probe.dataValue = dataValue
	probe.dataRID = dataRID
	return probe.err
}

func (probe *indexContractProbe) Delete(dataValue query.Constant, dataRID record.RID) error {
	probe.dataValue = dataValue
	probe.dataRID = dataRID
	return probe.err
}

func (probe *indexContractProbe) Close() error {
	probe.closed = true
	return probe.err
}

func TestIndexContractExposesTraversalMutationAndClose(t *testing.T) {
	key := query.NewIntConstant(17)
	rid := record.NewRID(4, 2)
	probe := &indexContractProbe{current: rid, advance: true}
	var index Index = probe

	if err := index.BeforeFirst(key); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if !probe.searchKey.Equal(key) {
		t.Errorf("BeforeFirst() key = %v, want %v", probe.searchKey, key)
	}
	if next, err := index.Next(); err != nil || !next {
		t.Errorf("Next() = (%v, %v), want (true, nil)", next, err)
	}
	if got, err := index.GetDataRID(); err != nil || !got.Equal(rid) {
		t.Errorf("GetDataRID() = (%v, %v), want (%v, nil)", got, err, rid)
	}
	if err := index.Insert(key, rid); err != nil || !probe.dataValue.Equal(key) || !probe.dataRID.Equal(rid) {
		t.Errorf("Insert() forwarding = (%v, %v, %v), want key/RID and nil error", probe.dataValue, probe.dataRID, err)
	}
	if err := index.Delete(key, rid); err != nil || !probe.dataValue.Equal(key) || !probe.dataRID.Equal(rid) {
		t.Errorf("Delete() forwarding = (%v, %v, %v), want key/RID and nil error", probe.dataValue, probe.dataRID, err)
	}
	if err := index.Close(); err != nil || !probe.closed {
		t.Errorf("Close() = (%v, closed %v), want nil error and closed", err, probe.closed)
	}
}

func TestIndexContractCanReturnErrorsFromFallibleOperations(t *testing.T) {
	wantErr := errors.New("index operation failed")
	probe := &indexContractProbe{err: wantErr}
	var index Index = probe
	key := query.NewIntConstant(1)
	rid := record.NewRID(0, 0)

	if err := index.BeforeFirst(key); !errors.Is(err, wantErr) {
		t.Errorf("BeforeFirst() error = %v, want original error", err)
	}
	if next, err := index.Next(); next || !errors.Is(err, wantErr) {
		t.Errorf("Next() = (%v, %v), want (false, original error)", next, err)
	}
	if got, err := index.GetDataRID(); got != (record.RID{}) || !errors.Is(err, wantErr) {
		t.Errorf("GetDataRID() = (%v, %v), want zero RID and original error", got, err)
	}
	if err := index.Insert(key, rid); !errors.Is(err, wantErr) {
		t.Errorf("Insert() error = %v, want original error", err)
	}
	if err := index.Delete(key, rid); !errors.Is(err, wantErr) {
		t.Errorf("Delete() error = %v, want original error", err)
	}
	if err := index.Close(); !errors.Is(err, wantErr) {
		t.Errorf("Close() error = %v, want original error", err)
	}
}
