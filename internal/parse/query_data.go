package parse

import (
	"strings"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

// QueryData holds the projection, source tables, and predicate of a SELECT statement.
// Its zero value represents an unset query and renders as "<unset-query>".
type QueryData struct {
	fields    []string
	tables    []string
	predicate query.Predicate
}

// NewQueryData constructs SELECT data and copies the supplied field and table lists.
func NewQueryData(fields, tables []string, predicate query.Predicate) QueryData {
	return QueryData{
		fields:    append([]string(nil), fields...),
		tables:    append([]string(nil), tables...),
		predicate: predicate,
	}
}

// Fields returns the selected field names in their original order.
func (data QueryData) Fields() []string {
	return append([]string(nil), data.fields...)
}

// Tables returns the source table names in their original order.
func (data QueryData) Tables() []string {
	return append([]string(nil), data.tables...)
}

// Predicate returns the query predicate by value.
func (data QueryData) Predicate() query.Predicate {
	return data.predicate
}

// String renders the SELECT statement in a stable, readable form.
func (data QueryData) String() string {
	if len(data.fields) == 0 || len(data.tables) == 0 {
		return "<unset-query>"
	}
	result := "select " + strings.Join(data.fields, ", ") + " from " + strings.Join(data.tables, ", ")
	if predicate := data.predicate.String(); predicate != "" {
		result += " where " + predicate
	}
	return result
}
