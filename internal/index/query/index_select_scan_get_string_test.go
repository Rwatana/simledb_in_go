package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectScanGetStringDelegatesToCurrentDataRecord(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewStringConstant("A-1"))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	transaction.stringValue = "Ada"
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	got, err := scan.GetString("name")
	if err != nil || got != "Ada" {
		t.Fatalf("GetString() = (%q, %v), want (%q, nil)", got, err, "Ada")
	}
}

func TestIndexSelectScanGetStringWrapsDataScanReadError(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewStringConstant("A-1"))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	readErr := errors.New("string record read failed")
	transaction.stringErr = readErr
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	if value, err := scan.GetString("name"); value != "" || !errors.Is(err, readErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetString() = (%q, %v), want contextual wrapped read error", value, err)
	}
}

func TestIndexSelectScanGetStringRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if value, err := nilScan.GetString("name"); value != "" || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetString() on nil receiver = (%q, %v), want contextual error", value, err)
	}
	if value, err := (&IndexSelectScan{}).GetString("name"); value != "" || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetString() without table scan = (%q, %v), want contextual error", value, err)
	}
}
