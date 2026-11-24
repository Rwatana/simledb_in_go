// Package btree implements B-tree index pages.
package btree

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// PageTransaction provides the record-page operations and pin release used by a B-tree page.
type PageTransaction interface {
	record.RecordTransaction
	Unpin(storage.BlockID) error
}

type pageBlockAppender interface {
	Append(string) (storage.BlockID, error)
}

// Page retains one pinned B-tree block and the layout used to interpret its records.
type Page struct {
	transaction PageTransaction
	block       storage.BlockID
	layout      *record.Layout
}

// NewPage pins block and retains its transaction and record layout.
func NewPage(transaction PageTransaction, block storage.BlockID, layout *record.Layout) (*Page, error) {
	if isNilPageTransaction(transaction) {
		return nil, fmt.Errorf("open B-tree page for block %v: transaction is nil", block)
	}
	if layout == nil {
		return nil, fmt.Errorf("open B-tree page for block %v: layout is nil", block)
	}
	if layout.Schema() == nil {
		return nil, fmt.Errorf("open B-tree page for block %v: layout schema is nil", block)
	}
	if block.FileName() == "" {
		return nil, fmt.Errorf("open B-tree page: block file name is empty")
	}
	if block.Number() < 0 {
		return nil, fmt.Errorf("open B-tree page for file %q: block number %d is negative", block.FileName(), block.Number())
	}
	if err := transaction.Pin(block); err != nil {
		return nil, fmt.Errorf("open B-tree page for block %v: pin: %w", block, err)
	}
	return &Page{transaction: transaction, block: block, layout: layout}, nil
}

// FindSlotBefore returns the slot immediately before the first dataval not less than searchKey.
func (page *Page) FindSlotBefore(searchKey query.Constant) (int, error) {
	slot, _, err := page.findSlotBeforeWithCount(searchKey)
	return slot, err
}

func (page *Page) findSlotBeforeWithCount(searchKey query.Constant) (int, int32, error) {
	if page == nil {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s: page is nil", searchKey)
	}
	if page.block.FileName() == "" {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s: page is closed", searchKey)
	}
	if isNilPageTransaction(page.transaction) {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: transaction is nil", searchKey, page.block)
	}
	if page.layout == nil {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: layout is nil", searchKey, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: layout schema is nil", searchKey, page.block)
	}
	dataval, ok := schema.Field("dataval")
	if !ok {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: layout has no dataval field", searchKey, page.block)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: layout has no dataval offset", searchKey, page.block)
	}
	slotSize := page.layout.SlotSize()
	if slotSize <= 0 || fieldOffset < 0 || fieldOffset >= slotSize {
		return -1, 0, fmt.Errorf("find B-tree page slot before %s in block %v: invalid dataval offset %d for slot size %d", searchKey, page.block, fieldOffset, slotSize)
	}
	if _, ok := searchKey.HashCode(); !ok {
		return -1, 0, fmt.Errorf("find B-tree page slot in block %v: search key is unset", page.block)
	}
	switch dataval.Type {
	case record.IntegerField:
		if _, ok := searchKey.AsInt(); !ok {
			return -1, 0, fmt.Errorf("find B-tree page slot in block %v: integer dataval cannot be compared with %s", page.block, searchKey)
		}
	case record.VarcharField:
		if _, ok := searchKey.AsString(); !ok {
			return -1, 0, fmt.Errorf("find B-tree page slot in block %v: varchar dataval cannot be compared with %s", page.block, searchKey)
		}
	default:
		return -1, 0, fmt.Errorf("find B-tree page slot in block %v: unsupported dataval type %d", page.block, dataval.Type)
	}

	countValue, err := page.transaction.GetInt(page.block, 4)
	if err != nil {
		return -1, 0, fmt.Errorf("find B-tree page slot in block %v: read record count: %w", page.block, err)
	}
	if countValue < 0 {
		return -1, 0, fmt.Errorf("find B-tree page slot in block %v: record count %d is negative", page.block, countValue)
	}
	for slot := 0; slot < int(countValue); slot++ {
		position, err := pageSlotFieldPosition(slot, slotSize, fieldOffset)
		if err != nil {
			return -1, 0, fmt.Errorf("find B-tree page slot %d in block %v: %w", slot, page.block, err)
		}
		var value query.Constant
		switch dataval.Type {
		case record.IntegerField:
			integer, err := page.transaction.GetInt(page.block, position)
			if err != nil {
				return -1, 0, fmt.Errorf("find B-tree page slot %d in block %v: read integer dataval: %w", slot, page.block, err)
			}
			value = query.NewIntConstant(integer)
		case record.VarcharField:
			text, err := page.transaction.GetString(page.block, position)
			if err != nil {
				return -1, 0, fmt.Errorf("find B-tree page slot %d in block %v: read varchar dataval: %w", slot, page.block, err)
			}
			value = query.NewStringConstant(text)
		}
		comparison := 0
		if dataval.Type == record.IntegerField {
			valueInt, _ := value.AsInt()
			searchInt, _ := searchKey.AsInt()
			comparison = cmp.Compare(valueInt, searchInt)
		} else {
			valueText, _ := value.AsString()
			searchText, _ := searchKey.AsString()
			comparison = slices.Compare(utf16.Encode([]rune(valueText)), utf16.Encode([]rune(searchText)))
		}
		if comparison >= 0 {
			return slot - 1, countValue, nil
		}
	}
	return int(countValue) - 1, countValue, nil
}

// FindChildBlock selects the directory child whose key is the greatest key not greater than searchKey.
func (page *Page) FindChildBlock(searchKey query.Constant) (storage.BlockID, error) {
	if page == nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s: page is nil", searchKey)
	}
	if page.block.FileName() == "" {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s: page is closed", searchKey)
	}
	if isNilPageTransaction(page.transaction) {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: transaction is nil", searchKey, page.block)
	}
	if page.layout == nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: layout is nil", searchKey, page.block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: %w", searchKey, page.block, err)
	}
	datavalFound := false
	blockFound := false
	for _, field := range fields {
		switch field.name {
		case "dataval":
			datavalFound = true
		case "block":
			blockFound = true
			if field.typeOf != record.IntegerField {
				return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: block field is not an integer", searchKey, page.block)
			}
		}
	}
	if !datavalFound {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: layout has no dataval field", searchKey, page.block)
	}
	if !blockFound {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: layout has no block field", searchKey, page.block)
	}

	slotBefore, count, err := page.findSlotBeforeWithCount(searchKey)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: %w", searchKey, page.block, err)
	}
	if count == 0 {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: directory page is empty", searchKey, page.block)
	}
	selected := slotBefore
	nextSlot := slotBefore + 1
	if nextSlot < int(count) {
		key, err := page.GetDataVal(nextSlot)
		if err != nil {
			return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: read candidate key at slot %d: %w", searchKey, page.block, nextSlot, err)
		}
		if key.Equal(searchKey) {
			selected = nextSlot
		}
	}
	if selected < 0 {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: key is smaller than the first directory key", searchKey, page.block)
	}
	childNumber, err := page.GetChildNum(selected)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: read child at slot %d: %w", searchKey, page.block, selected, err)
	}
	if childNumber < 0 {
		return storage.BlockID{}, fmt.Errorf("find B-tree child block for %s in block %v: child block number %d is negative", searchKey, page.block, childNumber)
	}
	return storage.NewBlockID(page.block.FileName(), int(childNumber)), nil
}

// OpenChildDirectory opens the selected child page when this page is above the leaf level.
func (page *Page) OpenChildDirectory(searchKey query.Constant) (*Page, error) {
	if page == nil {
		return nil, fmt.Errorf("open B-tree child directory for %s: page is nil", searchKey)
	}
	level, err := page.GetFlag()
	if err != nil {
		return nil, fmt.Errorf("open B-tree child directory for %s in block %v: read page level: %w", searchKey, page.block, err)
	}
	if level <= 0 {
		return nil, fmt.Errorf("open B-tree child directory for %s in block %v: page level %d has no directory children", searchKey, page.block, level)
	}
	childBlock, err := page.FindChildBlock(searchKey)
	if err != nil {
		return nil, fmt.Errorf("open B-tree child directory for %s in block %v: select child: %w", searchKey, page.block, err)
	}
	child, err := NewPage(page.transaction, childBlock, page.layout)
	if err != nil {
		return nil, fmt.Errorf("open B-tree child directory block %v for %s: %w", childBlock, searchKey, err)
	}
	return child, nil
}

// FindLeafBlock descends the selected directory path and returns its leaf block.
func (page *Page) FindLeafBlock(searchKey query.Constant) (storage.BlockID, error) {
	if page == nil {
		return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s: page is nil", searchKey)
	}
	for {
		if page.block.FileName() == "" {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s: page is closed", searchKey)
		}
		level, err := page.GetFlag()
		if err != nil {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s in block %v: read page level: %w", searchKey, page.block, err)
		}
		if level < 0 {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s in block %v: page level %d is negative", searchKey, page.block, level)
		}
		childBlock, err := page.FindChildBlock(searchKey)
		if err != nil {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s in block %v: select child: %w", searchKey, page.block, err)
		}
		if level == 0 {
			return childBlock, nil
		}
		if err := page.Close(); err != nil {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s in block %v: close directory page: %w", searchKey, page.block, err)
		}
		child, err := NewPage(page.transaction, childBlock, page.layout)
		if err != nil {
			return storage.BlockID{}, fmt.Errorf("find B-tree leaf block for %s: open child directory block %v: %w", searchKey, childBlock, err)
		}
		*page = *child
	}
}

// OpenLeafPage replaces an open level-zero directory page with a pinned leaf page.
func (page *Page) OpenLeafPage(leafBlock storage.BlockID, leafLayout *record.Layout) (*Page, error) {
	if page == nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory page is nil", leafBlock)
	}
	if page.block.FileName() == "" {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory page is closed", leafBlock)
	}
	if isNilPageTransaction(page.transaction) {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory transaction is nil", leafBlock)
	}
	if page.layout == nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory layout is nil", leafBlock)
	}
	if page.layout.Schema() == nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory layout schema is nil", leafBlock)
	}
	if leafBlock.FileName() == "" {
		return nil, fmt.Errorf("open B-tree leaf page: leaf block file name is empty")
	}
	if leafBlock.Number() < 0 {
		return nil, fmt.Errorf("open B-tree leaf page for file %q: leaf block number %d is negative", leafBlock.FileName(), leafBlock.Number())
	}
	if leafLayout == nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: leaf layout is nil", leafBlock)
	}
	if leafLayout.Schema() == nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: leaf layout schema is nil", leafBlock)
	}
	level, err := page.GetFlag()
	if err != nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: read directory level: %w", leafBlock, err)
	}
	if level != 0 {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: directory level is %d, want zero", leafBlock, level)
	}
	directoryBlock := page.block
	if err := page.Close(); err != nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: close directory block %v: %w", leafBlock, directoryBlock, err)
	}
	leaf, err := NewPage(page.transaction, leafBlock, leafLayout)
	if err != nil {
		return nil, fmt.Errorf("open B-tree leaf page for block %v: %w", leafBlock, err)
	}
	return leaf, nil
}

// NextLeafSlot advances one leaf-page slot and reports whether its value matches searchKey.
func (page *Page) NextLeafSlot(searchKey query.Constant, currentSlot int) (int, bool, error) {
	if page == nil {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s: page is nil", searchKey)
	}
	if page.block.FileName() == "" {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s: page is closed", searchKey)
	}
	if isNilPageTransaction(page.transaction) {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: transaction is nil", searchKey, page.block)
	}
	if page.layout == nil {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: layout is nil", searchKey, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: layout schema is nil", searchKey, page.block)
	}
	dataval, ok := schema.Field("dataval")
	if !ok {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: layout has no dataval field", searchKey, page.block)
	}
	if dataval.Type != record.IntegerField && dataval.Type != record.VarcharField {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: unsupported dataval type %d", searchKey, page.block, dataval.Type)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: layout has no dataval offset", searchKey, page.block)
	}
	slotSize := page.layout.SlotSize()
	if slotSize <= 0 || fieldOffset < 0 || fieldOffset >= slotSize {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: invalid dataval offset %d for slot size %d", searchKey, page.block, fieldOffset, slotSize)
	}
	if currentSlot < -1 {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: current slot %d is less than -1", searchKey, page.block, currentSlot)
	}
	maxInt := int(^uint(0) >> 1)
	if currentSlot == maxInt {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: current slot overflows int", searchKey, page.block)
	}
	nextSlot := currentSlot + 1
	count, err := page.GetNumRecs()
	if err != nil {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: read record count: %w", searchKey, page.block, err)
	}
	if count < 0 {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v: record count %d is negative", searchKey, page.block, count)
	}
	if nextSlot >= int(count) {
		return nextSlot, false, nil
	}
	value, err := page.GetDataVal(nextSlot)
	if err != nil {
		return -1, false, fmt.Errorf("find next B-tree leaf slot for %s in block %v at slot %d: read data value: %w", searchKey, page.block, nextSlot, err)
	}
	return nextSlot, value.Equal(searchKey), nil
}

// FindOverflowBlock returns the leaf overflow block when its first key matches searchKey.
func (page *Page) FindOverflowBlock(searchKey query.Constant) (storage.BlockID, bool, error) {
	if page == nil {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s: page is nil", searchKey)
	}
	if page.block.FileName() == "" {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s: page is closed", searchKey)
	}
	if isNilPageTransaction(page.transaction) {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: transaction is nil", searchKey, page.block)
	}
	if page.layout == nil {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: layout is nil", searchKey, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: layout schema is nil", searchKey, page.block)
	}
	dataval, ok := schema.Field("dataval")
	if !ok {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: layout has no dataval field", searchKey, page.block)
	}
	if dataval.Type != record.IntegerField && dataval.Type != record.VarcharField {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: unsupported dataval type %d", searchKey, page.block, dataval.Type)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: layout has no dataval offset", searchKey, page.block)
	}
	slotSize := page.layout.SlotSize()
	if slotSize <= 0 || fieldOffset < 0 || fieldOffset >= slotSize {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: invalid dataval offset %d for slot size %d", searchKey, page.block, fieldOffset, slotSize)
	}
	switch dataval.Type {
	case record.IntegerField:
		if _, ok := searchKey.AsInt(); !ok {
			return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block in block %v: integer dataval cannot be compared with %s", page.block, searchKey)
		}
	case record.VarcharField:
		if _, ok := searchKey.AsString(); !ok {
			return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block in block %v: varchar dataval cannot be compared with %s", page.block, searchKey)
		}
	}
	firstKey, err := page.GetDataVal(0)
	if err != nil {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: read first key: %w", searchKey, page.block, err)
	}
	flag, err := page.GetFlag()
	if err != nil {
		return storage.BlockID{}, false, fmt.Errorf("find B-tree overflow block for %s in block %v: read overflow flag: %w", searchKey, page.block, err)
	}
	if !firstKey.Equal(searchKey) || flag < 0 {
		return storage.BlockID{}, false, nil
	}
	return storage.NewBlockID(page.block.FileName(), int(flag)), true, nil
}

// OpenOverflowPage releases the current leaf page before pinning its selected overflow page.
func (page *Page) OpenOverflowPage(overflowBlock storage.BlockID) (*Page, error) {
	if page == nil {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: current page is nil", overflowBlock)
	}
	if page.block.FileName() == "" {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: current page is closed", overflowBlock)
	}
	if isNilPageTransaction(page.transaction) {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: current transaction is nil", overflowBlock)
	}
	if page.layout == nil {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: leaf layout is nil", overflowBlock)
	}
	if page.layout.Schema() == nil {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: leaf layout schema is nil", overflowBlock)
	}
	if overflowBlock.FileName() == "" {
		return nil, fmt.Errorf("open B-tree overflow page: target file name is empty")
	}
	if overflowBlock.FileName() != page.block.FileName() {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: target file differs from current leaf file %q", overflowBlock, page.block.FileName())
	}
	if overflowBlock.Number() < 0 {
		return nil, fmt.Errorf("open B-tree overflow page for file %q: target block number %d is negative", overflowBlock.FileName(), overflowBlock.Number())
	}
	if err := page.Close(); err != nil {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: close current leaf page: %w", overflowBlock, err)
	}
	overflow, err := NewPage(page.transaction, overflowBlock, page.layout)
	if err != nil {
		return nil, fmt.Errorf("open B-tree overflow page for block %v: %w", overflowBlock, err)
	}
	return overflow, nil
}

// GetDataVal reads the dataval field from the specified B-tree page slot.
func (page *Page) GetDataVal(slot int) (query.Constant, error) {
	if page == nil {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: layout is nil", slot, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: layout schema is nil", slot, page.block)
	}
	definition, ok := schema.Field("dataval")
	if !ok {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: layout has no dataval field", slot, page.block)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: layout has no dataval offset", slot, page.block)
	}
	switch definition.Type {
	case record.IntegerField, record.VarcharField:
	default:
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: unsupported dataval type %d", slot, page.block, definition.Type)
	}
	position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
	if err != nil {
		return query.Constant{}, fmt.Errorf("read B-tree page data value at slot %d in block %v: %w", slot, page.block, err)
	}
	if definition.Type == record.IntegerField {
		value, err := page.transaction.GetInt(page.block, position)
		if err != nil {
			return query.Constant{}, fmt.Errorf("read integer dataval at slot %d in block %v: %w", slot, page.block, err)
		}
		return query.NewIntConstant(value), nil
	}
	value, err := page.transaction.GetString(page.block, position)
	if err != nil {
		return query.Constant{}, fmt.Errorf("read varchar dataval at slot %d in block %v: %w", slot, page.block, err)
	}
	return query.NewStringConstant(value), nil
}

// SetDataVal writes the typed dataval field in the specified B-tree page slot.
func (page *Page) SetDataVal(slot int, value query.Constant) error {
	if page == nil {
		return fmt.Errorf("write B-tree page data value at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("write B-tree page data value at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: layout is nil", slot, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: layout schema is nil", slot, page.block)
	}
	definition, ok := schema.Field("dataval")
	if !ok {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: layout has no dataval field", slot, page.block)
	}
	var integer int32
	var text string
	switch definition.Type {
	case record.IntegerField:
		integer, ok = value.AsInt()
		if !ok {
			return fmt.Errorf("write B-tree page data value at slot %d in block %v: integer dataval cannot store %s", slot, page.block, value)
		}
	case record.VarcharField:
		text, ok = value.AsString()
		if !ok {
			return fmt.Errorf("write B-tree page data value at slot %d in block %v: varchar dataval cannot store %s", slot, page.block, value)
		}
	default:
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: unsupported dataval type %d", slot, page.block, definition.Type)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: layout has no dataval offset", slot, page.block)
	}
	position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
	if err != nil {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: %w", slot, page.block, err)
	}
	if position > math.MaxInt32 {
		return fmt.Errorf("write B-tree page data value at slot %d in block %v: offset %d exceeds int32 range", slot, page.block, position)
	}
	if definition.Type == record.IntegerField {
		if err := page.transaction.SetInt(page.block, int32(position), integer, true); err != nil {
			return fmt.Errorf("write integer dataval at slot %d in block %v at offset %d: %w", slot, page.block, position, err)
		}
		return nil
	}
	if err := page.transaction.SetString(page.block, int32(position), text, true); err != nil {
		return fmt.Errorf("write varchar dataval at slot %d in block %v at offset %d: %w", slot, page.block, position, err)
	}
	return nil
}

// GetFlag reads the B-tree page's flag field from the page header.
func (page *Page) GetFlag() (int32, error) {
	if page == nil {
		return 0, fmt.Errorf("read B-tree page flag: page is nil")
	}
	if page.block.FileName() == "" {
		return 0, fmt.Errorf("read B-tree page flag: page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return 0, fmt.Errorf("read B-tree page flag in block %v: transaction is nil", page.block)
	}
	flag, err := page.transaction.GetInt(page.block, 0)
	if err != nil {
		return 0, fmt.Errorf("read B-tree page flag in block %v: %w", page.block, err)
	}
	return flag, nil
}

// SetFlag writes the B-tree page's flag field in the page header.
func (page *Page) SetFlag(value int32) error {
	if page == nil {
		return fmt.Errorf("write B-tree page flag: page is nil")
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("write B-tree page flag: page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("write B-tree page flag in block %v: transaction is nil", page.block)
	}
	if err := page.transaction.SetInt(page.block, 0, value, true); err != nil {
		return fmt.Errorf("write B-tree page flag in block %v: %w", page.block, err)
	}
	return nil
}

// GetNumRecs reads the B-tree page's signed record count from the page header.
func (page *Page) GetNumRecs() (int32, error) {
	if page == nil {
		return 0, fmt.Errorf("read B-tree page record count: page is nil")
	}
	if page.block.FileName() == "" {
		return 0, fmt.Errorf("read B-tree page record count: page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return 0, fmt.Errorf("read B-tree page record count in block %v: transaction is nil", page.block)
	}
	count, err := page.transaction.GetInt(page.block, 4)
	if err != nil {
		return 0, fmt.Errorf("read B-tree page record count in block %v: %w", page.block, err)
	}
	return count, nil
}

func (page *Page) setNumRecs(count int32) error {
	if page == nil {
		return fmt.Errorf("write B-tree page record count: page is nil")
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("write B-tree page record count: page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("write B-tree page record count in block %v: transaction is nil", page.block)
	}
	if err := page.transaction.SetInt(page.block, 4, count, true); err != nil {
		return fmt.Errorf("write B-tree page record count in block %v: %w", page.block, err)
	}
	return nil
}

// GetChildNum reads the child block number from the specified B-tree directory slot.
func (page *Page) GetChildNum(slot int) (int32, error) {
	if page == nil {
		return 0, fmt.Errorf("read B-tree page child number at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return 0, fmt.Errorf("read B-tree page child number at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: layout is nil", slot, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: layout schema is nil", slot, page.block)
	}
	definition, ok := schema.Field("block")
	if !ok {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: layout has no block field", slot, page.block)
	}
	if definition.Type != record.IntegerField {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: block field is not an integer", slot, page.block)
	}
	fieldOffset, ok := page.layout.Offset("block")
	if !ok {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: layout has no block offset", slot, page.block)
	}
	position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
	if err != nil {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: %w", slot, page.block, err)
	}
	child, err := page.transaction.GetInt(page.block, position)
	if err != nil {
		return 0, fmt.Errorf("read B-tree page child number at slot %d in block %v: %w", slot, page.block, err)
	}
	return child, nil
}

// SetChildNum writes the child block number to the specified B-tree directory slot.
func (page *Page) SetChildNum(slot int, childBlock int32) error {
	if page == nil {
		return fmt.Errorf("write B-tree page child number at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("write B-tree page child number at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: layout is nil", slot, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: layout schema is nil", slot, page.block)
	}
	definition, ok := schema.Field("block")
	if !ok {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: layout has no block field", slot, page.block)
	}
	if definition.Type != record.IntegerField {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: block field is not an integer", slot, page.block)
	}
	fieldOffset, ok := page.layout.Offset("block")
	if !ok {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: layout has no block offset", slot, page.block)
	}
	position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
	if err != nil {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: %w", slot, page.block, err)
	}
	if position > math.MaxInt32 {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v: offset %d exceeds int32 range", slot, page.block, position)
	}
	if err := page.transaction.SetInt(page.block, int32(position), childBlock, true); err != nil {
		return fmt.Errorf("write B-tree page child number at slot %d in block %v at offset %d: %w", slot, page.block, position, err)
	}
	return nil
}

// GetDataRID reads the data record identifier stored in the specified leaf slot.
func (page *Page) GetDataRID(slot int) (record.RID, error) {
	blockNumber, err := page.GetChildNum(slot)
	if err != nil {
		return record.RID{}, fmt.Errorf("read B-tree page data RID at slot %d: %w", slot, err)
	}
	schema := page.layout.Schema()
	definition, ok := schema.Field("id")
	if !ok {
		return record.RID{}, fmt.Errorf("read B-tree page data RID at slot %d in block %v: layout has no id field", slot, page.block)
	}
	if definition.Type != record.IntegerField {
		return record.RID{}, fmt.Errorf("read B-tree page data RID at slot %d in block %v: id field is not an integer", slot, page.block)
	}
	fieldOffset, ok := page.layout.Offset("id")
	if !ok {
		return record.RID{}, fmt.Errorf("read B-tree page data RID at slot %d in block %v: layout has no id offset", slot, page.block)
	}
	position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
	if err != nil {
		return record.RID{}, fmt.Errorf("read B-tree page data RID id at slot %d in block %v: %w", slot, page.block, err)
	}
	id, err := page.transaction.GetInt(page.block, position)
	if err != nil {
		return record.RID{}, fmt.Errorf("read B-tree page data RID id at slot %d in block %v: %w", slot, page.block, err)
	}
	return record.NewRID(int(blockNumber), int(id)), nil
}

// SetDataRID writes the table record identifier to the specified B-tree leaf slot.
func (page *Page) SetDataRID(slot int, rid record.RID) error {
	if page == nil {
		return fmt.Errorf("write B-tree page data RID at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("write B-tree page data RID at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("write B-tree page data RID at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return fmt.Errorf("write B-tree page data RID at slot %d in block %v: layout is nil", slot, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return fmt.Errorf("write B-tree page data RID at slot %d in block %v: layout schema is nil", slot, page.block)
	}

	fields := [...]string{"block", "id"}
	positions := [2]int{}
	for i, field := range fields {
		definition, ok := schema.Field(field)
		if !ok {
			return fmt.Errorf("write B-tree page data RID at slot %d in block %v: layout has no %s field", slot, page.block, field)
		}
		if definition.Type != record.IntegerField {
			return fmt.Errorf("write B-tree page data RID at slot %d in block %v: %s field is not an integer", slot, page.block, field)
		}
		fieldOffset, ok := page.layout.Offset(field)
		if !ok {
			return fmt.Errorf("write B-tree page data RID at slot %d in block %v: layout has no %s offset", slot, page.block, field)
		}
		position, err := pageSlotFieldPosition(slot, page.layout.SlotSize(), fieldOffset)
		if err != nil {
			return fmt.Errorf("write B-tree page data RID %s at slot %d in block %v: %w", field, slot, page.block, err)
		}
		if position > math.MaxInt32 {
			return fmt.Errorf("write B-tree page data RID %s at slot %d in block %v: offset %d exceeds int32 range", field, slot, page.block, position)
		}
		positions[i] = position
	}

	values := [...]int{rid.BlockNumber(), rid.Slot()}
	for i, value := range values {
		if int64(value) < math.MinInt32 || int64(value) > math.MaxInt32 {
			return fmt.Errorf("write B-tree page data RID %s at slot %d in block %v: value %d exceeds int32 range", fields[i], slot, page.block, value)
		}
	}
	for i, value := range values {
		if err := page.transaction.SetInt(page.block, int32(positions[i]), int32(value), true); err != nil {
			return fmt.Errorf("write B-tree page data RID %s at slot %d in block %v at offset %d: %w", fields[i], slot, page.block, positions[i], err)
		}
	}
	return nil
}

// InsertDir opens a directory slot and stores its key and child block number.
func (page *Page) InsertDir(slot int, value query.Constant, childBlock int32) error {
	count, err := page.insertSlot(slot, value)
	if err != nil {
		return fmt.Errorf("insert B-tree directory entry at slot %d: %w", slot, err)
	}
	if err := page.setNumRecs(count + 1); err != nil {
		return fmt.Errorf("insert B-tree directory entry at slot %d in block %v: write record count: %w", slot, page.block, err)
	}
	if err := page.SetDataVal(slot, value); err != nil {
		return fmt.Errorf("insert B-tree directory entry at slot %d in block %v: write dataval: %w", slot, page.block, err)
	}
	if err := page.SetChildNum(slot, childBlock); err != nil {
		return fmt.Errorf("insert B-tree directory entry at slot %d in block %v: write child number: %w", slot, page.block, err)
	}
	return nil
}

// InsertLeaf opens a leaf slot, shifts later entries, and stores its key and record identifier.
func (page *Page) InsertLeaf(slot int, value query.Constant, rid record.RID) error {
	if page == nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("insert B-tree leaf entry at slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: layout is nil", slot, page.block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: invalid layout: %w", slot, page.block, err)
	}
	hasBlock, hasID := false, false
	for _, field := range fields {
		switch field.name {
		case "block":
			hasBlock = true
			if field.typeOf != record.IntegerField {
				return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: block field is not an integer", slot, page.block)
			}
		case "id":
			hasID = true
			if field.typeOf != record.IntegerField {
				return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: id field is not an integer", slot, page.block)
			}
		}
	}
	if !hasBlock {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: layout has no block field", slot, page.block)
	}
	if !hasID {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: layout has no id field", slot, page.block)
	}
	values := [...]int{rid.BlockNumber(), rid.Slot()}
	fieldNames := [...]string{"block", "id"}
	for i, field := range fieldNames {
		if int64(values[i]) < math.MinInt32 || int64(values[i]) > math.MaxInt32 {
			return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: RID %s value %d exceeds int32 range", slot, page.block, field, values[i])
		}
	}

	count, err := page.insertSlot(slot, value)
	if err != nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: open slot: %w", slot, page.block, err)
	}
	if err := page.setNumRecs(count + 1); err != nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: write record count: %w", slot, page.block, err)
	}
	if err := page.SetDataVal(slot, value); err != nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: write dataval: %w", slot, page.block, err)
	}
	if err := page.SetDataRID(slot, rid); err != nil {
		return fmt.Errorf("insert B-tree leaf entry at slot %d in block %v: write data RID: %w", slot, page.block, err)
	}
	return nil
}

// Delete removes the record at slot and shifts every later page record left.
func (page *Page) Delete(slot int) error {
	if page == nil {
		return fmt.Errorf("delete B-tree page slot %d: page is nil", slot)
	}
	if page.block.FileName() == "" {
		return fmt.Errorf("delete B-tree page slot %d: page is closed", slot)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("delete B-tree page slot %d in block %v: transaction is nil", slot, page.block)
	}
	if page.layout == nil {
		return fmt.Errorf("delete B-tree page slot %d in block %v: layout is nil", slot, page.block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return fmt.Errorf("delete B-tree page slot %d in block %v: invalid layout: %w", slot, page.block, err)
	}
	count, err := page.GetNumRecs()
	if err != nil {
		return fmt.Errorf("delete B-tree page slot %d in block %v: read record count: %w", slot, page.block, err)
	}
	if count < 0 {
		return fmt.Errorf("delete B-tree page slot %d in block %v: record count %d is negative", slot, page.block, count)
	}
	if slot < 0 || int64(slot) >= int64(count) {
		return fmt.Errorf("delete B-tree page slot %d in block %v: slot is outside [0,%d)", slot, page.block, count)
	}

	lastSlot := int(count) - 1
	lastPositions := make([]int, len(fields))
	for i, field := range fields {
		position, err := pageSlotFieldPosition(lastSlot, page.layout.SlotSize(), field.offset)
		if err != nil {
			return fmt.Errorf("delete B-tree page slot %d in block %v: field %q position: %w", slot, page.block, field.name, err)
		}
		if position > math.MaxInt32 {
			return fmt.Errorf("delete B-tree page slot %d in block %v: field %q offset %d exceeds int32 range", slot, page.block, field.name, position)
		}
		lastPositions[i] = position
	}

	for fromSlot := slot + 1; fromSlot <= lastSlot; fromSlot++ {
		if err := page.copyRecordBetween(lastSlot, fromSlot, fromSlot-1, fields, lastPositions); err != nil {
			return fmt.Errorf("delete B-tree page slot %d in block %v: shift record from slot %d: %w", slot, page.block, fromSlot, err)
		}
	}
	if err := page.setNumRecs(count - 1); err != nil {
		return fmt.Errorf("delete B-tree page slot %d in block %v: write record count: %w", slot, page.block, err)
	}
	return nil
}

// Format initializes block with a flag, an empty record count, and default values for each complete slot.
func (page *Page) Format(block storage.BlockID, flag int32) error {
	if page == nil {
		return fmt.Errorf("format B-tree page block %v: page is nil", block)
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("format B-tree page block %v: transaction is nil", block)
	}
	if page.layout == nil {
		return fmt.Errorf("format B-tree page block %v: layout is nil", block)
	}
	if block.FileName() == "" {
		return fmt.Errorf("format B-tree page block %v: target file name is empty", block)
	}
	if block.Number() < 0 {
		return fmt.Errorf("format B-tree page block %v: target block number is negative", block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return fmt.Errorf("format B-tree page block %v: invalid layout: %w", block, err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("format B-tree page block %v: layout schema has no fields", block)
	}
	blockSize, err := page.transaction.BlockSize()
	if err != nil {
		return fmt.Errorf("format B-tree page block %v: read block size: %w", block, err)
	}
	const headerSize = 8
	if blockSize < headerSize {
		return fmt.Errorf("format B-tree page block %v: block size %d is smaller than header size %d", block, blockSize, headerSize)
	}
	slotSize := page.layout.SlotSize()
	slotCount := (blockSize - headerSize) / slotSize
	if slotCount > 0 {
		lastSlot := slotCount - 1
		lastBase := headerSize + lastSlot*slotSize
		for _, field := range fields {
			position := lastBase + field.offset
			if position > math.MaxInt32 {
				return fmt.Errorf("format B-tree page block %v: field %q offset %d exceeds int32 range", block, field.name, position)
			}
		}
	}

	if err := page.transaction.SetInt(block, 0, flag, false); err != nil {
		return fmt.Errorf("format B-tree page block %v: write flag: %w", block, err)
	}
	if err := page.transaction.SetInt(block, 4, 0, false); err != nil {
		return fmt.Errorf("format B-tree page block %v: write record count: %w", block, err)
	}
	for slot := 0; slot < slotCount; slot++ {
		base := headerSize + slot*slotSize
		for _, field := range fields {
			position := base + field.offset
			switch field.typeOf {
			case record.IntegerField:
				if err := page.transaction.SetInt(block, int32(position), 0, false); err != nil {
					return fmt.Errorf("format B-tree page block %v: initialize field %q in slot %d at offset %d: %w", block, field.name, slot, position, err)
				}
			case record.VarcharField:
				if err := page.transaction.SetString(block, int32(position), "", false); err != nil {
					return fmt.Errorf("format B-tree page block %v: initialize field %q in slot %d at offset %d: %w", block, field.name, slot, position, err)
				}
			}
		}
	}
	return nil
}

// AppendNew appends and formats a block in the current page's file.
func (page *Page) AppendNew(flag int32) (storage.BlockID, error) {
	if page == nil {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page: page is nil")
	}
	if page.block.FileName() == "" {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page: current page is closed")
	}
	if page.block.Number() < 0 {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page from block %v: current block number is negative", page.block)
	}
	if isNilPageTransaction(page.transaction) {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: transaction is nil", page.block.FileName())
	}
	if page.layout == nil {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: layout is nil", page.block.FileName())
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: invalid layout: %w", page.block.FileName(), err)
	}
	if len(fields) == 0 {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: layout schema has no fields", page.block.FileName())
	}
	appender, ok := page.transaction.(pageBlockAppender)
	if !ok {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: transaction does not support block append", page.block.FileName())
	}
	block, err := appender.Append(page.block.FileName())
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: %w", page.block.FileName(), err)
	}
	if block.FileName() != page.block.FileName() {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: transaction returned block in file %q", page.block.FileName(), block.FileName())
	}
	if block.Number() < 0 {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page to %q: transaction returned negative block number %d", page.block.FileName(), block.Number())
	}
	if err := page.Format(block, flag); err != nil {
		return storage.BlockID{}, fmt.Errorf("append new B-tree page in block %v: format: %w", block, err)
	}
	return block, nil
}

// Split moves the records starting at splitSlot into a newly appended page.
func (page *Page) Split(splitSlot int, flag int32) (storage.BlockID, error) {
	if page == nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page: page is nil")
	}
	if page.block.FileName() == "" {
		return storage.BlockID{}, fmt.Errorf("split B-tree page: current page is closed")
	}
	if page.block.Number() < 0 {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: current block number is negative", page.block)
	}
	if isNilPageTransaction(page.transaction) {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: transaction is nil", page.block)
	}
	if page.layout == nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: layout is nil", page.block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: invalid layout: %w", page.block, err)
	}
	if len(fields) == 0 {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: layout schema has no fields", page.block)
	}
	count, err := page.GetNumRecs()
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: read record count: %w", page.block, err)
	}
	if count < 0 {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: record count %d is negative", page.block, count)
	}
	if splitSlot < 0 || int64(splitSlot) > int64(count) {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: split slot %d is outside [0,%d]", page.block, splitSlot, count)
	}
	blockSize, err := page.transaction.BlockSize()
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: read block size: %w", page.block, err)
	}
	const headerSize = 8
	if blockSize < headerSize {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: block size %d is smaller than header size %d", page.block, blockSize, headerSize)
	}
	slotSize := page.layout.SlotSize()
	slotCount := (blockSize - headerSize) / slotSize
	movedCount := int(count) - splitSlot
	if movedCount > slotCount {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: suffix has %d records but destination holds %d", page.block, movedCount, slotCount)
	}
	for _, slot := range []int{int(count) - 1, movedCount - 1} {
		if slot < 0 {
			continue
		}
		for _, field := range fields {
			position, err := pageSlotFieldPosition(slot, slotSize, field.offset)
			if err != nil {
				return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: field %q slot %d position: %w", page.block, field.name, slot, err)
			}
			if position > math.MaxInt32 {
				return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: field %q slot %d offset %d exceeds int32 range", page.block, field.name, slot, position)
			}
		}
	}
	newBlock, err := page.AppendNew(flag)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: append destination: %w", page.block, err)
	}
	destination, err := NewPage(page.transaction, newBlock, page.layout)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("split B-tree page in block %v: pin destination %v: %w", page.block, newBlock, err)
	}
	closeDestination := func(operationErr error) (storage.BlockID, error) {
		closeErr := destination.Close()
		if closeErr != nil {
			closeErr = fmt.Errorf("split B-tree page in block %v: close destination: %w", page.block, closeErr)
		}
		if operationErr != nil && closeErr != nil {
			return storage.BlockID{}, errors.Join(operationErr, closeErr)
		}
		if operationErr != nil {
			return storage.BlockID{}, operationErr
		}
		if closeErr != nil {
			return storage.BlockID{}, closeErr
		}
		return newBlock, nil
	}
	for destinationSlot := 0; destinationSlot < movedCount; destinationSlot++ {
		for _, field := range fields {
			sourcePosition := headerSize + splitSlot*slotSize + field.offset
			destinationPosition := headerSize + destinationSlot*slotSize + field.offset
			switch field.typeOf {
			case record.IntegerField:
				value, err := page.transaction.GetInt(page.block, sourcePosition)
				if err != nil {
					return closeDestination(fmt.Errorf("split B-tree page in block %v: read integer field %q at slot %d: %w", page.block, field.name, splitSlot, err))
				}
				if err := destination.transaction.SetInt(destination.block, int32(destinationPosition), value, true); err != nil {
					return closeDestination(fmt.Errorf("split B-tree page in block %v: write integer field %q at slot %d: %w", newBlock, field.name, destinationSlot, err))
				}
			case record.VarcharField:
				value, err := page.transaction.GetString(page.block, sourcePosition)
				if err != nil {
					return closeDestination(fmt.Errorf("split B-tree page in block %v: read varchar field %q at slot %d: %w", page.block, field.name, splitSlot, err))
				}
				if err := destination.transaction.SetString(destination.block, int32(destinationPosition), value, true); err != nil {
					return closeDestination(fmt.Errorf("split B-tree page in block %v: write varchar field %q at slot %d: %w", newBlock, field.name, destinationSlot, err))
				}
			}
		}
		if err := destination.setNumRecs(int32(destinationSlot + 1)); err != nil {
			return closeDestination(fmt.Errorf("split B-tree page in block %v: write destination record count: %w", newBlock, err))
		}
		if err := page.Delete(splitSlot); err != nil {
			return closeDestination(fmt.Errorf("split B-tree page in block %v: delete moved source slot %d: %w", page.block, splitSlot, err))
		}
	}
	if err := destination.SetFlag(flag); err != nil {
		return closeDestination(fmt.Errorf("split B-tree page in block %v: write destination flag: %w", newBlock, err))
	}
	return closeDestination(nil)
}

func (page *Page) insertSlot(slot int, value query.Constant) (int32, error) {
	if page == nil {
		return 0, fmt.Errorf("page is nil")
	}
	if page.block.FileName() == "" {
		return 0, fmt.Errorf("page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return 0, fmt.Errorf("transaction is nil in block %v", page.block)
	}
	if page.layout == nil {
		return 0, fmt.Errorf("layout is nil in block %v", page.block)
	}
	fields, err := btreeRecordFields(page.layout)
	if err != nil {
		return 0, fmt.Errorf("invalid layout in block %v: %w", page.block, err)
	}
	var datavalType record.FieldType
	hasDataval, hasBlock := false, false
	for _, field := range fields {
		switch field.name {
		case "dataval":
			datavalType, hasDataval = field.typeOf, true
		case "block":
			hasBlock = true
			if field.typeOf != record.IntegerField {
				return 0, fmt.Errorf("block field is not an integer in block %v", page.block)
			}
		}
	}
	if !hasDataval {
		return 0, fmt.Errorf("layout has no dataval field in block %v", page.block)
	}
	if !hasBlock {
		return 0, fmt.Errorf("layout has no block field in block %v", page.block)
	}
	switch datavalType {
	case record.IntegerField:
		if _, ok := value.AsInt(); !ok {
			return 0, fmt.Errorf("integer dataval cannot store %s in block %v", value, page.block)
		}
	case record.VarcharField:
		if _, ok := value.AsString(); !ok {
			return 0, fmt.Errorf("varchar dataval cannot store %s in block %v", value, page.block)
		}
	}

	count, err := page.GetNumRecs()
	if err != nil {
		return 0, fmt.Errorf("read record count in block %v: %w", page.block, err)
	}
	if count < 0 {
		return 0, fmt.Errorf("record count %d is negative in block %v", count, page.block)
	}
	if slot < 0 || int64(slot) > int64(count) {
		return 0, fmt.Errorf("slot %d is outside [0,%d] in block %v", slot, count, page.block)
	}
	if count == math.MaxInt32 {
		return 0, fmt.Errorf("record count cannot grow beyond int32 in block %v", page.block)
	}

	lastPositions := make([]int, len(fields))
	for i, field := range fields {
		position, err := pageSlotFieldPosition(int(count), page.layout.SlotSize(), field.offset)
		if err != nil {
			return 0, fmt.Errorf("field %q position in block %v: %w", field.name, page.block, err)
		}
		if position > math.MaxInt32 {
			return 0, fmt.Errorf("field %q offset %d exceeds int32 range in block %v", field.name, position, page.block)
		}
		lastPositions[i] = position
	}

	for toSlot := int(count); toSlot > slot; toSlot-- {
		if err := page.copyRecord(int(count), toSlot, fields, lastPositions); err != nil {
			return 0, fmt.Errorf("shift record to slot %d in block %v: %w", toSlot, page.block, err)
		}
	}
	return count, nil
}

type btreePageField struct {
	name   string
	typeOf record.FieldType
	offset int
}

func btreeRecordFields(layout *record.Layout) ([]btreePageField, error) {
	schema := layout.Schema()
	if schema == nil {
		return nil, fmt.Errorf("layout schema is nil")
	}
	slotSize := layout.SlotSize()
	if slotSize <= 0 {
		return nil, fmt.Errorf("layout slot size %d is not positive", slotSize)
	}
	names := schema.Fields()
	fields := make([]btreePageField, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("layout contains duplicate field %q", name)
		}
		seen[name] = struct{}{}
		definition, _ := schema.Field(name)
		fieldSize := 0
		switch definition.Type {
		case record.IntegerField:
			fieldSize = 4
		case record.VarcharField:
			if definition.Length < 0 || definition.Length > (math.MaxInt-4)/utf8.UTFMax {
				return nil, fmt.Errorf("field %q has invalid varchar length %d", name, definition.Length)
			}
			fieldSize = 4 + definition.Length*utf8.UTFMax
		default:
			return nil, fmt.Errorf("field %q has unsupported type %d", name, definition.Type)
		}
		offset, ok := layout.Offset(name)
		if !ok {
			return nil, fmt.Errorf("layout has no offset for field %q", name)
		}
		if offset < 0 || offset > slotSize-fieldSize {
			return nil, fmt.Errorf("field %q offset %d does not fit slot size %d", name, offset, slotSize)
		}
		fields = append(fields, btreePageField{name: name, typeOf: definition.Type, offset: offset})
	}
	return fields, nil
}

func (page *Page) copyRecord(maxSlot, toSlot int, fields []btreePageField, lastPositions []int) error {
	return page.copyRecordBetween(maxSlot, toSlot-1, toSlot, fields, lastPositions)
}

func (page *Page) copyRecordBetween(maxSlot, fromSlot, toSlot int, fields []btreePageField, lastPositions []int) error {
	slotSize := page.layout.SlotSize()
	fromDelta := (maxSlot - fromSlot) * slotSize
	toDelta := (maxSlot - toSlot) * slotSize
	for i, field := range fields {
		fromPosition := lastPositions[i] - fromDelta
		toPosition := lastPositions[i] - toDelta
		switch field.typeOf {
		case record.IntegerField:
			value, err := page.transaction.GetInt(page.block, fromPosition)
			if err != nil {
				return fmt.Errorf("copy B-tree page field %q from slot %d to %d: read integer: %w", field.name, fromSlot, toSlot, err)
			}
			if err := page.transaction.SetInt(page.block, int32(toPosition), value, true); err != nil {
				return fmt.Errorf("copy B-tree page field %q from slot %d to %d at offset %d: write integer: %w", field.name, fromSlot, toSlot, toPosition, err)
			}
		case record.VarcharField:
			value, err := page.transaction.GetString(page.block, fromPosition)
			if err != nil {
				return fmt.Errorf("copy B-tree page field %q from slot %d to %d: read varchar: %w", field.name, fromSlot, toSlot, err)
			}
			if err := page.transaction.SetString(page.block, int32(toPosition), value, true); err != nil {
				return fmt.Errorf("copy B-tree page field %q from slot %d to %d at offset %d: write varchar: %w", field.name, fromSlot, toSlot, toPosition, err)
			}
		}
	}
	return nil
}

func pageSlotFieldPosition(slot, slotSize, fieldOffset int) (int, error) {
	return pageSlotFieldPositionForCount(int64(slot), slotSize, fieldOffset)
}

func pageSlotFieldPositionForCount(slot int64, slotSize, fieldOffset int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if slot < 0 || slotSize <= 0 || fieldOffset < 0 || fieldOffset > maxInt-8 || fieldOffset >= slotSize {
		return 0, fmt.Errorf("invalid slot position inputs: slot %d, slot size %d, field offset %d", slot, slotSize, fieldOffset)
	}
	available := maxInt - 8 - fieldOffset
	if slot > int64(available/slotSize) {
		return 0, fmt.Errorf("slot %d field position exceeds int range", slot)
	}
	return 8 + int(slot)*slotSize + fieldOffset, nil
}

// Close releases the block pin owned by the page.
func (page *Page) Close() error {
	if page == nil {
		return fmt.Errorf("close nil B-tree page")
	}
	if page.block.FileName() == "" {
		return nil
	}
	if isNilPageTransaction(page.transaction) {
		return fmt.Errorf("close B-tree page for block %v: transaction is nil", page.block)
	}
	block := page.block
	if err := page.transaction.Unpin(block); err != nil {
		return fmt.Errorf("close B-tree page for block %v: unpin: %w", block, err)
	}
	page.block = storage.BlockID{}
	return nil
}

// IsFull reports whether the next B-tree record slot reaches or exceeds the block size.
func (page *Page) IsFull() (bool, error) {
	if page == nil {
		return false, fmt.Errorf("check whether B-tree page is full: page is nil")
	}
	if page.block.FileName() == "" {
		return false, fmt.Errorf("check whether B-tree page is full: page is closed")
	}
	if isNilPageTransaction(page.transaction) {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: transaction is nil", page.block)
	}
	if page.layout == nil {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: layout is nil", page.block)
	}
	if page.layout.Schema() == nil {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: layout schema is nil", page.block)
	}
	slotSize := page.layout.SlotSize()
	if slotSize <= 0 {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: invalid slot size %d", page.block, slotSize)
	}

	countValue, err := page.transaction.GetInt(page.block, 4)
	if err != nil {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: read record count: %w", page.block, err)
	}
	if countValue < 0 {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: record count %d is negative", page.block, countValue)
	}
	nextSlot := int64(countValue) + 1
	nextSlotPosition, err := pageSlotFieldPositionForCount(nextSlot, slotSize, 0)
	if err != nil {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: %w", page.block, err)
	}

	blockSize, err := page.transaction.BlockSize()
	if err != nil {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: read block size: %w", page.block, err)
	}
	if blockSize <= 0 {
		return false, fmt.Errorf("check whether B-tree page in block %v is full: invalid block size %d", page.block, blockSize)
	}
	return nextSlotPosition >= blockSize, nil
}

func isNilPageTransaction(transaction PageTransaction) bool {
	if transaction == nil {
		return true
	}
	value := reflect.ValueOf(transaction)
	return value.Kind() == reflect.Pointer && value.IsNil()
}
