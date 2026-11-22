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
