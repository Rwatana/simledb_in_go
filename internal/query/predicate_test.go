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

func TestPredicateSelectSubPredicate(t *testing.T) {
	termA := NewTerm(NewFieldExpression("a"), NewConstantExpression(NewIntConstant(1)))
	termAB := NewTerm(NewFieldExpression("a"), NewFieldExpression("b"))
	literalTerm := NewTerm(NewConstantExpression(NewIntConstant(1)), NewConstantExpression(NewIntConstant(1)))
	predicate := Predicate{terms: []Term{termA, termAB, literalTerm, termA}}

	selected, ok := predicate.SelectSubPredicate(schemaWithFields("a"))
	if !ok {
		t.Fatal("SelectSubPredicate(schemaWithFields(\"a\")) returned no predicate, want matches")
	}
	if want := []Term{termA, literalTerm, termA}; !reflect.DeepEqual(selected.terms, want) {
		t.Fatalf("selected terms = %v, want %v", selected.terms, want)
	}
	if want := []Term{termA, termAB, literalTerm, termA}; !reflect.DeepEqual(predicate.terms, want) {
		t.Fatalf("source terms = %v, want unchanged %v", predicate.terms, want)
	}

	noMatch, ok := NewPredicateWithTerm(termAB).SelectSubPredicate(schemaWithFields("a"))
	if ok || len(noMatch.terms) != 0 {
		t.Fatalf("no-match selection = (%v, %v), want (empty predicate, false)", noMatch.terms, ok)
	}

	selectedLiteral, ok := NewPredicateWithTerm(literalTerm).SelectSubPredicate(nil)
	if !ok || !reflect.DeepEqual(selectedLiteral.terms, []Term{literalTerm}) {
		t.Fatalf("literal selection for nil schema = (%v, %v), want one matching term", selectedLiteral.terms, ok)
	}

	var empty Predicate
	if selectedEmpty, ok := empty.SelectSubPredicate(schemaWithFields("a")); ok || len(selectedEmpty.terms) != 0 {
		t.Fatalf("empty predicate selection = (%v, %v), want (empty predicate, false)", selectedEmpty.terms, ok)
	}
}

func TestPredicateJoinSubPredicate(t *testing.T) {
	crossSchema := NewTerm(NewFieldExpression("left"), NewFieldExpression("right"))
	leftOnly := NewTerm(NewFieldExpression("left"), NewConstantExpression(NewIntConstant(1)))
	rightOnly := NewTerm(NewFieldExpression("right"), NewConstantExpression(NewIntConstant(2)))
	missingFromUnion := NewTerm(NewFieldExpression("left"), NewFieldExpression("missing"))
	literalOnly := NewTerm(NewConstantExpression(NewIntConstant(1)), NewConstantExpression(NewIntConstant(1)))
	predicate := Predicate{terms: []Term{crossSchema, leftOnly, rightOnly, missingFromUnion, literalOnly, crossSchema}}
	leftSchema := schemaWithFields("left")
	rightSchema := schemaWithFields("right")

	selected, ok := predicate.JoinSubPredicate(leftSchema, rightSchema)
	if !ok {
		t.Fatal("JoinSubPredicate(leftSchema, rightSchema) returned no predicate, want cross-schema terms")
	}
	if want := []Term{crossSchema, crossSchema}; !reflect.DeepEqual(selected.terms, want) {
		t.Fatalf("selected terms = %v, want %v", selected.terms, want)
	}
	if want := []string{"left"}; !reflect.DeepEqual(leftSchema.Fields(), want) {
		t.Fatalf("left schema fields = %v, want unchanged %v", leftSchema.Fields(), want)
	}
	if want := []string{"right"}; !reflect.DeepEqual(rightSchema.Fields(), want) {
		t.Fatalf("right schema fields = %v, want unchanged %v", rightSchema.Fields(), want)
	}
	if want := []Term{crossSchema, leftOnly, rightOnly, missingFromUnion, literalOnly, crossSchema}; !reflect.DeepEqual(predicate.terms, want) {
		t.Fatalf("source terms = %v, want unchanged %v", predicate.terms, want)
	}

	noMatch, ok := NewPredicateWithTerm(leftOnly).JoinSubPredicate(leftSchema, rightSchema)
	if ok || len(noMatch.terms) != 0 {
		t.Fatalf("no-match selection = (%v, %v), want (empty predicate, false)", noMatch.terms, ok)
	}
	var empty Predicate
	if selectedEmpty, ok := empty.JoinSubPredicate(leftSchema, rightSchema); ok || len(selectedEmpty.terms) != 0 {
		t.Fatalf("empty predicate selection = (%v, %v), want (empty predicate, false)", selectedEmpty.terms, ok)
	}
	if selectedNil, ok := NewPredicateWithTerm(crossSchema).JoinSubPredicate(nil, nil); ok || len(selectedNil.terms) != 0 {
		t.Fatalf("nil schema selection = (%v, %v), want (empty predicate, false)", selectedNil.terms, ok)
	}
	if selectedLiteral, ok := NewPredicateWithTerm(literalOnly).JoinSubPredicate(nil, nil); ok || len(selectedLiteral.terms) != 0 {
		t.Fatalf("nil schema literal selection = (%v, %v), want (empty predicate, false)", selectedLiteral.terms, ok)
	}
}

func TestPredicateEquatesWithConstant(t *testing.T) {
	zeroValue := Constant{}
	other := NewIntConstant(7)
	nonmatching := NewTerm(NewFieldExpression("other"), NewConstantExpression(other))
	firstMatch := NewTerm(NewFieldExpression("age"), NewConstantExpression(zeroValue))
	secondMatch := NewTerm(NewConstantExpression(other), NewFieldExpression("age"))
	predicate := Predicate{terms: []Term{nonmatching, firstMatch, secondMatch}}

	got, ok := predicate.EquatesWithConstant("age")
	if got != zeroValue || !ok {
		t.Fatalf("EquatesWithConstant(age) = (%v, %v), want present zero-value constant", got, ok)
	}
	if want := []Term{nonmatching, firstMatch, secondMatch}; !reflect.DeepEqual(predicate.terms, want) {
		t.Fatalf("predicate terms = %v, want unchanged %v", predicate.terms, want)
	}

	if got, ok := predicate.EquatesWithConstant("missing"); got != (Constant{}) || ok {
		t.Fatalf("EquatesWithConstant(missing) = (%v, %v), want absent", got, ok)
	}
	var empty Predicate
	if got, ok := empty.EquatesWithConstant("age"); got != (Constant{}) || ok {
		t.Fatalf("empty.EquatesWithConstant(age) = (%v, %v), want absent", got, ok)
	}
}

func TestPredicateEquatesWithField(t *testing.T) {
	nonmatching := NewTerm(NewFieldExpression("x"), NewFieldExpression("y"))
	firstMatch := NewTerm(NewFieldExpression("age"), NewFieldExpression("birth_year"))
	secondMatch := NewTerm(NewFieldExpression("age"), NewFieldExpression("created_at"))
	predicate := Predicate{terms: []Term{nonmatching, firstMatch, secondMatch}}

	got, ok := predicate.EquatesWithField("age")
	if got != "birth_year" || !ok {
		t.Fatalf("EquatesWithField(age) = (%q, %v), want (birth_year, true)", got, ok)
	}
	if want := []Term{nonmatching, firstMatch, secondMatch}; !reflect.DeepEqual(predicate.terms, want) {
		t.Fatalf("predicate terms = %v, want unchanged %v", predicate.terms, want)
	}

	emptyName := NewPredicateWithTerm(NewTerm(NewFieldExpression("age"), NewFieldExpression("")))
	if got, ok := emptyName.EquatesWithField("age"); got != "" || !ok {
		t.Fatalf("EquatesWithField(age) for empty opposite field = (%q, %v), want (empty, true)", got, ok)
	}
	if got, ok := predicate.EquatesWithField("missing"); got != "" || ok {
		t.Fatalf("EquatesWithField(missing) = (%q, %v), want absent", got, ok)
	}
	var empty Predicate
	if got, ok := empty.EquatesWithField("age"); got != "" || ok {
		t.Fatalf("empty.EquatesWithField(age) = (%q, %v), want absent", got, ok)
	}
}
