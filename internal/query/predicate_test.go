package query

import (
	"errors"
	"reflect"
	"testing"
)

func TestEmptyPredicateIsTrueWithoutScanAccess(t *testing.T) {
	var predicate Predicate
	if got, err := predicate.IsSatisfied(nil); err != nil || !got {
		t.Fatalf("IsSatisfied(nil) = (%v, %v), want (true, nil)", got, err)
	}

	scan := &valueScanStub{}
	if got, err := predicate.IsSatisfied(scan); err != nil || !got {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (true, nil)", got, err)
	}
	if scan.calls != 0 {
		t.Fatalf("GetVal calls = %d, want 0", scan.calls)
	}
}

func TestSingleTermPredicateEvaluatesTerm(t *testing.T) {
	scan := &termScanStub{values: map[string]Constant{
		"lhs": NewIntConstant(4),
		"rhs": NewIntConstant(4),
	}}
	predicate := NewPredicateWithTerm(NewTerm(NewFieldExpression("lhs"), NewFieldExpression("rhs")))
	if got, err := predicate.IsSatisfied(scan); err != nil || !got {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (true, nil)", got, err)
	}
	if want := []string{"lhs", "rhs"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}

	falsePredicate := NewPredicateWithTerm(NewTerm(
		NewConstantExpression(NewIntConstant(4)),
		NewConstantExpression(NewIntConstant(5)),
	))
	if got, err := falsePredicate.IsSatisfied(nil); err != nil || got {
		t.Fatalf("IsSatisfied(nil) = (%v, %v), want (false, nil)", got, err)
	}
}

func TestSingleTermPredicateReturnsTermError(t *testing.T) {
	wantErr := errors.New("term field unavailable")
	scan := &termScanStub{errors: map[string]error{"lhs": wantErr}}
	predicate := NewPredicateWithTerm(NewTerm(NewFieldExpression("lhs"), NewFieldExpression("rhs")))
	got, err := predicate.IsSatisfied(scan)
	if got || err != wantErr {
		t.Fatalf("IsSatisfied(scan) = (%v, %v), want (false, original error)", got, err)
	}
	if want := []string{"lhs"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}
}
