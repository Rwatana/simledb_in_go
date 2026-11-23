package query

import (
	"math"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// DistinctValueEstimator provides field cardinality estimates to query terms.
type DistinctValueEstimator interface {
	// DistinctValues estimates the number of distinct values for fieldName.
	DistinctValues(fieldName string) int
}

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

// ReductionFactor estimates how much this equality term reduces result rows.
// It returns the larger estimate for two fields, the field estimate for a
// field compared with a constant, one for equal constants, and math.MaxInt32
// for unequal constants. A missing estimator yields zero for field terms.
func (term Term) ReductionFactor(estimator DistinctValueEstimator) int {
	lhsField, lhsIsField := term.lhs.AsFieldName()
	rhsField, rhsIsField := term.rhs.AsFieldName()
	if lhsIsField && rhsIsField {
		if estimator == nil {
			return 0
		}
		lhsEstimate := estimator.DistinctValues(lhsField)
		rhsEstimate := estimator.DistinctValues(rhsField)
		if lhsEstimate > rhsEstimate {
			return lhsEstimate
		}
		return rhsEstimate
	}
	if lhsIsField {
		if estimator == nil {
			return 0
		}
		return estimator.DistinctValues(lhsField)
	}
	if rhsIsField {
		if estimator == nil {
			return 0
		}
		return estimator.DistinctValues(rhsField)
	}

	lhsConstant, lhsIsConstant := term.lhs.AsConstant()
	rhsConstant, rhsIsConstant := term.rhs.AsConstant()
	if lhsIsConstant && rhsIsConstant && lhsConstant.Equal(rhsConstant) {
		return 1
	}
	return math.MaxInt32
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

// AppliesTo reports whether both expressions in this term apply to schema.
func (term Term) AppliesTo(schema *record.Schema) bool {
	return term.lhs.AppliesTo(schema) && term.rhs.AppliesTo(schema)
}

// String renders the term's left and right expressions separated by an equals sign.
func (term Term) String() string {
	return term.lhs.String() + "=" + term.rhs.String()
}
