package record

import (
	"fmt"
	"math"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

const (
	recordSlotEmpty int32 = 0
	recordSlotUsed  int32 = 1
)

// RecordTransaction provides the transaction operations used by a record page.
type RecordTransaction interface {
	Pin(storage.BlockID) error
	BlockSize() (int, error)
	GetInt(storage.BlockID, int) (int32, error)
	GetString(storage.BlockID, int) (string, error)
	SetInt(storage.BlockID, int32, int32, bool) error
	SetString(storage.BlockID, int32, string, bool) error
}

// RecordPage addresses fixed-size record slots within one pinned block.
type RecordPage struct {
	transaction RecordTransaction
	block       storage.BlockID
	layout      *Layout
}

// NewRecordPage pins block and returns a record page using layout.
func NewRecordPage(transaction RecordTransaction, block storage.BlockID, layout *Layout) (*RecordPage, error) {
	if err := transaction.Pin(block); err != nil {
		return nil, fmt.Errorf("initialize record page for block %v: pin: %w", block, err)
	}
	return &RecordPage{transaction: transaction, block: block, layout: layout}, nil
}

// GetInt reads an integer field from slot.
func (p *RecordPage) GetInt(slot int, fieldName string) (int32, error) {
	fieldOffset, ok := p.layout.Offset(fieldName)
	if !ok {
		return 0, fmt.Errorf("read integer from block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
	}
	position := slot*p.layout.SlotSize() + fieldOffset
	value, err := p.transaction.GetInt(p.block, position)
	if err != nil {
		return 0, fmt.Errorf("read integer from block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, position, err)
	}
	return value, nil
}

// GetString reads a string field from slot.
func (p *RecordPage) GetString(slot int, fieldName string) (string, error) {
	fieldOffset, ok := p.layout.Offset(fieldName)
	if !ok {
		return "", fmt.Errorf("read string from block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
	}
	position := slot*p.layout.SlotSize() + fieldOffset
	value, err := p.transaction.GetString(p.block, position)
	if err != nil {
		return "", fmt.Errorf("read string from block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, position, err)
	}
	return value, nil
}

// SetInt writes an integer field in slot with transaction logging enabled.
func (p *RecordPage) SetInt(slot int, fieldName string, value int32) error {
	fieldOffset, ok := p.layout.Offset(fieldName)
	if !ok {
		return fmt.Errorf("write integer to block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
	}
	position := slot*p.layout.SlotSize() + fieldOffset
	if position < math.MinInt32 || position > math.MaxInt32 {
		return fmt.Errorf("write integer to block %v slot %d field %q: offset %d exceeds int32 range", p.block, slot, fieldName, position)
	}
	if err := p.transaction.SetInt(p.block, int32(position), value, true); err != nil {
		return fmt.Errorf("write integer to block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, position, err)
	}
	return nil
}

// SetString writes a string field in slot with transaction logging enabled.
func (p *RecordPage) SetString(slot int, fieldName, value string) error {
	fieldOffset, ok := p.layout.Offset(fieldName)
	if !ok {
		return fmt.Errorf("write string to block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
	}
	position := slot*p.layout.SlotSize() + fieldOffset
	if position < math.MinInt32 || position > math.MaxInt32 {
		return fmt.Errorf("write string to block %v slot %d field %q: offset %d exceeds int32 range", p.block, slot, fieldName, position)
	}
	if err := p.transaction.SetString(p.block, int32(position), value, true); err != nil {
		return fmt.Errorf("write string to block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, position, err)
	}
	return nil
}

// Delete marks slot empty with transaction logging enabled.
func (p *RecordPage) Delete(slot int) error {
	position := slot * p.layout.SlotSize()
	if position < math.MinInt32 || position > math.MaxInt32 {
		return fmt.Errorf("delete record from block %v slot %d: offset %d exceeds int32 range", p.block, slot, position)
	}
	if err := p.transaction.SetInt(p.block, int32(position), recordSlotEmpty, true); err != nil {
		return fmt.Errorf("delete record from block %v slot %d at offset %d: %w", p.block, slot, position, err)
	}
	return nil
}

// Occupy marks slot used with transaction logging enabled.
func (p *RecordPage) Occupy(slot int) error {
	position := slot * p.layout.SlotSize()
	if position < math.MinInt32 || position > math.MaxInt32 {
		return fmt.Errorf("occupy record in block %v slot %d: offset %d exceeds int32 range", p.block, slot, position)
	}
	if err := p.transaction.SetInt(p.block, int32(position), recordSlotUsed, true); err != nil {
		return fmt.Errorf("occupy record in block %v slot %d at offset %d: %w", p.block, slot, position, err)
	}
	return nil
}

// Format initializes every complete record slot in the block without logging.
func (p *RecordPage) Format() error {
	slotSize := p.layout.SlotSize()
	if slotSize <= 0 {
		return fmt.Errorf("format record block %v: slot size %d must be positive", p.block, slotSize)
	}
	blockSize, err := p.transaction.BlockSize()
	if err != nil {
		return fmt.Errorf("format record block %v: read block size: %w", p.block, err)
	}

	for slot := 0; slot < blockSize/slotSize; slot++ {
		slotPosition := slot * slotSize
		if slotPosition < math.MinInt32 || slotPosition > math.MaxInt32 {
			return fmt.Errorf("format record block %v slot %d flag: offset %d exceeds int32 range", p.block, slot, slotPosition)
		}
		if err := p.transaction.SetInt(p.block, int32(slotPosition), recordSlotEmpty, false); err != nil {
			return fmt.Errorf("format record block %v slot %d flag at offset %d: %w", p.block, slot, slotPosition, err)
		}

		for _, fieldName := range p.layout.Schema().Fields() {
			fieldOffset, ok := p.layout.Offset(fieldName)
			if !ok {
				return fmt.Errorf("format record block %v slot %d: field %q is not in layout", p.block, slot, fieldName)
			}
			fieldPosition := slotPosition + fieldOffset
			if fieldPosition < math.MinInt32 || fieldPosition > math.MaxInt32 {
				return fmt.Errorf("format record block %v slot %d field %q: offset %d exceeds int32 range", p.block, slot, fieldName, fieldPosition)
			}
			fieldType, _ := p.layout.Schema().TypeOf(fieldName)
			if fieldType == IntegerField {
				if err := p.transaction.SetInt(p.block, int32(fieldPosition), 0, false); err != nil {
					return fmt.Errorf("format record block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, fieldPosition, err)
				}
				continue
			}
			if err := p.transaction.SetString(p.block, int32(fieldPosition), "", false); err != nil {
				return fmt.Errorf("format record block %v slot %d field %q at offset %d: %w", p.block, slot, fieldName, fieldPosition, err)
			}
		}
	}
	return nil
}

// NextAfter returns the next occupied slot after slot, or -1 when none remains.
func (p *RecordPage) NextAfter(slot int) (int, error) {
	return p.searchAfter(slot, recordSlotUsed, "occupied")
}

// NextEmptyAfter returns the next empty slot after slot without modifying it.
func (p *RecordPage) NextEmptyAfter(slot int) (int, error) {
	return p.searchAfter(slot, recordSlotEmpty, "empty")
}

func (p *RecordPage) searchAfter(slot int, target int32, description string) (int, error) {
	if slot < -1 {
		return -1, fmt.Errorf("find %s record in block %v: starting slot %d is before -1", description, p.block, slot)
	}
	slotSize := p.layout.SlotSize()
	if slotSize <= 0 {
		return -1, fmt.Errorf("find %s record in block %v: slot size %d must be positive", description, p.block, slotSize)
	}
	blockSize, err := p.transaction.BlockSize()
	if err != nil {
		return -1, fmt.Errorf("find %s record in block %v: read block size: %w", description, p.block, err)
	}
	slotCount := blockSize / slotSize
	if slot >= slotCount-1 {
		return -1, nil
	}

	for candidate := slot + 1; candidate < slotCount; candidate++ {
		position := candidate * slotSize
		flag, err := p.transaction.GetInt(p.block, position)
		if err != nil {
			return -1, fmt.Errorf("find %s record in block %v slot %d flag at offset %d: %w", description, p.block, candidate, position, err)
		}
		if flag == target {
			return candidate, nil
		}
	}
	return -1, nil
}
