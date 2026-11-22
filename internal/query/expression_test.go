package query

import (
	"errors"
	"testing"
)

type valueScanStub struct {
	name  string
	value Constant
	err   error
	calls int
}

func (scan *valueScanStub) GetVal(name string) (Constant, error) {
	scan.calls++
	scan.name = name
	return scan.value, scan.err
}

func TestLiteralExpressionReturnsItsConstant(t *testing.T) {
	for _, constant := range []Constant{NewIntConstant(0), NewStringConstant("")} {
		expression := NewConstantExpression(constant)
		if got, ok := expression.AsConstant(); !ok || !got.Equal(constant) {
			t.Fatalf("AsConstant() = (%v, %v), want (%v, true)", got, ok, constant)
		}
		if expression.IsFieldName() {
			t.Fatal("IsFieldName() = true for a literal expression")
		}
	}
}

func TestZeroValueExpressionHasNoConstant(t *testing.T) {
	if got, ok := (Expression{}).AsConstant(); ok || got != (Constant{}) {
		t.Fatalf("AsConstant() = (%v, %v), want (zero, false)", got, ok)
	}
}

func TestFieldExpressionReturnsItsName(t *testing.T) {
	expression := NewFieldExpression("student name")
	if got, ok := expression.AsFieldName(); !ok || got != "student name" {
		t.Fatalf("AsFieldName() = (%q, %v), want (%q, true)", got, ok, "student name")
	}
	if _, ok := expression.AsConstant(); ok {
		t.Fatal("AsConstant() reports a literal for a field expression")
	}
	if !expression.IsFieldName() {
		t.Fatal("IsFieldName() = false for a field expression")
	}
}

func TestZeroValueExpressionHasNoFieldName(t *testing.T) {
	if got, ok := (Expression{}).AsFieldName(); ok || got != "" {
		t.Fatalf("AsFieldName() = (%q, %v), want (empty, false)", got, ok)
	}
}

func TestEvaluateLiteralExpressionDoesNotReadScan(t *testing.T) {
	want := NewIntConstant(7)
	scan := &valueScanStub{}
	got, err := NewConstantExpression(want).Evaluate(scan)
	if err != nil || !got.Equal(want) {
		t.Fatalf("Evaluate() = (%v, %v), want (%v, nil)", got, err, want)
	}
	if scan.calls != 0 {
		t.Fatalf("GetVal calls = %d, want 0", scan.calls)
	}
}

func TestEvaluateFieldExpressionReadsFieldOnce(t *testing.T) {
	want := NewStringConstant("Ada")
	scan := &valueScanStub{value: want}
	got, err := NewFieldExpression("name").Evaluate(scan)
	if err != nil || !got.Equal(want) {
		t.Fatalf("Evaluate() = (%v, %v), want (%v, nil)", got, err, want)
	}
	if scan.calls != 1 || scan.name != "name" {
		t.Fatalf("GetVal calls = %d for %q, want one call for name", scan.calls, scan.name)
	}
}

func TestEvaluateFieldExpressionReturnsScanError(t *testing.T) {
	want := errors.New("missing field")
	scan := &valueScanStub{err: want}
	_, err := NewFieldExpression("unknown").Evaluate(scan)
	if err != want {
		t.Fatalf("Evaluate() error = %v, want original error %v", err, want)
	}
}
