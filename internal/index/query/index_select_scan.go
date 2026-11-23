// Package indexquery adapts an index lookup to query scans.
package indexquery

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/index"
	"github.com/Rwatana/simledb_in_go/internal/query"
)

// IndexSelectScan retains the data scan, index, and key for an indexed selection.
type IndexSelectScan struct {
	dataScan  *query.RecordScan
	index     index.Index
	searchKey query.Constant
}

// NewIndexSelectScan initializes an indexed selection positioned before its first result.
func NewIndexSelectScan(dataScan *query.RecordScan, indexScan index.Index, searchKey query.Constant) (*IndexSelectScan, error) {
	if dataScan == nil {
		return nil, fmt.Errorf("initialize index select scan: table scan is nil")
	}
	if indexScan == nil {
		return nil, fmt.Errorf("initialize index select scan: index is nil")
	}
	if _, ok := searchKey.HashCode(); !ok {
		return nil, fmt.Errorf("initialize index select scan: search key is unset")
	}
	scan := &IndexSelectScan{dataScan: dataScan, index: indexScan, searchKey: searchKey}
	if err := scan.BeforeFirst(); err != nil {
		return nil, fmt.Errorf("initialize index select scan: position before first match: %w", err)
	}
	return scan, nil
}

// BeforeFirst positions the index before entries matching the stored search key.
func (scan *IndexSelectScan) BeforeFirst() error {
	if scan == nil {
		return fmt.Errorf("position nil index select scan before first match")
	}
	if scan.index == nil {
		return fmt.Errorf("position index select scan: index is nil")
	}
	if _, ok := scan.searchKey.HashCode(); !ok {
		return fmt.Errorf("position index select scan: search key is unset")
	}
	if err := scan.index.BeforeFirst(scan.searchKey); err != nil {
		return fmt.Errorf("position index select scan before key %s: %w", scan.searchKey, err)
	}
	return nil
}

// Next advances the index and moves the data scan to the matching record.
func (scan *IndexSelectScan) Next() (bool, error) {
	if scan == nil {
		return false, fmt.Errorf("advance nil index select scan")
	}
	if scan.index == nil {
		return false, fmt.Errorf("advance index select scan: index is nil")
	}
	if scan.dataScan == nil {
		return false, fmt.Errorf("advance index select scan: table scan is nil")
	}
	if _, ok := scan.searchKey.HashCode(); !ok {
		return false, fmt.Errorf("advance index select scan: search key is unset")
	}
	found, err := scan.index.Next()
	if err != nil {
		return false, fmt.Errorf("advance index select scan index: %w", err)
	}
	if !found {
		return false, nil
	}
	rid, err := scan.index.GetDataRID()
	if err != nil {
		return false, fmt.Errorf("read index select scan data RID: %w", err)
	}
	if err := scan.dataScan.MoveToRID(rid); err != nil {
		return false, fmt.Errorf("move index select scan data table to RID %v: %w", rid, err)
	}
	return true, nil
}

// GetInt reads an integer field from the current data record.
func (scan *IndexSelectScan) GetInt(fieldName string) (int32, error) {
	if scan == nil {
		return 0, fmt.Errorf("read integer field %q through nil index select scan", fieldName)
	}
	if scan.dataScan == nil {
		return 0, fmt.Errorf("read integer field %q through index select scan: table scan is nil", fieldName)
	}
	value, err := scan.dataScan.GetInt(fieldName)
	if err != nil {
		return 0, fmt.Errorf("read integer field %q through index select scan: %w", fieldName, err)
	}
	return value, nil
}

// GetString reads a string field from the current data record.
func (scan *IndexSelectScan) GetString(fieldName string) (string, error) {
	if scan == nil {
		return "", fmt.Errorf("read string field %q through nil index select scan", fieldName)
	}
	if scan.dataScan == nil {
		return "", fmt.Errorf("read string field %q through index select scan: table scan is nil", fieldName)
	}
	value, err := scan.dataScan.GetString(fieldName)
	if err != nil {
		return "", fmt.Errorf("read string field %q through index select scan: %w", fieldName, err)
	}
	return value, nil
}

// GetVal reads a typed query value from the current data record.
func (scan *IndexSelectScan) GetVal(fieldName string) (query.Constant, error) {
	if scan == nil {
		return query.Constant{}, fmt.Errorf("read field %q through nil index select scan", fieldName)
	}
	if scan.dataScan == nil {
		return query.Constant{}, fmt.Errorf("read field %q through index select scan: table scan is nil", fieldName)
	}
	value, err := scan.dataScan.GetVal(fieldName)
	if err != nil {
		return query.Constant{}, fmt.Errorf("read field %q through index select scan: %w", fieldName, err)
	}
	return value, nil
}

// HasField reports whether the data table declares fieldName.
func (scan *IndexSelectScan) HasField(fieldName string) bool {
	if scan == nil || scan.dataScan == nil || scan.dataScan.TableScan == nil {
		return false
	}
	return scan.dataScan.HasField(fieldName)
}

// Close releases the retained index and data-table scan.
func (scan *IndexSelectScan) Close() error {
	if scan == nil {
		return fmt.Errorf("close nil index select scan")
	}
	var closeErrors []error
	if scan.index != nil {
		if err := scan.index.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close index select scan index: %w", err))
		}
	}
	if scan.dataScan != nil && scan.dataScan.TableScan != nil {
		if err := scan.dataScan.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close index select scan data table: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}
