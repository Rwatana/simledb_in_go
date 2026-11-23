// Package indexquery adapts an index lookup to query scans.
package indexquery

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/index"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// IndexSelectScan retains the data scan, index, and key for an indexed selection.
type IndexSelectScan struct {
	dataScan  *record.TableScan
	index     index.Index
	searchKey query.Constant
}

// NewIndexSelectScan initializes an indexed selection positioned before its first result.
func NewIndexSelectScan(dataScan *record.TableScan, indexScan index.Index, searchKey query.Constant) (*IndexSelectScan, error) {
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
