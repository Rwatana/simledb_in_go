package indexquery

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/index"
	"github.com/Rwatana/simledb_in_go/internal/query"
)

// IndexJoinScan retains the left scan, right table scan, and index for an index join.
type IndexJoinScan struct {
	lhs        query.Scan
	index      index.Index
	joinField  string
	rhs        *query.RecordScan
	hasLeftRow bool
}

// NewIndexJoinScan initializes the left scan and index at the first left join value.
func NewIndexJoinScan(lhs query.Scan, indexScan index.Index, joinField string, rhs *query.RecordScan) (*IndexJoinScan, error) {
	scan := &IndexJoinScan{lhs: lhs, index: indexScan, joinField: joinField, rhs: rhs}
	if err := scan.BeforeFirst(); err != nil {
		return nil, fmt.Errorf("initialize index join scan: %w", err)
	}
	return scan, nil
}

// BeforeFirst positions the left input and index at the first join value.
func (scan *IndexJoinScan) BeforeFirst() error {
	if scan == nil {
		return fmt.Errorf("position nil index join scan before first row")
	}
	scan.hasLeftRow = false
	if scan.lhs == nil {
		return fmt.Errorf("position index join scan: left scan is nil")
	}
	if scan.index == nil {
		return fmt.Errorf("position index join scan: index is nil")
	}
	if scan.rhs == nil || scan.rhs.TableScan == nil {
		return fmt.Errorf("position index join scan: right table scan is nil")
	}
	if err := scan.lhs.BeforeFirst(); err != nil {
		return fmt.Errorf("position index join scan left scan before first: %w", err)
	}
	found, err := scan.lhs.Next()
	if err != nil {
		return fmt.Errorf("position index join scan left scan input: %w", err)
	}
	if !found {
		return nil
	}
	joinValue, err := scan.lhs.GetVal(scan.joinField)
	if err != nil {
		return fmt.Errorf("read index join scan join field %q: %w", scan.joinField, err)
	}
	if err := scan.index.BeforeFirst(joinValue); err != nil {
		return fmt.Errorf("position index join scan index: %w", err)
	}
	scan.hasLeftRow = true
	return nil
}

// Next advances to the next record matching a left-side join value.
func (scan *IndexJoinScan) Next() (bool, error) {
	if scan == nil {
		return false, fmt.Errorf("advance nil index join scan")
	}
	if scan.lhs == nil {
		return false, fmt.Errorf("advance index join scan: left scan is nil")
	}
	if scan.index == nil {
		return false, fmt.Errorf("advance index join scan: index is nil")
	}
	if scan.rhs == nil || scan.rhs.TableScan == nil {
		return false, fmt.Errorf("advance index join scan: right table scan is nil")
	}
	if !scan.hasLeftRow {
		return false, nil
	}
	for {
		found, err := scan.index.Next()
		if err != nil {
			return false, fmt.Errorf("advance index join scan index: %w", err)
		}
		if found {
			rid, err := scan.index.GetDataRID()
			if err != nil {
				return false, fmt.Errorf("read index join scan RID: %w", err)
			}
			if err := scan.rhs.MoveToRID(rid); err != nil {
				return false, fmt.Errorf("move index join scan right table to RID %v: %w", rid, err)
			}
			return true, nil
		}
		found, err = scan.lhs.Next()
		if err != nil {
			return false, fmt.Errorf("advance index join scan left scan: %w", err)
		}
		if !found {
			scan.hasLeftRow = false
			return false, nil
		}
		joinValue, err := scan.lhs.GetVal(scan.joinField)
		if err != nil {
			return false, fmt.Errorf("read index join scan join field %q: %w", scan.joinField, err)
		}
		if err := scan.index.BeforeFirst(joinValue); err != nil {
			scan.hasLeftRow = false
			return false, fmt.Errorf("position index join scan index for left row: %w", err)
		}
	}
}

// GetInt reads an integer field from the right scan when present, otherwise from the left scan.
func (scan *IndexJoinScan) GetInt(fieldName string) (int32, error) {
	if scan == nil {
		return 0, fmt.Errorf("read integer field %q through nil index join scan", fieldName)
	}
	if scan.lhs == nil {
		return 0, fmt.Errorf("read integer field %q through index join scan: left scan is nil", fieldName)
	}
	if scan.rhs == nil || scan.rhs.TableScan == nil {
		return 0, fmt.Errorf("read integer field %q through index join scan: right table scan is nil", fieldName)
	}
	if scan.rhs.HasField(fieldName) {
		value, err := scan.rhs.GetInt(fieldName)
		if err != nil {
			return 0, fmt.Errorf("read integer field %q through index join scan right table: %w", fieldName, err)
		}
		return value, nil
	}
	value, err := scan.lhs.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q through index join scan left scan: %w", fieldName, err)
	}
	return value, nil
}

// GetVal reads a typed field from the right scan when present, otherwise from the left scan.
func (scan *IndexJoinScan) GetVal(fieldName string) (query.Constant, error) {
	if scan == nil {
		return query.Constant{}, fmt.Errorf("read field %q through nil index join scan", fieldName)
	}
	if scan.lhs == nil {
		return query.Constant{}, fmt.Errorf("read field %q through index join scan: left scan is nil", fieldName)
	}
	if scan.rhs == nil || scan.rhs.TableScan == nil {
		return query.Constant{}, fmt.Errorf("read field %q through index join scan: right table scan is nil", fieldName)
	}
	if scan.rhs.HasField(fieldName) {
		value, err := scan.rhs.GetVal(fieldName)
		if err != nil {
			return query.Constant{}, fmt.Errorf("read field %q through index join scan right table: %w", fieldName, err)
		}
		return value, nil
	}
	value, err := scan.lhs.GetVal(fieldName)
	if err != nil {
		return query.Constant{}, fmt.Errorf("read field %q through index join scan left scan: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the right scan when present, otherwise from the left scan.
func (scan *IndexJoinScan) GetString(fieldName string) (string, error) {
	if scan == nil {
		return "", fmt.Errorf("read string field %q through nil index join scan", fieldName)
	}
	if scan.lhs == nil {
		return "", fmt.Errorf("read string field %q through index join scan: left scan is nil", fieldName)
	}
	if scan.rhs == nil || scan.rhs.TableScan == nil {
		return "", fmt.Errorf("read string field %q through index join scan: right table scan is nil", fieldName)
	}
	if scan.rhs.HasField(fieldName) {
		value, err := scan.rhs.GetString(fieldName)
		if err != nil {
			return "", fmt.Errorf("read string field %q through index join scan right table: %w", fieldName, err)
		}
		return value, nil
	}
	value, err := scan.lhs.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q through index join scan left scan: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether either side of the join declares fieldName.
func (scan *IndexJoinScan) HasField(fieldName string) bool {
	if scan == nil || scan.lhs == nil || scan.rhs == nil || scan.rhs.TableScan == nil {
		return false
	}
	return scan.rhs.HasField(fieldName) || scan.lhs.HasField(fieldName)
}
