package indexquery

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestIndexSelectScanHasFieldDelegatesToCurrentDataScan(t *testing.T) {
	dataScan, _ := newIndexSelectGetValDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	if !scan.HasField("id") || !scan.HasField("name") {
		t.Fatal("HasField() returned false for a field in the table schema")
	}
	if scan.HasField("missing") {
		t.Fatal("HasField() returned true for a field absent from the table schema")
	}
}

func TestIndexSelectScanHasFieldRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if nilScan.HasField("id") {
		t.Fatal("HasField() on a nil adapter returned true")
	}
	for _, scan := range []*IndexSelectScan{
		{},
		{dataScan: &query.RecordScan{}},
	} {
		if scan.HasField("id") {
			t.Errorf("HasField() on invalid adapter %#v returned true", scan)
		}
	}
}
