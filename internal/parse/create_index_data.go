package parse

// CreateIndexData holds the names in a CREATE INDEX statement.
type CreateIndexData struct {
	indexName string
	tableName string
	fieldName string
}

// NewCreateIndexData constructs CREATE INDEX data from the index, table, and field names.
func NewCreateIndexData(indexName, tableName, fieldName string) CreateIndexData {
	return CreateIndexData{indexName: indexName, tableName: tableName, fieldName: fieldName}
}

// IndexName returns the name of the index to create.
func (data CreateIndexData) IndexName() string {
	return data.indexName
}

// TableName returns the name of the table containing the indexed field.
func (data CreateIndexData) TableName() string {
	return data.tableName
}

// FieldName returns the name of the field to index.
func (data CreateIndexData) FieldName() string {
	return data.fieldName
}
