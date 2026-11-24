package materialize

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

// RecordComparator compares query values in the order of its retained sort fields.
type RecordComparator struct {
	fields []string
}

// NewRecordComparator creates a comparator that owns a copy of fields.
func NewRecordComparator(fields []string) *RecordComparator {
	return &RecordComparator{fields: append([]string(nil), fields...)}
}

// Compare returns the lexicographic ordering of the current values in left and right.
func (comparator *RecordComparator) Compare(left, right query.ValueScan) (int, error) {
	if comparator == nil {
		return 0, fmt.Errorf("compare records: comparator is nil")
	}
	if len(comparator.fields) == 0 {
		return 0, nil
	}
	if left == nil {
		return 0, fmt.Errorf("compare records: left scan is nil")
	}
	if right == nil {
		return 0, fmt.Errorf("compare records: right scan is nil")
	}
	for _, fieldName := range comparator.fields {
		leftValue, err := left.GetVal(fieldName)
		if err != nil {
			return 0, fmt.Errorf("compare records at field %q: read left value: %w", fieldName, err)
		}
		rightValue, err := right.GetVal(fieldName)
		if err != nil {
			return 0, fmt.Errorf("compare records at field %q: read right value: %w", fieldName, err)
		}
		result, comparable := leftValue.Compare(rightValue)
		if !comparable {
			return 0, fmt.Errorf("compare records at field %q: values %q and %q are not comparable", fieldName, leftValue, rightValue)
		}
		if result != 0 {
			return result, nil
		}
	}
	return 0, nil
}
