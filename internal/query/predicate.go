package query

import "github.com/Rwatana/simledb_in_go/internal/record"

// Predicate represents a conjunction of query terms.
// Its zero value is an empty conjunction that is always satisfied.
type Predicate struct {
	terms []Term
}

// NewPredicateWithTerm constructs a predicate containing one term.
func NewPredicateWithTerm(term Term) Predicate {
	return Predicate{terms: []Term{term}}
}

// ConjoinWith returns a predicate containing this predicate's terms followed by other's terms.
// Neither input predicate is modified.
func (predicate Predicate) ConjoinWith(other Predicate) Predicate {
	terms := make([]Term, 0, len(predicate.terms)+len(other.terms))
	terms = append(terms, predicate.terms...)
	terms = append(terms, other.terms...)
	return Predicate{terms: terms}
}

// SelectSubPredicate returns the terms that apply to schema and whether any terms matched.
func (predicate Predicate) SelectSubPredicate(schema *record.Schema) (Predicate, bool) {
	selected := Predicate{}
	for _, term := range predicate.terms {
		if term.AppliesTo(schema) {
			selected.terms = append(selected.terms, term)
		}
	}
	if len(selected.terms) == 0 {
		return Predicate{}, false
	}
	return selected, true
}

// JoinSubPredicate returns the terms that apply to the schemas' union but not to either schema alone.
func (predicate Predicate) JoinSubPredicate(first, second *record.Schema) (Predicate, bool) {
	union := &record.Schema{}
	if first != nil {
		union.AddAllFrom(first)
	}
	if second != nil {
		union.AddAllFrom(second)
	}

	selected := Predicate{}
	for _, term := range predicate.terms {
		if !term.AppliesTo(first) && !term.AppliesTo(second) && term.AppliesTo(union) {
			selected.terms = append(selected.terms, term)
		}
	}
	if len(selected.terms) == 0 {
		return Predicate{}, false
	}
	return selected, true
}

// IsSatisfied reports whether every term in the predicate is satisfied by scan.
func (predicate Predicate) IsSatisfied(scan ValueScan) (bool, error) {
	for _, term := range predicate.terms {
		satisfied, err := term.IsSatisfied(scan)
		if err != nil {
			return false, err
		}
		if !satisfied {
			return false, nil
		}
	}
	return true, nil
}
