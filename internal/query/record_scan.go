package query

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// RecordScan adapts a record table scan to the query update-scan contract.
// The embedded table scan supplies traversal and update operations; this type
// adds query constants without making the record package depend on query.
type RecordScan struct {
	*record.TableScan
	schema *record.Schema
}

var _ UpdateScan = (*RecordScan)(nil)

// NewRecordScan creates a query scan for tableName using transaction and layout.
func NewRecordScan(transaction record.TableScanTransaction, tableName string, layout *record.Layout) (*RecordScan, error) {
	if transaction == nil {
		return nil, fmt.Errorf("create query scan for %q: transaction is nil", tableName)
	}
	if layout == nil {
		return nil, fmt.Errorf("create query scan for %q: layout is nil", tableName)
	}
	schema := layout.Schema()
	if schema == nil {
		return nil, fmt.Errorf("create query scan for %q: layout schema is nil", tableName)
	}
	tableScan, err := record.NewTableScan(transaction, tableName, layout)
	if err != nil {
		return nil, fmt.Errorf("create query scan for %q: %w", tableName, err)
	}
	return &RecordScan{TableScan: tableScan, schema: schema}, nil
}

// GetVal reads a field and converts it to the corresponding query constant.
func (scan *RecordScan) GetVal(fieldName string) (Constant, error) {
	if scan == nil || scan.TableScan == nil || scan.schema == nil {
		return Constant{}, fmt.Errorf("read query value for field %q: scan is not initialized", fieldName)
	}
	fieldType, ok := scan.schema.TypeOf(fieldName)
	if !ok {
		return Constant{}, fmt.Errorf("read query value for field %q: field is not in schema", fieldName)
	}
	switch fieldType {
	case record.IntegerField:
		value, err := scan.TableScan.GetInt(fieldName)
		if err != nil {
			return Constant{}, fmt.Errorf("read integer query value for field %q: %w", fieldName, err)
		}
		return NewIntConstant(value), nil
	case record.VarcharField:
		value, err := scan.TableScan.GetString(fieldName)
		if err != nil {
			return Constant{}, fmt.Errorf("read string query value for field %q: %w", fieldName, err)
		}
		return NewStringConstant(value), nil
	default:
		return Constant{}, fmt.Errorf("read query value for field %q: unsupported record field type %d", fieldName, fieldType)
	}
}

// SetVal writes value to a field when its query kind matches the schema type.
func (scan *RecordScan) SetVal(fieldName string, value Constant) error {
	if scan == nil || scan.TableScan == nil || scan.schema == nil {
		return fmt.Errorf("write query value for field %q: scan is not initialized", fieldName)
	}
	fieldType, ok := scan.schema.TypeOf(fieldName)
	if !ok {
		return fmt.Errorf("write query value for field %q: field is not in schema", fieldName)
	}
	switch fieldType {
	case record.IntegerField:
		integer, ok := value.AsInt()
		if !ok {
			return fmt.Errorf("write query value for field %q: expected integer constant", fieldName)
		}
		if err := scan.TableScan.SetInt(fieldName, integer); err != nil {
			return fmt.Errorf("write integer query value for field %q: %w", fieldName, err)
		}
		return nil
	case record.VarcharField:
		text, ok := value.AsString()
		if !ok {
			return fmt.Errorf("write query value for field %q: expected string constant", fieldName)
		}
		if err := scan.TableScan.SetString(fieldName, text); err != nil {
			return fmt.Errorf("write string query value for field %q: %w", fieldName, err)
		}
		return nil
	default:
		return fmt.Errorf("write query value for field %q: unsupported record field type %d", fieldName, fieldType)
	}
}
