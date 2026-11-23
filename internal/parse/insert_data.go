package parse

import "github.com/Rwatana/simledb_in_go/internal/query"

// InsertData holds the table, fields, and values of an INSERT statement.
type InsertData struct {
	tableName string
	fields    []string
	values    []query.Constant
}

// NewInsertData constructs INSERT data and copies the supplied field and value lists.
func NewInsertData(tableName string, fields []string, values []query.Constant) InsertData {
	return InsertData{
		tableName: tableName,
		fields:    append([]string(nil), fields...),
		values:    append([]query.Constant(nil), values...),
	}
}

// TableName returns the table targeted by the INSERT statement.
func (data InsertData) TableName() string {
	return data.tableName
}

// Fields returns the inserted field names in their original order.
func (data InsertData) Fields() []string {
	return append([]string(nil), data.fields...)
}

// Values returns the constants corresponding to the inserted fields in order.
func (data InsertData) Values() []query.Constant {
	return append([]query.Constant(nil), data.values...)
}
