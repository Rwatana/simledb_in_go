package parse

import "github.com/Rwatana/simledb_in_go/internal/record"

// CreateTableData holds the name and schema of a CREATE TABLE statement.
type CreateTableData struct {
	tableName string
	schema    record.Schema
}

// NewCreateTableData constructs CREATE TABLE data and copies the schema.
func NewCreateTableData(tableName string, schema record.Schema) CreateTableData {
	return CreateTableData{tableName: tableName, schema: copySchema(schema)}
}

// TableName returns the name of the table to create.
func (data CreateTableData) TableName() string {
	return data.tableName
}

// Schema returns an independent copy of the table schema.
func (data CreateTableData) Schema() record.Schema {
	return copySchema(data.schema)
}

func copySchema(schema record.Schema) record.Schema {
	var copied record.Schema
	for _, field := range schema.Fields() {
		definition, _ := schema.Field(field)
		copied.AddField(field, definition.Type, definition.Length)
	}
	return copied
}
