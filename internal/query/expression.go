package query

// Expression represents a literal query value or, in a later feature, a field reference.
type Expression struct {
	constant    Constant
	hasConstant bool
}

// NewConstantExpression constructs an expression that contains a literal constant.
func NewConstantExpression(constant Constant) Expression {
	return Expression{constant: constant, hasConstant: true}
}

// AsConstant returns the literal constant and whether this expression contains one.
func (expression Expression) AsConstant() (Constant, bool) {
	if !expression.hasConstant {
		return Constant{}, false
	}
	return expression.constant, true
}

// IsFieldName reports whether this expression refers to a field.
func (expression Expression) IsFieldName() bool {
	return false
}
