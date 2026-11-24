// Package btree implements B-tree index pages.
package btree

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf16"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// PageTransaction provides the record-page operations and pin release used by a B-tree page.
type PageTransaction interface {
	record.RecordTransaction
	Unpin(storage.BlockID) error
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
	if page == nil {
		return -1, fmt.Errorf("find B-tree page slot before %s: page is nil", searchKey)
	}
	if isNilPageTransaction(page.transaction) {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: transaction is nil", searchKey, page.block)
	}
	if page.layout == nil {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: layout is nil", searchKey, page.block)
	}
	schema := page.layout.Schema()
	if schema == nil {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: layout schema is nil", searchKey, page.block)
	}
	dataval, ok := schema.Field("dataval")
	if !ok {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: layout has no dataval field", searchKey, page.block)
	}
	fieldOffset, ok := page.layout.Offset("dataval")
	if !ok {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: layout has no dataval offset", searchKey, page.block)
	}
	slotSize := page.layout.SlotSize()
	if slotSize <= 0 || fieldOffset < 0 || fieldOffset >= slotSize {
		return -1, fmt.Errorf("find B-tree page slot before %s in block %v: invalid dataval offset %d for slot size %d", searchKey, page.block, fieldOffset, slotSize)
	}
	if _, ok := searchKey.HashCode(); !ok {
		return -1, fmt.Errorf("find B-tree page slot in block %v: search key is unset", page.block)
	}
	switch dataval.Type {
	case record.IntegerField:
		if _, ok := searchKey.AsInt(); !ok {
			return -1, fmt.Errorf("find B-tree page slot in block %v: integer dataval cannot be compared with %s", page.block, searchKey)
		}
	case record.VarcharField:
		if _, ok := searchKey.AsString(); !ok {
			return -1, fmt.Errorf("find B-tree page slot in block %v: varchar dataval cannot be compared with %s", page.block, searchKey)
		}
	default:
		return -1, fmt.Errorf("find B-tree page slot in block %v: unsupported dataval type %d", page.block, dataval.Type)
	}

	countValue, err := page.transaction.GetInt(page.block, 4)
	if err != nil {
		return -1, fmt.Errorf("find B-tree page slot in block %v: read record count: %w", page.block, err)
	}
	if countValue < 0 {
		return -1, fmt.Errorf("find B-tree page slot in block %v: record count %d is negative", page.block, countValue)
	}
	for slot := 0; slot < int(countValue); slot++ {
		position, err := pageSlotFieldPosition(slot, slotSize, fieldOffset)
		if err != nil {
			return -1, fmt.Errorf("find B-tree page slot %d in block %v: %w", slot, page.block, err)
		}
		var value query.Constant
		switch dataval.Type {
		case record.IntegerField:
			integer, err := page.transaction.GetInt(page.block, position)
			if err != nil {
				return -1, fmt.Errorf("find B-tree page slot %d in block %v: read integer dataval: %w", slot, page.block, err)
			}
			value = query.NewIntConstant(integer)
		case record.VarcharField:
			text, err := page.transaction.GetString(page.block, position)
			if err != nil {
				return -1, fmt.Errorf("find B-tree page slot %d in block %v: read varchar dataval: %w", slot, page.block, err)
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
			return slot - 1, nil
		}
	}
	return int(countValue) - 1, nil
}

func pageSlotFieldPosition(slot, slotSize, fieldOffset int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if slot < 0 || slotSize <= 0 || fieldOffset < 0 || fieldOffset > maxInt-8 || fieldOffset >= slotSize {
		return 0, fmt.Errorf("invalid slot position inputs: slot %d, slot size %d, field offset %d", slot, slotSize, fieldOffset)
	}
	available := maxInt - 8 - fieldOffset
	if slot > available/slotSize {
		return 0, fmt.Errorf("slot %d field position exceeds int range", slot)
	}
	return 8 + slot*slotSize + fieldOffset, nil
}

func isNilPageTransaction(transaction PageTransaction) bool {
	if transaction == nil {
		return true
	}
	value := reflect.ValueOf(transaction)
	return value.Kind() == reflect.Pointer && value.IsNil()
}
