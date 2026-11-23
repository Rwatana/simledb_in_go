package query

import "fmt"

// ProjectScan exposes a selected field list while delegating row access to a scan.
type ProjectScan struct {
	source Scan
	fields map[string]struct{}
}

var _ Scan = (*ProjectScan)(nil)

// NewProjectScan constructs a projection over source using the supplied field names.
func NewProjectScan(source Scan, fieldNames []string) (*ProjectScan, error) {
	if source == nil {
		return nil, fmt.Errorf("create project scan: source scan is nil")
	}
	fields := make(map[string]struct{}, len(fieldNames))
	for _, fieldName := range fieldNames {
		fields[fieldName] = struct{}{}
	}
	return &ProjectScan{source: source, fields: fields}, nil
}

func (scan *ProjectScan) sourceFor(operation string) (Scan, error) {
	if scan == nil || scan.source == nil {
		return nil, fmt.Errorf("%s project scan: source is not initialized", operation)
	}
	return scan.source, nil
}

// BeforeFirst positions the source scan before its first row.
func (scan *ProjectScan) BeforeFirst() error {
	source, err := scan.sourceFor("position before first row in")
	if err != nil {
		return err
	}
	if err := source.BeforeFirst(); err != nil {
		return fmt.Errorf("position before first row in project scan: %w", err)
	}
	return nil
}

// Next advances the source scan to its next row.
func (scan *ProjectScan) Next() (bool, error) {
	source, err := scan.sourceFor("advance")
	if err != nil {
		return false, err
	}
	next, err := source.Next()
	if err != nil {
		return false, fmt.Errorf("advance project scan: %w", err)
	}
	return next, nil
}

// GetInt reads an integer field from the current source row.
func (scan *ProjectScan) GetInt(fieldName string) (int32, error) {
	source, err := scan.sourceFor("read integer field")
	if err != nil {
		return 0, err
	}
	value, err := source.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q from project scan: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the current source row.
func (scan *ProjectScan) GetString(fieldName string) (string, error) {
	source, err := scan.sourceFor("read string field")
	if err != nil {
		return "", err
	}
	value, err := source.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q from project scan: %w", fieldName, err)
	}
	return value, nil
}

// GetVal reads a query value from the current source row.
func (scan *ProjectScan) GetVal(fieldName string) (Constant, error) {
	source, err := scan.sourceFor("read value field")
	if err != nil {
		return Constant{}, err
	}
	value, err := source.GetVal(fieldName)
	if err != nil {
		return Constant{}, fmt.Errorf("read value field %q from project scan: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether fieldName is part of the projection.
func (scan *ProjectScan) HasField(fieldName string) bool {
	if scan == nil {
		return false
	}
	_, ok := scan.fields[fieldName]
	return ok
}

// Close closes the source scan.
func (scan *ProjectScan) Close() error {
	source, err := scan.sourceFor("close")
	if err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("close project scan: %w", err)
	}
	return nil
}
