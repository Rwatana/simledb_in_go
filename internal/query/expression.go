package query

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// ValueScan is the portion of a scan required to evaluate a field expression.
type ValueScan interface {
	GetVal(name string) (Constant, error)
}

// Expression represents a literal query value or, in a later feature, a field reference.
type Expression struct {
	constant    Constant
	hasConstant bool
	fieldName   string
	hasField    bool
}

// NewConstantExpression constructs an expression that contains a literal constant.
func NewConstantExpression(constant Constant) Expression {
	return Expression{constant: constant, hasConstant: true}
}

// NewFieldExpression constructs an expression that refers to a field name.
func NewFieldExpression(name string) Expression {
	return Expression{fieldName: name, hasField: true}
}

// Evaluate returns a literal or reads the referenced field from scan.
func (expression Expression) Evaluate(scan ValueScan) (Constant, error) {
	if expression.hasConstant {
		return expression.constant, nil
	}
	if expression.hasField {
		if scan == nil {
			return Constant{}, fmt.Errorf("evaluate field %q: nil scan", expression.fieldName)
		}
		value, err := scan.GetVal(expression.fieldName)
		if err != nil {
			return Constant{}, err
		}
		return value, nil
	}
	return Constant{}, fmt.Errorf("evaluate expression: no literal or field reference")
}

// AppliesTo reports whether this expression's referenced field is present in schema.
// Literal expressions apply to every schema. An unset expression or nil schema does not apply.
func (expression Expression) AppliesTo(schema *record.Schema) bool {
	if expression.hasConstant {
		return true
	}
	if !expression.hasField || schema == nil {
		return false
	}
	return schema.HasField(expression.fieldName)
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
	return expression.hasField
}

// AsFieldName returns the referenced field name and whether this expression contains one.
func (expression Expression) AsFieldName() (string, bool) {
	if !expression.hasField {
		return "", false
	}
	return expression.fieldName, true
}
