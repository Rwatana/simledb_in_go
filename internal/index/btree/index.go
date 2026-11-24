package btree

import (
	"fmt"
	"strings"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// BTreeIndex retains one index's transaction, file names, record layouts, and root block.
type BTreeIndex struct {
	transaction       btreeFileTransaction
	indexName         string
	leafFileName      string
	leafLayout        *record.Layout
	directoryFileName string
	directoryLayout   *record.Layout
	rootBlock         storage.BlockID
}

// NewBTreeIndex initializes the files and retained state for one B-tree index.
// The caller must roll back the transaction if construction fails after an append.
func NewBTreeIndex(transaction btreeFileTransaction, indexName string, leafLayout *record.Layout) (*BTreeIndex, error) {
	if isNilPageTransaction(transaction) {
		return nil, fmt.Errorf("initialize B-tree index %q: transaction is nil", indexName)
	}
	if strings.TrimSpace(indexName) == "" {
		return nil, fmt.Errorf("initialize B-tree index: index name is empty")
	}
	directoryLayout, err := directoryLayoutFromLeaf(leafLayout)
	if err != nil {
		return nil, fmt.Errorf("initialize B-tree index %q: derive directory layout: %w", indexName, err)
	}
	leafFileName := indexName + "leaf"
	if err := ensureLeafFile(transaction, leafFileName, leafLayout); err != nil {
		return nil, fmt.Errorf("initialize B-tree index %q: initialize leaf file: %w", indexName, err)
	}
	directoryFileName := indexName + "dir"
	if err := ensureDirectoryFile(transaction, directoryFileName, directoryLayout); err != nil {
		return nil, fmt.Errorf("initialize B-tree index %q: initialize directory file: %w", indexName, err)
	}
	return &BTreeIndex{
		transaction:       transaction,
		indexName:         indexName,
		leafFileName:      leafFileName,
		leafLayout:        leafLayout,
		directoryFileName: directoryFileName,
		directoryLayout:   directoryLayout,
		rootBlock:         storage.NewBlockID(directoryFileName, 0),
	}, nil
}
