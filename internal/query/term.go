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

// EquatesWithConstant returns the literal compared with fieldName, if this term has that form.
func (term Term) EquatesWithConstant(fieldName string) (Constant, bool) {
	lhsField, lhsIsField := term.lhs.AsFieldName()
	rhsField, rhsIsField := term.rhs.AsFieldName()
	lhsConstant, lhsIsConstant := term.lhs.AsConstant()
	rhsConstant, rhsIsConstant := term.rhs.AsConstant()

	if lhsIsField && lhsField == fieldName && rhsIsConstant {
		return rhsConstant, true
	}
	if rhsIsField && rhsField == fieldName && lhsIsConstant {
		return lhsConstant, true
	}
	return Constant{}, false
}

// EquatesWithField returns the other field name when this term compares fieldName with another field.
func (term Term) EquatesWithField(fieldName string) (string, bool) {
	lhsField, lhsIsField := term.lhs.AsFieldName()
	rhsField, rhsIsField := term.rhs.AsFieldName()
	if lhsIsField && lhsField == fieldName && rhsIsField {
		return rhsField, true
	}
	if rhsIsField && rhsField == fieldName && lhsIsField {
		return lhsField, true
	}
	return "", false
}
