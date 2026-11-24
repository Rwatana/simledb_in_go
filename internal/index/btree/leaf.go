// Package btree implements B-tree index pages.
package btree

import (
	"errors"
	"fmt"

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
