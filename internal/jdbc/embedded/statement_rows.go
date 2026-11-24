package embedded

import (
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

type statementRows struct {
	scan    query.Scan
	schema  *record.Schema
	columns []string
	closed  bool
}

var _ driver.Rows = (*statementRows)(nil)
var _ driver.RowsColumnTypeDatabaseTypeName = (*statementRows)(nil)
var _ driver.RowsColumnTypeScanType = (*statementRows)(nil)
var _ driver.RowsColumnTypeLength = (*statementRows)(nil)

func newStatementRows(scan query.Scan, schema *record.Schema) (*statementRows, error) {
	if scan == nil {
		return nil, fmt.Errorf("create embedded statement rows: scan is nil")
	}
	if schema == nil {
		return nil, fmt.Errorf("create embedded statement rows: schema is nil")
	}
	return &statementRows{scan: scan, schema: schema, columns: schema.Fields()}, nil
}

func (rows *statementRows) Columns() []string {
	if rows == nil {
		return nil
	}
	return append([]string(nil), rows.columns...)
}

// ColumnTypeDatabaseTypeName reports the database type name for a result column.
func (rows *statementRows) ColumnTypeDatabaseTypeName(index int) string {
	if rows == nil || rows.schema == nil || index < 0 || index >= len(rows.columns) {
		return ""
	}
	definition, ok := rows.schema.Field(rows.columns[index])
	if !ok {
		return ""
	}
	switch definition.Type {
	case record.IntegerField:
		return "INTEGER"
	case record.VarcharField:
		return "VARCHAR"
	default:
		return ""
	}
}

// ColumnTypeScanType reports the Go type used to scan a result column.
func (rows *statementRows) ColumnTypeScanType(index int) reflect.Type {
	if rows == nil || rows.schema == nil || index < 0 || index >= len(rows.columns) {
		return nil
	}
	definition, ok := rows.schema.Field(rows.columns[index])
	if !ok {
		return nil
	}
	switch definition.Type {
	case record.IntegerField:
		return reflect.TypeOf(int64(0))
	case record.VarcharField:
		return reflect.TypeOf("")
	default:
		return nil
	}
}

// ColumnTypeLength reports a variable-length result column's declared length.
func (rows *statementRows) ColumnTypeLength(index int) (int64, bool) {
	if rows == nil || rows.schema == nil || index < 0 || index >= len(rows.columns) {
		return 0, false
	}
	definition, ok := rows.schema.Field(rows.columns[index])
	if !ok {
		return 0, false
	}
	switch definition.Type {
	case record.VarcharField:
		if definition.Length < 0 {
			return 0, false
		}
		return int64(definition.Length), true
	default:
		return 0, false
	}
}

func (rows *statementRows) Close() error {
	if rows == nil {
		return fmt.Errorf("close embedded statement rows: rows are nil")
	}
	if rows.closed {
		return nil
	}
	rows.closed = true
	if rows.scan == nil {
		return fmt.Errorf("close embedded statement rows: scan is nil")
	}
	if err := rows.scan.Close(); err != nil {
		return fmt.Errorf("close embedded statement rows scan: %w", err)
	}
	return nil
}

func (rows *statementRows) Next(destination []driver.Value) error {
	if rows == nil {
		return fmt.Errorf("advance embedded statement rows: rows are nil")
	}
	if rows.closed {
		return fmt.Errorf("advance embedded statement rows: rows are closed")
	}
	if rows.scan == nil || rows.schema == nil {
		return fmt.Errorf("advance embedded statement rows: scan or schema is nil")
	}
	if len(destination) != len(rows.columns) {
		return fmt.Errorf("advance embedded statement rows: destination has %d values for %d columns", len(destination), len(rows.columns))
	}
	available, err := rows.scan.Next()
	if err != nil {
		return fmt.Errorf("advance embedded statement rows scan: %w", err)
	}
	if !available {
		return io.EOF
	}
	for index, name := range rows.columns {
		definition, _ := rows.schema.Field(name)
		switch definition.Type {
		case record.IntegerField:
			value, err := rows.scan.GetInt(name)
			if err != nil {
				return fmt.Errorf("read embedded statement integer column %q: %w", name, err)
			}
			destination[index] = int64(value)
		case record.VarcharField:
			value, err := rows.scan.GetString(name)
			if err != nil {
				return fmt.Errorf("read embedded statement string column %q: %w", name, err)
			}
			destination[index] = value
		default:
			return fmt.Errorf("advance embedded statement rows: column %q has unsupported type %d", name, definition.Type)
		}
	}
	return nil
}
