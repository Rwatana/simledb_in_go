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
