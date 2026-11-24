package btree

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Rwatana/simledb_in_go/internal/query"
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
	leaf              *Leaf
	pendingDirectory  *Page
}

// BeforeFirst positions the index cursor immediately before entries matching searchKey.
func (index *BTreeIndex) BeforeFirst(searchKey query.Constant) error {
	if index == nil {
		return fmt.Errorf("position B-tree index before %s: index is nil", searchKey)
	}
	if isNilPageTransaction(index.transaction) {
		return fmt.Errorf("position B-tree index %q before %s: transaction is nil", index.indexName, searchKey)
	}
	if strings.TrimSpace(index.leafFileName) == "" || strings.TrimSpace(index.directoryFileName) == "" {
		return fmt.Errorf("position B-tree index %q before %s: file name is empty", index.indexName, searchKey)
	}
	if index.rootBlock.FileName() != index.directoryFileName || index.rootBlock.Number() != 0 {
		return fmt.Errorf("position B-tree index %q before %s: root block %v is not block zero of %q", index.indexName, searchKey, index.rootBlock, index.directoryFileName)
	}
	if _, err := leafPageFields(index.leafLayout); err != nil {
		return fmt.Errorf("position B-tree index %q before %s: invalid leaf layout: %w", index.indexName, searchKey, err)
	}
	directoryKeyType, err := directoryPageKeyType(index.directoryLayout)
	if err != nil {
		return fmt.Errorf("position B-tree index %q before %s: invalid directory layout: %w", index.indexName, searchKey, err)
	}
	leafKeyType, ok := index.leafLayout.Schema().TypeOf("dataval")
	if !ok || leafKeyType != directoryKeyType {
		return fmt.Errorf("position B-tree index %q before %s: leaf and directory key types differ", index.indexName, searchKey)
	}
	if _, ok := searchKey.HashCode(); !ok {
		return fmt.Errorf("position B-tree index %q: search key is unset", index.indexName)
	}
	switch directoryKeyType {
	case record.IntegerField:
		if _, ok := searchKey.AsInt(); !ok {
			return fmt.Errorf("position B-tree index %q: integer dataval cannot be compared with %s", index.indexName, searchKey)
		}
	case record.VarcharField:
		if _, ok := searchKey.AsString(); !ok {
			return fmt.Errorf("position B-tree index %q: varchar dataval cannot be compared with %s", index.indexName, searchKey)
		}
	}

	if index.leaf != nil {
		if err := index.leaf.Close(); err != nil {
			return fmt.Errorf("position B-tree index %q before %s: close current leaf: %w", index.indexName, searchKey, err)
		}
		index.leaf = nil
	}
	if index.pendingDirectory != nil {
		if err := index.pendingDirectory.Close(); err != nil {
			return fmt.Errorf("position B-tree index %q before %s: close pending directory page: %w", index.indexName, searchKey, err)
		}
		index.pendingDirectory = nil
	}

	directoryPage, err := NewPage(index.transaction, index.rootBlock, index.directoryLayout)
	if err != nil {
		return fmt.Errorf("position B-tree index %q before %s: open root directory: %w", index.indexName, searchKey, err)
	}
	leafBlock, err := directoryPage.FindLeafBlock(searchKey)
	if err != nil {
		closeErr := index.closeDirectoryAfterFailure(directoryPage)
		if closeErr != nil {
			return fmt.Errorf("position B-tree index %q before %s: %w", index.indexName, searchKey, errors.Join(err, fmt.Errorf("close directory page: %w", closeErr)))
		}
		return fmt.Errorf("position B-tree index %q before %s: search directory: %w", index.indexName, searchKey, err)
	}
	leafBlock = storage.NewBlockID(index.leafFileName, leafBlock.Number())
	leafPage, err := directoryPage.OpenLeafPage(leafBlock, index.leafLayout)
	if err != nil {
		if directoryPage.block.FileName() != "" {
			if closeErr := index.closeDirectoryAfterFailure(directoryPage); closeErr != nil {
				return fmt.Errorf("position B-tree index %q before %s: %w", index.indexName, searchKey, errors.Join(err, fmt.Errorf("close directory page: %w", closeErr)))
			}
		}
		return fmt.Errorf("position B-tree index %q before %s: open selected leaf: %w", index.indexName, searchKey, err)
	}
	currentSlot, err := leafPage.FindSlotBefore(searchKey)
	if err != nil {
		closeErr := leafPage.Close()
		if closeErr != nil {
			index.leaf = &Leaf{contents: leafPage, searchKey: searchKey, currentSlot: -1}
			return fmt.Errorf("position B-tree index %q before %s: %w", index.indexName, searchKey, errors.Join(err, fmt.Errorf("close selected leaf page: %w", closeErr)))
		}
		return fmt.Errorf("position B-tree index %q before %s: position selected leaf: %w", index.indexName, searchKey, err)
	}
	index.leaf = &Leaf{contents: leafPage, searchKey: searchKey, currentSlot: currentSlot}
	return nil
}

func (index *BTreeIndex) closeDirectoryAfterFailure(page *Page) error {
	if err := page.Close(); err != nil {
		index.pendingDirectory = page
		return err
	}
	return nil
}

// Next advances the positioned B-tree index cursor to its next matching entry.
func (index *BTreeIndex) Next() (bool, error) {
	if index == nil {
		return false, fmt.Errorf("advance B-tree index: index is nil")
	}
	if index.leaf == nil {
		return false, fmt.Errorf("advance B-tree index %q: cursor is not positioned", index.indexName)
	}
	found, err := index.leaf.Next()
	if err != nil {
		return false, fmt.Errorf("advance B-tree index %q: %w", index.indexName, err)
	}
	return found, nil
}

// GetDataRID returns the table record identifier at the current matching index entry.
func (index *BTreeIndex) GetDataRID() (record.RID, error) {
	if index == nil {
		return record.RID{}, fmt.Errorf("read current B-tree index RID: index is nil")
	}
	if index.leaf == nil {
		return record.RID{}, fmt.Errorf("read current B-tree index RID for %q: cursor is not positioned", index.indexName)
	}
	rid, err := index.leaf.GetDataRID()
	if err != nil {
		return record.RID{}, fmt.Errorf("read current B-tree index RID for %q: %w", index.indexName, err)
	}
	return rid, nil
}

// Close releases the pages retained by the B-tree index cursor.
func (index *BTreeIndex) Close() error {
	if index == nil {
		return fmt.Errorf("close B-tree index: index is nil")
	}
	var closeErrors []error
	if index.leaf != nil {
		if err := index.leaf.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close B-tree index %q leaf cursor: %w", index.indexName, err))
		} else {
			index.leaf = nil
		}
	}
	if index.pendingDirectory != nil {
		if err := index.pendingDirectory.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close B-tree index %q pending directory page: %w", index.indexName, err))
		} else {
			index.pendingDirectory = nil
		}
	}
	return errors.Join(closeErrors...)
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
