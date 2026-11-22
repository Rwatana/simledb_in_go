package query_test

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
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
