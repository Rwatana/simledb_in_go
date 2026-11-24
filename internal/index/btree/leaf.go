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
