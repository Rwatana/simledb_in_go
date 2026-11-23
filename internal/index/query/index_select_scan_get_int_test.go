package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexSelectScanGetIntDelegatesToCurrentDataRecord(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	transaction.intValue = 27
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	got, err := scan.GetInt("id")
	if err != nil || got != 27 {
		t.Fatalf("GetInt() = (%d, %v), want (27, nil)", got, err)
	}
}

func TestIndexSelectScanGetIntWrapsDataScanReadError(t *testing.T) {
	dataScan, transaction := newIndexSelectNextDataScan(t)
	scan, err := NewIndexSelectScan(dataScan, &indexSelectNextIndex{}, query.NewIntConstant(42))
	if err != nil {
		t.Fatalf("NewIndexSelectScan() error = %v", err)
	}
	readErr := errors.New("record read failed")
	transaction.intErr = readErr
	if err := dataScan.MoveToRID(record.NewRID(0, 3)); err != nil {
		t.Fatalf("MoveToRID() error = %v", err)
	}

	if value, err := scan.GetInt("id"); value != 0 || !errors.Is(err, readErr) || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetInt() = (%d, %v), want contextual wrapped read error", value, err)
	}
}

func TestIndexSelectScanGetIntRejectsInvalidState(t *testing.T) {
	var nilScan *IndexSelectScan
	if value, err := nilScan.GetInt("id"); value != 0 || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetInt() on nil receiver = (%d, %v), want contextual error", value, err)
	}
	if value, err := (&IndexSelectScan{}).GetInt("id"); value != 0 || err == nil || !strings.Contains(err.Error(), "index select scan") {
		t.Fatalf("GetInt() without table scan = (%d, %v), want contextual error", value, err)
	}
}
