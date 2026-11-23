package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func newIndexSelectGetValDataScan(t *testing.T) (*query.RecordScan, *indexSelectNextTransaction) {
	t.Helper()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 12)
	transaction := &indexSelectNextTransaction{}
	scan, err := query.NewRecordScan(transaction, "students", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	return scan, transaction
}

func TestIndexSelectScanGetValDelegatesIntegerAndStringFields(t *testing.T) {
	dataScan, transaction := newIndexSelectGetValDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	transaction.intValue = 27
	transaction.stringValue = "Ada"
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	for _, test := range []struct {
		field string
		want  query.Constant
	}{
		{field: "id", want: query.NewIntConstant(27)},
		{field: "name", want: query.NewStringConstant("Ada")},
	} {
		got, err := scan.GetVal(test.field)
		if err != nil || !got.Equal(test.want) {
			t.Errorf("GetVal(%q) = (%v, %v), want (%v, nil)", test.field, got, err, test.want)
		}
	}
}

func TestIndexSelectScanGetValWrapsDataScanReadError(t *testing.T) {
	dataScan, transaction := newIndexSelectGetValDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	readErr := errors.New("query record read failed")
	transaction.intErr = readErr
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	if value, err := scan.GetVal("id"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetVal() = (%v, %v), want contextual wrapped read error", value, err)
	}
}

func TestIndexSelectScanGetValRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if value, err := nilScan.GetVal("id"); value != (query.Constant{}) || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetVal() on nil receiver = (%v, %v), want contextual error", value, err)
	}
	if value, err := (&IndexSelectScan{}).GetVal("id"); value != (query.Constant{}) || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetVal() without table scan = (%v, %v), want contextual error", value, err)
	}
}
