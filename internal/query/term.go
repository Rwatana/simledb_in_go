package query

// Term compares two expressions for equality.
type Term struct {
	lhs Expression
	rhs Expression
}

// NewTerm constructs an equality comparison between lhs and rhs.
func NewTerm(lhs, rhs Expression) Term {
	return Term{lhs: lhs, rhs: rhs}
}

// IsSatisfied evaluates both expressions from left to right and reports whether their values are equal.
func (term Term) IsSatisfied(scan ValueScan) (bool, error) {
	lhs, err := term.lhs.Evaluate(scan)
	if err != nil {
		return false, err
	}
	rhs, err := term.rhs.Evaluate(scan)
	if err != nil {
		return false, err
	}
	return lhs.Equal(rhs), nil
}
