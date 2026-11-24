package materialize

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

// SortScan merges one or two sorted temporary-table scans into ascending order.
type SortScan struct {
	first      query.UpdateScan
	second     query.UpdateScan
	comparator *RecordComparator
	current    query.UpdateScan
	hasFirst   bool
	hasSecond  bool
	closed     bool
}

var _ query.Scan = (*SortScan)(nil)

// NewSortScan opens and merges one or two already-sorted temporary-table runs.
// The returned scan owns both opened scans and must be closed by its caller.
func NewSortScan(runs []*TempTable, comparator *RecordComparator) (*SortScan, error) {
	if len(runs) < 1 || len(runs) > 2 {
		return nil, fmt.Errorf("create sorted scan: expected one or two runs, got %d", len(runs))
	}
	if comparator == nil {
		return nil, fmt.Errorf("create sorted scan: comparator is nil")
	}
	for i, run := range runs {
		if run == nil || run.transaction == nil || run.tableName == "" || run.layout == nil || run.layout.Schema() == nil {
			return nil, fmt.Errorf("create sorted scan: run %d is incomplete", i+1)
		}
	}

	first, err := runs[0].Open()
	if err != nil {
		return nil, fmt.Errorf("create sorted scan: open first run: %w", err)
	}
	scan := &SortScan{first: first, comparator: comparator}
	if len(runs) == 2 {
		second, err := runs[1].Open()
		if err != nil {
			openErr := fmt.Errorf("create sorted scan: open second run: %w", err)
			return nil, errors.Join(openErr, closeSortedRuns(first, nil))
		}
		scan.second = second
	}
	if scan.hasFirst, err = scan.first.Next(); err != nil {
		positionErr := fmt.Errorf("create sorted scan: position first run: %w", err)
		return nil, errors.Join(positionErr, closeSortedRuns(scan.first, scan.second))
	}
	if scan.second != nil {
		if scan.hasSecond, err = scan.second.Next(); err != nil {
			positionErr := fmt.Errorf("create sorted scan: position second run: %w", err)
			return nil, errors.Join(positionErr, closeSortedRuns(scan.first, scan.second))
		}
	}
	return scan, nil
}

// BeforeFirst rewinds both sorted runs and clears the selected record.
func (scan *SortScan) BeforeFirst() error {
	if scan == nil {
		return fmt.Errorf("rewind sorted scan: scan is nil")
	}
	if scan.closed {
		return fmt.Errorf("rewind sorted scan: scan is closed")
	}
	scan.current = nil
	scan.hasFirst = false
	scan.hasSecond = false
	first, err := rewindSortedRun(scan.first, "first")
	if err != nil {
		return err
	}
	scan.hasFirst = first
	if scan.second == nil {
		return nil
	}
	second, err := rewindSortedRun(scan.second, "second")
	if err != nil {
		scan.hasFirst = false
		return err
	}
	scan.hasSecond = second
	return nil
}

// Next selects the next lowest record from the remaining run heads.
func (scan *SortScan) Next() (bool, error) {
	if scan == nil {
		return false, fmt.Errorf("advance sorted scan: scan is nil")
	}
	if scan.closed {
		return false, fmt.Errorf("advance sorted scan: scan is closed")
	}
	if scan.current != nil {
		if scan.current == scan.first {
			hasMore, err := scan.first.Next()
			if err != nil {
				return false, fmt.Errorf("advance first sorted run: %w", err)
			}
			scan.hasFirst = hasMore
		} else {
			hasMore, err := scan.second.Next()
			if err != nil {
				return false, fmt.Errorf("advance second sorted run: %w", err)
			}
			scan.hasSecond = hasMore
		}
	}
	scan.current = nil

	switch {
	case !scan.hasFirst && !scan.hasSecond:
		return false, nil
	case scan.hasFirst && scan.hasSecond:
		order, err := scan.comparator.Compare(scan.first, scan.second)
		if err != nil {
			return false, fmt.Errorf("compare sorted run heads: %w", err)
		}
		if order < 0 {
			scan.current = scan.first
		} else {
			scan.current = scan.second
		}
	case scan.hasFirst:
		scan.current = scan.first
	default:
		scan.current = scan.second
	}
	return true, nil
}

// GetVal reads a query constant from the selected record.
func (scan *SortScan) GetVal(fieldName string) (query.Constant, error) {
	current, err := scan.activeRun()
	if err != nil {
		return query.Constant{}, fmt.Errorf("read sorted value for field %q: %w", fieldName, err)
	}
	value, err := current.GetVal(fieldName)
	if err != nil {
		return query.Constant{}, fmt.Errorf("read sorted value for field %q: %w", fieldName, err)
	}
	return value, nil
}

// GetInt reads an integer field from the selected record.
func (scan *SortScan) GetInt(fieldName string) (int32, error) {
	current, err := scan.activeRun()
	if err != nil {
		return 0, fmt.Errorf("read sorted integer field %q: %w", fieldName, err)
	}
	value, err := current.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read sorted integer field %q: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the selected record.
func (scan *SortScan) GetString(fieldName string) (string, error) {
	current, err := scan.activeRun()
	if err != nil {
		return "", fmt.Errorf("read sorted string field %q: %w", fieldName, err)
	}
	value, err := current.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read sorted string field %q: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether the selected record contains fieldName.
func (scan *SortScan) HasField(fieldName string) bool {
	current, err := scan.activeRun()
	return err == nil && current.HasField(fieldName)
}

// Close releases both input scans and preserves any close failures.
func (scan *SortScan) Close() error {
	if scan == nil {
		return fmt.Errorf("close sorted scan: scan is nil")
	}
	if scan.closed {
		return nil
	}
	scan.closed = true
	scan.current = nil
	return closeSortedRuns(scan.first, scan.second)
}

func (scan *SortScan) activeRun() (query.UpdateScan, error) {
	if scan == nil {
		return nil, fmt.Errorf("scan is nil")
	}
	if scan.closed {
		return nil, fmt.Errorf("scan is closed")
	}
	if scan.current == nil {
		return nil, fmt.Errorf("scan is not positioned on a record")
	}
	return scan.current, nil
}

func rewindSortedRun(run query.UpdateScan, name string) (bool, error) {
	if err := run.BeforeFirst(); err != nil {
		return false, fmt.Errorf("rewind %s sorted run: %w", name, err)
	}
	hasMore, err := run.Next()
	if err != nil {
		return false, fmt.Errorf("position %s sorted run after rewind: %w", name, err)
	}
	return hasMore, nil
}

func closeSortedRuns(first, second query.UpdateScan) error {
	var closeErr error
	if first != nil {
		if err := first.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close first sorted run: %w", err))
		}
	}
	if second != nil {
		if err := second.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close second sorted run: %w", err))
		}
	}
	return closeErr
}
