package query

import (
	"errors"
	"reflect"
	"testing"
)

type termScanStub struct {
	values map[string]Constant
	errors map[string]error
	calls  []string
}

func (scan *termScanStub) GetVal(name string) (Constant, error) {
	scan.calls = append(scan.calls, name)
	if err := scan.errors[name]; err != nil {
		return Constant{}, err
	}
	return scan.values[name], nil
}

func TestTermSatisfiedForLiteralExpressions(t *testing.T) {
	tests := []struct {
		name string
		lhs  Constant
		rhs  Constant
		want bool
	}{
		{name: "equal integers", lhs: NewIntConstant(4), rhs: NewIntConstant(4), want: true},
		{name: "equal strings", lhs: NewStringConstant("Ada"), rhs: NewStringConstant("Ada"), want: true},
		{name: "unequal integers", lhs: NewIntConstant(4), rhs: NewIntConstant(5), want: false},
		{name: "different kinds", lhs: NewIntConstant(0), rhs: NewStringConstant("0"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewTerm(NewConstantExpression(test.lhs), NewConstantExpression(test.rhs)).IsSatisfied(nil)
			if err != nil || got != test.want {
				t.Fatalf("IsSatisfied(nil) = (%v, %v), want (%v, nil)", got, err, test.want)
			}
		})
	}
}

func TestTermSatisfiedEvaluatesFieldsLeftToRight(t *testing.T) {
	scan := &termScanStub{values: map[string]Constant{
		"lhs": NewStringConstant("same"),
		"rhs": NewStringConstant("same"),
	}}
	got, err := NewTerm(NewFieldExpression("lhs"), NewFieldExpression("rhs")).IsSatisfied(scan)
	if err != nil || !got {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (true, nil)", got, err)
	}
	if want := []string{"lhs", "rhs"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}
}

func TestTermSatisfiedReturnsFirstEvaluationError(t *testing.T) {
	wantErr := errors.New("left field unavailable")
	scan := &termScanStub{
		errors: map[string]error{"lhs": wantErr},
	}
	got, err := NewTerm(NewFieldExpression("lhs"), NewFieldExpression("rhs")).IsSatisfied(scan)
	if got || err != wantErr {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (false, original error)", got, err)
	}
	if want := []string{"lhs"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}
}

func TestTermSatisfiedReturnsRightEvaluationError(t *testing.T) {
	wantErr := errors.New("right field unavailable")
	scan := &termScanStub{
		values: map[string]Constant{"lhs": NewIntConstant(1)},
		errors: map[string]error{"rhs": wantErr},
	}
	got, err := NewTerm(NewFieldExpression("lhs"), NewFieldExpression("rhs")).IsSatisfied(scan)
	if got || err != wantErr {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (false, original error)", got, err)
	}
	if want := []string{"lhs", "rhs"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}
}

func TestTermEquatesWithConstant(t *testing.T) {
	integer := NewIntConstant(7)
	zeroValueConstant := Constant{}
	tests := []struct {
		name      string
		term      Term
		fieldName string
		want      Constant
		wantOK    bool
	}{
		{
			name:      "field on left",
			term:      NewTerm(NewFieldExpression("age"), NewConstantExpression(integer)),
			fieldName: "age",
			want:      integer,
			wantOK:    true,
		},
		{
			name:      "field on right",
			term:      NewTerm(NewConstantExpression(integer), NewFieldExpression("age")),
			fieldName: "age",
			want:      integer,
			wantOK:    true,
		},
		{
			name:      "explicit empty field name",
			term:      NewTerm(NewFieldExpression(""), NewConstantExpression(integer)),
			fieldName: "",
			want:      integer,
			wantOK:    true,
		},
		{
			name:      "literal zero-value constant is present",
			term:      NewTerm(NewFieldExpression("age"), NewConstantExpression(zeroValueConstant)),
			fieldName: "age",
			want:      zeroValueConstant,
			wantOK:    true,
		},
		{
			name:      "different field",
			term:      NewTerm(NewFieldExpression("name"), NewConstantExpression(integer)),
			fieldName: "age",
		},
		{
			name:      "two fields",
			term:      NewTerm(NewFieldExpression("age"), NewFieldExpression("name")),
			fieldName: "age",
		},
		{
			name:      "two literals",
			term:      NewTerm(NewConstantExpression(integer), NewConstantExpression(integer)),
			fieldName: "age",
		},
		{
			name:      "unset left expression",
			term:      NewTerm(Expression{}, NewConstantExpression(integer)),
			fieldName: "age",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.term.EquatesWithConstant(test.fieldName)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("EquatesWithConstant(%q) = (%v, %v), want (%v, %v)", test.fieldName, got, ok, test.want, test.wantOK)
			}
		})
	}
}

func TestTermEquatesWithField(t *testing.T) {
	tests := []struct {
		name      string
		term      Term
		fieldName string
		want      string
		wantOK    bool
	}{
		{
			name:      "requested field on left",
			term:      NewTerm(NewFieldExpression("age"), NewFieldExpression("birth_year")),
			fieldName: "age",
			want:      "birth_year",
			wantOK:    true,
		},
		{
			name:      "requested field on right",
			term:      NewTerm(NewFieldExpression("birth_year"), NewFieldExpression("age")),
			fieldName: "age",
			want:      "birth_year",
			wantOK:    true,
		},
		{
			name:      "explicit empty field name",
			term:      NewTerm(NewFieldExpression(""), NewFieldExpression("other")),
			fieldName: "",
			want:      "other",
			wantOK:    true,
		},
		{
			name:      "different field",
			term:      NewTerm(NewFieldExpression("name"), NewFieldExpression("age")),
			fieldName: "height",
		},
		{
			name:      "field and literal",
			term:      NewTerm(NewFieldExpression("age"), NewConstantExpression(NewIntConstant(1))),
			fieldName: "age",
		},
		{
			name:      "two literals",
			term:      NewTerm(NewConstantExpression(NewIntConstant(1)), NewConstantExpression(NewIntConstant(1))),
			fieldName: "age",
		},
		{
			name:      "unset expression",
			term:      NewTerm(Expression{}, NewFieldExpression("age")),
			fieldName: "age",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.term.EquatesWithField(test.fieldName)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("EquatesWithField(%q) = (%q, %v), want (%q, %v)", test.fieldName, got, ok, test.want, test.wantOK)
			}
		})
	}
}
