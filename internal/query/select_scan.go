package query

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// SelectScan filters an input scan using a query predicate.
type SelectScan struct {
	source    Scan
	predicate Predicate
}

var _ UpdateScan = (*SelectScan)(nil)

// NewSelectScan constructs a selection scan over source using predicate.
func NewSelectScan(source Scan, predicate Predicate) (*SelectScan, error) {
	if source == nil {
		return nil, fmt.Errorf("create select scan: source scan is nil")
	}
	return &SelectScan{source: source, predicate: predicate}, nil
}

func (scan *SelectScan) sourceFor(operation string) (Scan, error) {
	if scan == nil || scan.source == nil {
		return nil, fmt.Errorf("%s select scan: source is not initialized", operation)
	}
	return scan.source, nil
}

func (scan *SelectScan) updateSource(operation string) (UpdateScan, error) {
	source, err := scan.sourceFor(operation)
	if err != nil {
		return nil, err
	}
	update, ok := source.(UpdateScan)
	if !ok {
		return nil, fmt.Errorf("%s select scan: source does not support updates", operation)
	}
	return update, nil
}

// BeforeFirst positions the source before its first row.
func (scan *SelectScan) BeforeFirst() error {
	source, err := scan.sourceFor("position before first row in")
	if err != nil {
		return err
	}
	if err := source.BeforeFirst(); err != nil {
		return fmt.Errorf("position before first row in select scan: %w", err)
	}
	return nil
}

// Next advances until a row satisfies the selection predicate or the source ends.
func (scan *SelectScan) Next() (bool, error) {
	source, err := scan.sourceFor("advance")
	if err != nil {
		return false, err
	}
	for {
		next, err := source.Next()
		if err != nil {
			return false, fmt.Errorf("advance select scan: %w", err)
		}
		if !next {
			return false, nil
		}
		satisfied, err := scan.predicate.IsSatisfied(source)
		if err != nil {
			return false, fmt.Errorf("evaluate select predicate: %w", err)
		}
		if satisfied {
			return true, nil
		}
	}
}

// GetInt reads an integer field from the current selected row.
func (scan *SelectScan) GetInt(fieldName string) (int32, error) {
	source, err := scan.sourceFor("read integer field")
	if err != nil {
		return 0, err
	}
	value, err := source.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q from select scan: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the current selected row.
func (scan *SelectScan) GetString(fieldName string) (string, error) {
	source, err := scan.sourceFor("read string field")
	if err != nil {
		return "", err
	}
	value, err := source.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q from select scan: %w", fieldName, err)
	}
	return value, nil
}

// GetVal reads a query value from the current selected row.
func (scan *SelectScan) GetVal(fieldName string) (Constant, error) {
	source, err := scan.sourceFor("read value field")
	if err != nil {
		return Constant{}, err
	}
	value, err := source.GetVal(fieldName)
	if err != nil {
		return Constant{}, fmt.Errorf("read value field %q from select scan: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether the source scan contains fieldName.
func (scan *SelectScan) HasField(fieldName string) bool {
	source, err := scan.sourceFor("check field")
	return err == nil && source.HasField(fieldName)
}

// Close closes the source scan.
func (scan *SelectScan) Close() error {
	source, err := scan.sourceFor("close")
	if err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("close select scan: %w", err)
	}
	return nil
}

// SetInt writes an integer field through an update-capable source.
func (scan *SelectScan) SetInt(fieldName string, value int32) error {
	source, err := scan.updateSource("set integer field")
	if err != nil {
		return err
	}
	if err := source.SetInt(fieldName, value); err != nil {
		return fmt.Errorf("set integer field %q through select scan: %w", fieldName, err)
	}
	return nil
}

// SetString writes a string field through an update-capable source.
func (scan *SelectScan) SetString(fieldName, value string) error {
	source, err := scan.updateSource("set string field")
	if err != nil {
		return err
	}
	if err := source.SetString(fieldName, value); err != nil {
		return fmt.Errorf("set string field %q through select scan: %w", fieldName, err)
	}
	return nil
}

// SetVal writes a query value through an update-capable source.
func (scan *SelectScan) SetVal(fieldName string, value Constant) error {
	source, err := scan.updateSource("set value field")
	if err != nil {
		return err
	}
	if err := source.SetVal(fieldName, value); err != nil {
		return fmt.Errorf("set value field %q through select scan: %w", fieldName, err)
	}
	return nil
}

// Insert inserts a row through an update-capable source.
func (scan *SelectScan) Insert() error {
	source, err := scan.updateSource("insert through")
	if err != nil {
		return err
	}
	if err := source.Insert(); err != nil {
		return fmt.Errorf("insert through select scan: %w", err)
	}
	return nil
}

// Delete deletes the current row through an update-capable source.
func (scan *SelectScan) Delete() error {
	source, err := scan.updateSource("delete through")
	if err != nil {
		return err
	}
	if err := source.Delete(); err != nil {
		return fmt.Errorf("delete through select scan: %w", err)
	}
	return nil
}

// GetRID returns the current row's identifier through an update-capable source.
func (scan *SelectScan) GetRID() (record.RID, error) {
	source, err := scan.updateSource("get row identifier from")
	if err != nil {
		return record.RID{}, err
	}
	rid, err := source.GetRID()
	if err != nil {
		return record.RID{}, fmt.Errorf("get row identifier from select scan: %w", err)
	}
	return rid, nil
}

// MoveToRID positions the source at rid through an update-capable source.
func (scan *SelectScan) MoveToRID(rid record.RID) error {
	source, err := scan.updateSource("move select scan to row")
	if err != nil {
		return err
	}
	if err := source.MoveToRID(rid); err != nil {
		return fmt.Errorf("move select scan to row %s: %w", rid, err)
	}
	return nil
}
