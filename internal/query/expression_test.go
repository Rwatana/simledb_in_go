package query

import "testing"

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
