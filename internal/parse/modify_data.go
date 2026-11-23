package parse

import "github.com/Rwatana/simledb_in_go/internal/query"

// ModifyData holds the target and new value of an UPDATE statement.
type ModifyData struct {
	tableName   string
	targetField string
	newValue    query.Expression
	predicate   query.Predicate
}

// NewModifyData constructs UPDATE data from its target, new value, and predicate.
func NewModifyData(tableName, targetField string, newValue query.Expression, predicate query.Predicate) ModifyData {
	return ModifyData{
		tableName:   tableName,
		targetField: targetField,
		newValue:    newValue,
		predicate:   predicate,
	}
}

// TableName returns the table targeted by the UPDATE statement.
func (data ModifyData) TableName() string {
	return data.tableName
}

// TargetField returns the field whose value will be changed.
func (data ModifyData) TargetField() string {
	return data.targetField
}

// NewValue returns the expression evaluated for the target field.
func (data ModifyData) NewValue() query.Expression {
	return data.newValue
}

// Predicate returns the predicate selecting records to modify.
func (data ModifyData) Predicate() query.Predicate {
	return data.predicate
}
