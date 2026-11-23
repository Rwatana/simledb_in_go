package query_test

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

type scanContractStub struct {
	calls []string
}

func (scan *scanContractStub) BeforeFirst() error {
	scan.calls = append(scan.calls, "before-first")
	return nil
}

func (scan *scanContractStub) Next() (bool, error) {
	scan.calls = append(scan.calls, "next")
	return true, nil
}

func (scan *scanContractStub) GetInt(string) (int32, error) {
	scan.calls = append(scan.calls, "get-int")
	return 7, nil
}

func (scan *scanContractStub) GetString(string) (string, error) {
	scan.calls = append(scan.calls, "get-string")
	return "value", nil
}

func (scan *scanContractStub) GetVal(string) (query.Constant, error) {
	scan.calls = append(scan.calls, "get-value")
	return query.NewIntConstant(7), nil
}

func (scan *scanContractStub) HasField(string) bool {
	scan.calls = append(scan.calls, "has-field")
	return true
}

func (scan *scanContractStub) Close() error {
	scan.calls = append(scan.calls, "close")
	return nil
}

func TestScanContractProvidesTraversalAndValueOperations(t *testing.T) {
	var scan query.Scan = &scanContractStub{}

	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", next, err)
	}
	if value, err := scan.GetInt("id"); err != nil || value != 7 {
		t.Fatalf("GetInt(id) = (%d, %v), want (7, nil)", value, err)
	}
	if value, err := scan.GetString("name"); err != nil || value != "value" {
		t.Fatalf("GetString(name) = (%q, %v), want (value, nil)", value, err)
	}
	if value, err := scan.GetVal("id"); err != nil || value != query.NewIntConstant(7) {
		t.Fatalf("GetVal(id) = (%v, %v), want integer constant 7", value, err)
	}
	if !scan.HasField("id") {
		t.Error("HasField(id) = false, want true")
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	got := scan.(*scanContractStub).calls
	want := []string{"before-first", "next", "get-int", "get-string", "get-value", "has-field", "close"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scan calls = %v, want %v", got, want)
	}
}

type updateScanContractStub struct {
	scanContractStub
	rid record.RID
}

func (scan *updateScanContractStub) SetInt(string, int32) error {
	scan.calls = append(scan.calls, "set-int")
	return nil
}

func (scan *updateScanContractStub) SetString(string, string) error {
	scan.calls = append(scan.calls, "set-string")
	return nil
}

func (scan *updateScanContractStub) SetVal(string, query.Constant) error {
	scan.calls = append(scan.calls, "set-value")
	return nil
}

func (scan *updateScanContractStub) Insert() error {
	scan.calls = append(scan.calls, "insert")
	return nil
}

func (scan *updateScanContractStub) Delete() error {
	scan.calls = append(scan.calls, "delete")
	return nil
}

func (scan *updateScanContractStub) GetRID() (record.RID, error) {
	scan.calls = append(scan.calls, "get-rid")
	return scan.rid, nil
}

func (scan *updateScanContractStub) MoveToRID(rid record.RID) error {
	scan.calls = append(scan.calls, "move-to-rid")
	scan.rid = rid
	return nil
}

func TestUpdateScanContractProvidesMutationAndRIDOperations(t *testing.T) {
	initialRID := record.NewRID(2, 3)
	updatedRID := record.NewRID(5, 7)
	var scan query.UpdateScan = &updateScanContractStub{rid: initialRID}

	if err := scan.SetInt("id", 9); err != nil {
		t.Fatalf("SetInt(id, 9) error = %v", err)
	}
	if err := scan.SetString("name", "value"); err != nil {
		t.Fatalf("SetString(name, value) error = %v", err)
	}
	if err := scan.SetVal("score", query.NewIntConstant(11)); err != nil {
		t.Fatalf("SetVal(score, 11) error = %v", err)
	}
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := scan.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got, err := scan.GetRID(); err != nil || !got.Equal(initialRID) {
		t.Fatalf("GetRID() = (%v, %v), want (%v, nil)", got, err, initialRID)
	}
	if err := scan.MoveToRID(updatedRID); err != nil {
		t.Fatalf("MoveToRID(%v) error = %v", updatedRID, err)
	}
	if got, err := scan.GetRID(); err != nil || !got.Equal(updatedRID) {
		t.Fatalf("GetRID() after MoveToRID = (%v, %v), want (%v, nil)", got, err, updatedRID)
	}

	got := scan.(*updateScanContractStub).calls
	want := []string{"set-int", "set-string", "set-value", "insert", "delete", "get-rid", "move-to-rid", "get-rid"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("update scan calls = %v, want %v", got, want)
	}
}
