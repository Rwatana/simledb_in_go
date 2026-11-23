package query_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestProductScanEnumeratesLeftMajorPairsAndRestarts(t *testing.T) {
	left := &selectScanStub{
		rows: []map[string]query.Constant{
			{"id": query.NewIntConstant(1), "shared": query.NewStringConstant("left-1")},
			{"id": query.NewIntConstant(2), "shared": query.NewStringConstant("left-2")},
		},
		fields: map[string]bool{"id": true, "shared": true},
	}
	right := &selectScanStub{
		rows: []map[string]query.Constant{
			{"name": query.NewStringConstant("Ada"), "shared": query.NewStringConstant("right-A")},
			{"name": query.NewStringConstant("Grace"), "shared": query.NewStringConstant("right-G")},
		},
		fields: map[string]bool{"name": true, "shared": true},
	}
	scan, err := query.NewProductScan(left, right)
	if err != nil {
		t.Fatalf("NewProductScan() error = %v", err)
	}

	want := []struct {
		id     int32
		name   string
		shared string
	}{
		{1, "Ada", "left-1"},
		{1, "Grace", "left-1"},
		{2, "Ada", "left-2"},
		{2, "Grace", "left-2"},
	}
	for pass := 0; pass < 2; pass++ {
		if err := scan.BeforeFirst(); err != nil {
			t.Fatalf("BeforeFirst() pass %d error = %v", pass, err)
		}
		for row, expected := range want {
			next, err := scan.Next()
			if err != nil || !next {
				t.Fatalf("Next() row %d = (%t, %v), want (true, nil)", row, next, err)
			}
			id, err := scan.GetInt("id")
			if err != nil || id != expected.id {
				t.Errorf("GetInt(id) = (%d, %v), want (%d, nil)", id, err, expected.id)
			}
			name, err := scan.GetString("name")
			if err != nil || name != expected.name {
				t.Errorf("GetString(name) = (%q, %v), want (%q, nil)", name, err, expected.name)
			}
			shared, err := scan.GetVal("shared")
			if text, ok := shared.AsString(); err != nil || !ok || text != expected.shared {
				t.Errorf("GetVal(shared) = (%v, %v), want %q from the left scan", shared, err, expected.shared)
			}
		}
		if next, err := scan.Next(); err != nil || next {
			t.Fatalf("Next() after product = (%t, %v), want (false, nil)", next, err)
		}
	}
	if !scan.HasField("id") || !scan.HasField("name") || !scan.HasField("shared") || scan.HasField("missing") {
		t.Fatalf("HasField results do not match the union of input fields")
	}
}

func TestProductScanWithAnEmptyInputReturnsNoRows(t *testing.T) {
	tests := []struct {
		name  string
		left  []map[string]query.Constant
		right []map[string]query.Constant
	}{
		{name: "empty left", right: []map[string]query.Constant{{"id": query.NewIntConstant(1)}}},
		{name: "empty right", left: []map[string]query.Constant{{"id": query.NewIntConstant(1)}, {"id": query.NewIntConstant(2)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left := &selectScanStub{rows: test.left}
			right := &selectScanStub{rows: test.right}
			scan, err := query.NewProductScan(left, right)
			if err != nil {
				t.Fatalf("NewProductScan() error = %v", err)
			}
			if err := scan.BeforeFirst(); err != nil {
				t.Fatalf("BeforeFirst() error = %v", err)
			}
			if next, err := scan.Next(); err != nil || next {
				t.Fatalf("Next() = (%t, %v), want (false, nil)", next, err)
			}
			if len(left.calls) == 0 || left.calls[0] != "before-first" || len(right.calls) == 0 || right.calls[0] != "before-first" {
				t.Fatalf("BeforeFirst() calls = (%v, %v), want both inputs reset", left.calls, right.calls)
			}
		})
	}
}

func TestNewProductScanRejectsNilInputs(t *testing.T) {
	valid := &selectScanStub{}
	if scan, err := query.NewProductScan(nil, valid); err == nil || scan != nil || !strings.Contains(err.Error(), "left scan is nil") {
		t.Fatalf("NewProductScan(nil, source) = (%v, %v), want left-input error", scan, err)
	}
	if scan, err := query.NewProductScan(valid, nil); err == nil || scan != nil || !strings.Contains(err.Error(), "right scan is nil") {
		t.Fatalf("NewProductScan(source, nil) = (%v, %v), want right-input error", scan, err)
	}
}

func TestProductScanWrapsInputResetErrors(t *testing.T) {
	leftErr := errors.New("left reset failed")
	rightErr := errors.New("right reset failed")
	left := &selectScanStub{beforeFirstErr: leftErr}
	right := &selectScanStub{beforeFirstErr: rightErr}
	scan, err := query.NewProductScan(left, right)
	if err != nil {
		t.Fatalf("NewProductScan() error = %v", err)
	}
	err = scan.BeforeFirst()
	if !errors.Is(err, leftErr) || !errors.Is(err, rightErr) || !strings.Contains(err.Error(), "product scan") {
		t.Fatalf("BeforeFirst() error = %v, want both contextual reset errors", err)
	}
	if len(left.calls) != 1 || len(right.calls) != 1 {
		t.Fatalf("BeforeFirst() calls = (%v, %v), want both inputs reset once", left.calls, right.calls)
	}
}

func TestProductScanWrapsIterationAndRightRestartErrors(t *testing.T) {
	leftErr := errors.New("left next failed")
	left := &selectScanStub{nextErrors: map[int]error{0: leftErr}}
	right := &selectScanStub{}
	scan, err := query.NewProductScan(left, right)
	if err != nil {
		t.Fatalf("NewProductScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if next, err := scan.Next(); next || !errors.Is(err, leftErr) || !strings.Contains(err.Error(), "advance left input") {
		t.Fatalf("Next() with left error = (%t, %v), want contextual left error", next, err)
	}

	rightErr := errors.New("right next failed")
	left = &selectScanStub{rows: []map[string]query.Constant{{"id": query.NewIntConstant(1)}}}
	right = &selectScanStub{nextErrors: map[int]error{0: rightErr}}
	scan, _ = query.NewProductScan(left, right)
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() before right error = %v", err)
	}
	if next, err := scan.Next(); next || !errors.Is(err, rightErr) || !strings.Contains(err.Error(), "advance right input") {
		t.Fatalf("Next() with right error = (%t, %v), want contextual right error", next, err)
	}

	restartErr := errors.New("right reset failed")
	left = &selectScanStub{rows: []map[string]query.Constant{{"id": query.NewIntConstant(1)}, {"id": query.NewIntConstant(2)}}}
	right = &selectScanStub{rows: []map[string]query.Constant{{"name": query.NewStringConstant("Ada")}}}
	scan, _ = query.NewProductScan(left, right)
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() before restart error = %v", err)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("first Next() = (%t, %v), want (true, nil)", next, err)
	}
	right.beforeFirstErr = restartErr
	if next, err := scan.Next(); next || !errors.Is(err, restartErr) || !strings.Contains(err.Error(), "restart right input") {
		t.Fatalf("Next() with right restart error = (%t, %v), want contextual reset error", next, err)
	}
}

func TestProductScanDelegatesFieldReadsAndWrapsErrors(t *testing.T) {
	readErr := errors.New("field read failed")
	left := &selectScanStub{
		rows:         []map[string]query.Constant{{"id": query.NewIntConstant(1), "shared": query.NewStringConstant("left")}},
		fields:       map[string]bool{"id": true, "shared": true},
		getIntErrors: map[string]error{"id": readErr},
	}
	right := &selectScanStub{
		rows:            []map[string]query.Constant{{"name": query.NewStringConstant("Ada"), "shared": query.NewStringConstant("right")}},
		fields:          map[string]bool{"name": true, "shared": true},
		getStringErrors: map[string]error{"name": readErr},
		getValErrors:    map[string]error{"name": readErr},
	}
	scan, err := query.NewProductScan(left, right)
	if err != nil {
		t.Fatalf("NewProductScan() error = %v", err)
	}
	if _, err := scan.GetInt("id"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read integer field") {
		t.Errorf("GetInt(id) error = %v, want contextual left-input error", err)
	}
	if _, err := scan.GetString("name"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read string field") {
		t.Errorf("GetString(name) error = %v, want contextual right-input error", err)
	}
	if _, err := scan.GetVal("name"); !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read value field") {
		t.Errorf("GetVal(name) error = %v, want contextual right-input error", err)
	}
	if _, err := scan.GetVal("missing"); err == nil || !strings.Contains(err.Error(), "field") {
		t.Errorf("GetVal(missing) error = %v, want missing-field error", err)
	}
	if scan.HasField("missing") {
		t.Error("HasField(missing) = true, want false")
	}
}

func TestProductScanClosesBothInputsAndPreservesErrors(t *testing.T) {
	leftErr := errors.New("left close failed")
	rightErr := errors.New("right close failed")
	left := &selectScanStub{closeErr: leftErr}
	right := &selectScanStub{closeErr: rightErr}
	scan, err := query.NewProductScan(left, right)
	if err != nil {
		t.Fatalf("NewProductScan() error = %v", err)
	}
	err = scan.Close()
	if !errors.Is(err, leftErr) || !errors.Is(err, rightErr) || !strings.Contains(err.Error(), "close left input") || !strings.Contains(err.Error(), "close right input") {
		t.Fatalf("Close() error = %v, want both contextual errors", err)
	}
	for name, source := range map[string]*selectScanStub{"left": left, "right": right} {
		if len(source.calls) != 1 || source.calls[0] != "close" {
			t.Errorf("%s Close() calls = %v, want one close", name, source.calls)
		}
	}

	left = &selectScanStub{}
	right = &selectScanStub{}
	scan, _ = query.NewProductScan(left, right)
	if err := scan.Close(); err != nil {
		t.Errorf("Close() with successful inputs error = %v, want nil", err)
	}
}

func TestProductScanZeroValueReportsUninitializedInputs(t *testing.T) {
	var scan query.ProductScan
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("BeforeFirst() error = %v, want uninitialized error", err)
	}
	if next, err := scan.Next(); next || err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Next() = (%t, %v), want false and uninitialized error", next, err)
	}
	if _, err := scan.GetInt("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetInt() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetString("name"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetString() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetVal("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetVal() error = %v, want uninitialized error", err)
	}
	if scan.HasField("id") {
		t.Error("HasField(id) = true, want false for the zero value")
	}
	if err := scan.Close(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Close() error = %v, want uninitialized error", err)
	}
}
