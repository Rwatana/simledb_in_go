package query

import (
	"errors"
	"fmt"
)

// ProductScan enumerates the Cartesian product of two scans.
type ProductScan struct {
	left       Scan
	right      Scan
	leftActive bool
	resetRight bool
}

var _ Scan = (*ProductScan)(nil)

// NewProductScan constructs a left-major Cartesian product of left and right.
func NewProductScan(left, right Scan) (*ProductScan, error) {
	if left == nil {
		return nil, fmt.Errorf("create product scan: left scan is nil")
	}
	if right == nil {
		return nil, fmt.Errorf("create product scan: right scan is nil")
	}
	return &ProductScan{left: left, right: right}, nil
}

func (scan *ProductScan) inputs(operation string) (Scan, Scan, error) {
	if scan == nil || scan.left == nil || scan.right == nil {
		return nil, nil, fmt.Errorf("%s product scan: inputs are not initialized", operation)
	}
	return scan.left, scan.right, nil
}

func (scan *ProductScan) fieldSource(fieldName string) (Scan, error) {
	left, right, err := scan.inputs("find field")
	if err != nil {
		return nil, err
	}
	if left.HasField(fieldName) {
		return left, nil
	}
	if right.HasField(fieldName) {
		return right, nil
	}
	return nil, fmt.Errorf("product scan: field %q is not present in either input", fieldName)
}

// BeforeFirst resets both inputs and positions the product before its first pair.
func (scan *ProductScan) BeforeFirst() error {
	left, right, err := scan.inputs("position before first pair in")
	if err != nil {
		return err
	}

	leftErr := left.BeforeFirst()
	rightErr := right.BeforeFirst()
	scan.leftActive = false
	scan.resetRight = false
	var resetErrors []error
	if leftErr != nil {
		resetErrors = append(resetErrors, fmt.Errorf("reset left input of product scan: %w", leftErr))
	}
	if rightErr != nil {
		resetErrors = append(resetErrors, fmt.Errorf("reset right input of product scan: %w", rightErr))
	}
	if len(resetErrors) > 0 {
		return errors.Join(resetErrors...)
	}
	return nil
}

// Next advances to the next left-major pair, restarting the right input per left row.
func (scan *ProductScan) Next() (bool, error) {
	left, right, err := scan.inputs("advance")
	if err != nil {
		return false, err
	}
	for {
		if !scan.leftActive {
			hasLeft, err := left.Next()
			if err != nil {
				return false, fmt.Errorf("advance left input in product scan: %w", err)
			}
			if !hasLeft {
				return false, nil
			}
			scan.leftActive = true
		}
		if scan.resetRight {
			if err := right.BeforeFirst(); err != nil {
				return false, fmt.Errorf("restart right input in product scan: %w", err)
			}
			scan.resetRight = false
		}
		hasRight, err := right.Next()
		if err != nil {
			return false, fmt.Errorf("advance right input in product scan: %w", err)
		}
		if hasRight {
			return true, nil
		}
		scan.leftActive = false
		scan.resetRight = true
	}
}

// GetInt reads an integer field from the current product row.
func (scan *ProductScan) GetInt(fieldName string) (int32, error) {
	source, err := scan.fieldSource(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q from product scan: %w", fieldName, err)
	}
	value, err := source.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q from product scan: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the current product row.
func (scan *ProductScan) GetString(fieldName string) (string, error) {
	source, err := scan.fieldSource(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q from product scan: %w", fieldName, err)
	}
	value, err := source.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q from product scan: %w", fieldName, err)
	}
	return value, nil
}

// GetVal reads a query value from the current product row.
func (scan *ProductScan) GetVal(fieldName string) (Constant, error) {
	source, err := scan.fieldSource(fieldName)
	if err != nil {
		return Constant{}, fmt.Errorf("read value field %q from product scan: %w", fieldName, err)
	}
	value, err := source.GetVal(fieldName)
	if err != nil {
		return Constant{}, fmt.Errorf("read value field %q from product scan: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether either input contains fieldName.
func (scan *ProductScan) HasField(fieldName string) bool {
	left, right, err := scan.inputs("check field")
	return err == nil && (left.HasField(fieldName) || right.HasField(fieldName))
}

// Close closes both input scans and returns all close failures.
func (scan *ProductScan) Close() error {
	left, right, err := scan.inputs("close")
	if err != nil {
		return err
	}
	var closeErrors []error
	if err := left.Close(); err != nil {
		closeErrors = append(closeErrors, fmt.Errorf("close left input of product scan: %w", err))
	}
	if err := right.Close(); err != nil {
		closeErrors = append(closeErrors, fmt.Errorf("close right input of product scan: %w", err))
	}
	return errors.Join(closeErrors...)
}
