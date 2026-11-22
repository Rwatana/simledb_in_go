package query

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
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

func TestExpressionString(t *testing.T) {
	tests := []struct {
		name       string
		expression Expression
		want       string
	}{
		{name: "integer literal", expression: NewConstantExpression(NewIntConstant(-12)), want: "-12"},
		{name: "string literal has no quotes", expression: NewConstantExpression(NewStringConstant("Ada")), want: "Ada"},
		{name: "empty string literal", expression: NewConstantExpression(NewStringConstant("")), want: ""},
		{name: "field reference", expression: NewFieldExpression("student name"), want: "student name"},
		{name: "empty field name", expression: NewFieldExpression(""), want: ""},
		{name: "unset expression", expression: Expression{}, want: "<unset-expression>"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.expression.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
		})
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

func TestEvaluateFieldExpressionRejectsNilScan(t *testing.T) {
	_, err := NewFieldExpression("name").Evaluate(nil)
	if err == nil || err.Error() != `evaluate field "name": nil scan` {
		t.Fatalf("Evaluate() error = %v, want contextual nil-scan error", err)
	}
}

func TestEvaluateUnsetExpressionReturnsErrorWithoutScan(t *testing.T) {
	scan := &valueScanStub{}
	_, err := (Expression{}).Evaluate(scan)
	if err == nil || err.Error() != "evaluate expression: no literal or field reference" {
		t.Fatalf("Evaluate() error = %v, want contextual unset-expression error", err)
	}
	if scan.calls != 0 {
		t.Fatalf("GetVal calls = %d, want 0 for an unset expression", scan.calls)
	}
}

func TestExpressionAppliesToSchema(t *testing.T) {
	var empty record.Schema
	var nilSchema *record.Schema

	tests := []struct {
		name       string
		expression Expression
		schema     *record.Schema
		want       bool
	}{
		{name: "literal applies without fields", expression: NewConstantExpression(NewIntConstant(3)), schema: &empty, want: true},
		{name: "literal applies to nil schema", expression: NewConstantExpression(NewStringConstant("x")), schema: nilSchema, want: true},
		{name: "present field", expression: NewFieldExpression("name"), schema: schemaWithField("name"), want: true},
		{name: "absent field", expression: NewFieldExpression("age"), schema: schemaWithField("name"), want: false},
		{name: "empty field name can be present", expression: NewFieldExpression(""), schema: schemaWithField(""), want: true},
		{name: "field with nil schema", expression: NewFieldExpression("name"), schema: nilSchema, want: false},
		{name: "unset expression", expression: Expression{}, schema: schemaWithField(""), want: false},
		{name: "unset expression with nil schema", expression: Expression{}, schema: nilSchema, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.expression.AppliesTo(test.schema); got != test.want {
				t.Fatalf("AppliesTo() = %v, want %v", got, test.want)
			}
		})
	}
}

func schemaWithField(name string) *record.Schema {
	var schema record.Schema
	schema.AddIntegerField(name)
	return &schema
}
