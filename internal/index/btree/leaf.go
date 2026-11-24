// Package btree implements B-tree index pages.
package btree

import (
	"errors"
	"fmt"
	"math"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Leaf retains a pinned leaf page and the slot immediately before the first matching key.
type Leaf struct {
	contents    *Page
	searchKey   query.Constant
	currentSlot int
}

// NewLeaf opens a leaf page and positions immediately before the first record matching searchKey.
func NewLeaf(transaction PageTransaction, block storage.BlockID, layout *record.Layout, searchKey query.Constant) (*Leaf, error) {
	contents, err := NewPage(transaction, block, layout)
	if err != nil {
		return nil, fmt.Errorf("open B-tree leaf cursor for block %v: %w", block, err)
	}
	currentSlot, err := contents.FindSlotBefore(searchKey)
	if err != nil {
		closeErr := contents.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("position B-tree leaf cursor for %s in block %v: %w", searchKey, block, errors.Join(err, fmt.Errorf("close page: %w", closeErr)))
		}
		return nil, fmt.Errorf("position B-tree leaf cursor for %s in block %v: %w", searchKey, block, err)
	}
	return &Leaf{contents: contents, searchKey: searchKey, currentSlot: currentSlot}, nil
}

// Close releases the page pin retained by the leaf cursor.
func (leaf *Leaf) Close() error {
	if leaf == nil {
		return fmt.Errorf("close B-tree leaf cursor: leaf is nil")
	}
	if leaf.contents == nil {
		return nil
	}
	block := leaf.contents.block
	if err := leaf.contents.Close(); err != nil {
		return fmt.Errorf("close B-tree leaf cursor for %s in block %v: %w", leaf.searchKey, block, err)
	}
	leaf.contents = nil
	return nil
}

// Next advances the cursor to its next matching leaf entry, following a matching overflow page when needed.
func (leaf *Leaf) Next() (bool, error) {
	if leaf == nil {
		return false, fmt.Errorf("advance B-tree leaf cursor: leaf is nil")
	}
	if leaf.contents == nil {
		return false, fmt.Errorf("advance B-tree leaf cursor for %s: page is nil", leaf.searchKey)
	}
	nextSlot, found, err := leaf.contents.NextLeafSlot(leaf.searchKey, leaf.currentSlot)
	if err != nil {
		return false, fmt.Errorf("advance B-tree leaf cursor for %s in block %v: %w", leaf.searchKey, leaf.contents.block, err)
	}
	leaf.currentSlot = nextSlot
	if found {
		return true, nil
	}
	overflowBlock, hasOverflow, err := leaf.contents.FindOverflowBlock(leaf.searchKey)
	if err != nil {
		return false, fmt.Errorf("advance B-tree leaf cursor for %s in block %v: find overflow: %w", leaf.searchKey, leaf.contents.block, err)
	}
	if !hasOverflow {
		return false, nil
	}
	overflow, err := leaf.contents.OpenOverflowPage(overflowBlock)
	if err != nil {
		return false, fmt.Errorf("advance B-tree leaf cursor for %s to overflow block %v: %w", leaf.searchKey, overflowBlock, err)
	}
	leaf.contents = overflow
	leaf.currentSlot = 0
	return true, nil
}

// GetDataRID returns the table record identifier at the current matching leaf entry.
func (leaf *Leaf) GetDataRID() (record.RID, error) {
	if leaf == nil {
		return record.RID{}, fmt.Errorf("read data RID from B-tree leaf cursor: leaf is nil")
	}
	if leaf.contents == nil {
		return record.RID{}, fmt.Errorf("read data RID from B-tree leaf cursor for %s: page is nil", leaf.searchKey)
	}
	rid, err := leaf.contents.GetDataRID(leaf.currentSlot)
	if err != nil {
		return record.RID{}, fmt.Errorf("read data RID from B-tree leaf cursor for %s in block %v at slot %d: %w", leaf.searchKey, leaf.contents.block, leaf.currentSlot, err)
	}
	return rid, nil
}

// Delete removes the first current-search-key leaf entry with the specified data RID.
func (leaf *Leaf) Delete(dataRID record.RID) error {
	if leaf == nil {
		return fmt.Errorf("delete B-tree leaf entry %s: leaf is nil", dataRID)
	}
	if leaf.contents == nil {
		return fmt.Errorf("delete B-tree leaf entry %s for %s: page is nil", dataRID, leaf.searchKey)
	}
	for {
		found, err := leaf.Next()
		if err != nil {
			return fmt.Errorf("delete B-tree leaf entry %s for %s: advance cursor: %w", dataRID, leaf.searchKey, err)
		}
		if !found {
			return nil
		}
		currentRID, err := leaf.GetDataRID()
		if err != nil {
			return fmt.Errorf("delete B-tree leaf entry %s for %s: read current data RID: %w", dataRID, leaf.searchKey, err)
		}
		if !currentRID.Equal(dataRID) {
			continue
		}
		if err := leaf.contents.Delete(leaf.currentSlot); err != nil {
			return fmt.Errorf("delete B-tree leaf entry %s for %s in block %v at slot %d: %w", dataRID, leaf.searchKey, leaf.contents.block, leaf.currentSlot, err)
		}
		return nil
	}
}

// Insert adds dataRID with the cursor's search key and links full duplicate-key pages into an overflow chain.
func (leaf *Leaf) Insert(dataRID record.RID) (*DirEntry, error) {
	if leaf == nil {
		return nil, fmt.Errorf("insert B-tree leaf entry for %s: leaf is nil", dataRID)
	}
	if leaf.contents == nil {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s: page is nil", dataRID, leaf.searchKey)
	}
	if leaf.currentSlot < -1 {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s: current slot %d is less than -1", dataRID, leaf.searchKey, leaf.currentSlot)
	}
	maxInt := int(^uint(0) >> 1)
	if leaf.currentSlot == maxInt {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s: current slot overflows int", dataRID, leaf.searchKey)
	}
	handled, entry, err := leaf.insertBeforeFirst(dataRID)
	if err != nil {
		return nil, err
	}
	if handled {
		return entry, nil
	}
	full, err := leaf.contents.IsFull()
	if err != nil {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: check capacity: %w", dataRID, leaf.searchKey, leaf.contents.block, err)
	}
	if full {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: page has no free slot", dataRID, leaf.searchKey, leaf.contents.block)
	}
	slot := leaf.currentSlot + 1
	if err := leaf.contents.InsertLeaf(slot, leaf.searchKey, dataRID); err != nil {
		return nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v at slot %d: %w", dataRID, leaf.searchKey, leaf.contents.block, slot, err)
	}
	leaf.currentSlot = slot
	if err := leaf.splitDuplicateOverflowIfFull(dataRID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (leaf *Leaf) splitDuplicateOverflowIfFull(dataRID record.RID) error {
	page := leaf.contents
	full, err := page.IsFull()
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: check capacity after insert: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if !full {
		return nil
	}
	count, err := page.GetNumRecs()
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read record count after insert: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if count < 1 {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: record count %d is invalid after insert", dataRID, leaf.searchKey, page.block, count)
	}
	firstKey, err := page.GetDataVal(0)
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read first key after insert: %w", dataRID, leaf.searchKey, page.block, err)
	}
	lastSlot := int(count) - 1
	lastKey, err := page.GetDataVal(lastSlot)
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read last key at slot %d after insert: %w", dataRID, leaf.searchKey, page.block, lastSlot, err)
	}
	if !firstKey.Equal(lastKey) {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: full page contains mixed keys %s and %s; mixed-key split is not implemented", dataRID, leaf.searchKey, page.block, firstKey, lastKey)
	}
	flag, err := page.GetFlag()
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read overflow flag: %w", dataRID, leaf.searchKey, page.block, err)
	}
	overflowBlock, err := page.Split(1, flag)
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: split duplicate overflow page: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if int64(overflowBlock.Number()) > math.MaxInt32 {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: overflow block number %d exceeds signed 32-bit flag range", dataRID, leaf.searchKey, page.block, overflowBlock.Number())
	}
	if err := page.SetFlag(int32(overflowBlock.Number())); err != nil {
		return fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: link duplicate overflow block %d: %w", dataRID, leaf.searchKey, page.block, overflowBlock.Number(), err)
	}
	return nil
}

func (leaf *Leaf) insertBeforeFirst(dataRID record.RID) (bool, *DirEntry, error) {
	page := leaf.contents
	count, err := page.GetNumRecs()
	if err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read record count: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if count < 0 {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: record count %d is negative", dataRID, leaf.searchKey, page.block, count)
	}
	if count == 0 {
		return false, nil, nil
	}
	flag, err := page.GetFlag()
	if err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read page flag: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if flag < 0 {
		return false, nil, nil
	}
	firstKey, err := page.GetDataVal(0)
	if err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: read first key: %w", dataRID, leaf.searchKey, page.block, err)
	}
	comparison, comparable := compareBTreeKeys(firstKey, leaf.searchKey)
	if !comparable {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: first key %s is not comparable to the search key", dataRID, leaf.searchKey, page.block, firstKey)
	}
	if comparison <= 0 {
		return false, nil, nil
	}

	rightBlock, err := page.Split(0, flag)
	if err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: split old page: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if err := page.SetFlag(-1); err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v: mark left page as overflow: %w", dataRID, leaf.searchKey, page.block, err)
	}
	if err := page.InsertLeaf(0, leaf.searchKey, dataRID); err != nil {
		return true, nil, fmt.Errorf("insert B-tree leaf entry %s for %s in block %v at slot 0: %w", dataRID, leaf.searchKey, page.block, err)
	}
	leaf.currentSlot = 0
	entry := NewDirEntry(firstKey, rightBlock.Number())
	return true, &entry, nil
}
