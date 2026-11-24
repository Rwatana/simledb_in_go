package plan

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/multibuffer"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// ChunkValueScan adapts a multibuffer chunk scan to query value access.
type ChunkValueScan struct {
	*multibuffer.ChunkScan
	schema *record.Schema
}

var _ query.Scan = (*ChunkValueScan)(nil)

// NewChunkValueScan wraps scan with query constants using layout's schema.
func NewChunkValueScan(scan *multibuffer.ChunkScan, layout *record.Layout) (*ChunkValueScan, error) {
	if scan == nil {
		return nil, fmt.Errorf("create chunk value scan: chunk scan is nil")
	}
	if layout == nil {
		return nil, fmt.Errorf("create chunk value scan: layout is nil")
	}
	schema := layout.Schema()
	if schema == nil {
		return nil, fmt.Errorf("create chunk value scan: layout schema is nil")
	}
	return &ChunkValueScan{ChunkScan: scan, schema: schema}, nil
}

// GetVal reads the current field and converts it to a query constant.
func (scan *ChunkValueScan) GetVal(fieldName string) (query.Constant, error) {
	if scan == nil || scan.ChunkScan == nil || scan.schema == nil {
		return query.Constant{}, fmt.Errorf("read query value for field %q: chunk value scan is not initialized", fieldName)
	}
	fieldType, ok := scan.schema.TypeOf(fieldName)
	if !ok {
		return query.Constant{}, fmt.Errorf("read query value for field %q: field is not in schema", fieldName)
	}
	switch fieldType {
	case record.IntegerField:
		value, err := scan.ChunkScan.GetInt(fieldName)
		if err != nil {
			return query.Constant{}, fmt.Errorf("read integer query value for field %q: %w", fieldName, err)
		}
		return query.NewIntConstant(value), nil
	case record.VarcharField:
		value, err := scan.ChunkScan.GetString(fieldName)
		if err != nil {
			return query.Constant{}, fmt.Errorf("read string query value for field %q: %w", fieldName, err)
		}
		return query.NewStringConstant(value), nil
	default:
		return query.Constant{}, fmt.Errorf("read query value for field %q: unsupported record field type %d", fieldName, fieldType)
	}
}
