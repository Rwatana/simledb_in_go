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

func TestPredicateConjoinWithConcatenatesTermsAndPreservesInputs(t *testing.T) {
	termA := NewTerm(NewFieldExpression("a"), NewConstantExpression(NewIntConstant(1)))
	termB := NewTerm(NewFieldExpression("b"), NewConstantExpression(NewIntConstant(2)))
	left := NewPredicateWithTerm(termA)
	right := NewPredicateWithTerm(termB)

	joined := left.ConjoinWith(right).ConjoinWith(NewPredicateWithTerm(termA))
	if want := []Term{termA, termB, termA}; !reflect.DeepEqual(joined.terms, want) {
		t.Fatalf("joined terms = %v, want %v", joined.terms, want)
	}
	scan := &termScanStub{values: map[string]Constant{
		"a": NewIntConstant(1),
		"b": NewIntConstant(2),
	}}
	if got, err := joined.IsSatisfied(scan); err != nil || !got {
		t.Fatalf("joined.IsSatisfied(scan) = (%v, %v), want (true, nil)", got, err)
	}
	if want := []string{"a", "b", "a"}; !reflect.DeepEqual(scan.calls, want) {
		t.Fatalf("GetVal calls = %v, want %v", scan.calls, want)
	}

	if want := []Term{termA}; !reflect.DeepEqual(left.terms, want) {
		t.Fatalf("left terms = %v, want unchanged %v", left.terms, want)
	}
	if want := []Term{termB}; !reflect.DeepEqual(right.terms, want) {
		t.Fatalf("right terms = %v, want unchanged %v", right.terms, want)
	}

	joined.terms[0] = termB
	if want := []Term{termA}; !reflect.DeepEqual(left.terms, want) {
		t.Fatalf("left terms after result mutation = %v, want unchanged %v", left.terms, want)
	}

	var empty Predicate
	if got := empty.ConjoinWith(left); !reflect.DeepEqual(got.terms, left.terms) {
		t.Fatalf("empty.ConjoinWith(left) terms = %v, want %v", got.terms, left.terms)
	}
	if got := left.ConjoinWith(empty); !reflect.DeepEqual(got.terms, left.terms) {
		t.Fatalf("left.ConjoinWith(empty) terms = %v, want %v", got.terms, left.terms)
	}
}
