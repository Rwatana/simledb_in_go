package parse

import "github.com/Rwatana/simledb_in_go/internal/query"

// DeleteData holds the target table and predicate for a DELETE statement.
type DeleteData struct {
	tableName string
	predicate query.Predicate
}

// NewDeleteData constructs DELETE data from a table name and predicate.
func NewDeleteData(tableName string, predicate query.Predicate) DeleteData {
	return DeleteData{tableName: tableName, predicate: predicate}
}

// TableName returns the table targeted by the DELETE statement.
func (data DeleteData) TableName() string {
	return data.tableName
}

// Predicate returns the predicate that selects records to delete.
func (data DeleteData) Predicate() query.Predicate {
	return data.predicate
}
