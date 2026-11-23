package query

// Predicate represents a conjunction of query terms.
// Its zero value is an empty conjunction that is always satisfied.
type Predicate struct{}

// IsSatisfied reports whether this empty predicate is true without reading scan.
func (Predicate) IsSatisfied(ValueScan) (bool, error) {
	return true, nil
}
