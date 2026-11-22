package query

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
