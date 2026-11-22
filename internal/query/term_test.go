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
