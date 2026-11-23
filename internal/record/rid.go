package record

import "fmt"

// RID identifies one record slot within a file block.
type RID struct {
	blockNumber int
	slot        int
}

// NewRID returns a record identifier for blockNumber and slot.
func NewRID(blockNumber, slot int) RID {
	return RID{blockNumber: blockNumber, slot: slot}
}

// BlockNumber returns the identifier's file block number.
func (r RID) BlockNumber() int {
	return r.blockNumber
}

// Slot returns the identifier's record slot number.
func (r RID) Slot() int {
	return r.slot
}

// Equal reports whether r and other identify the same record slot.
func (r RID) Equal(other RID) bool {
	return r.blockNumber == other.blockNumber && r.slot == other.slot
}

// String returns the identifier as a block-and-slot coordinate pair.
func (r RID) String() string {
	return fmt.Sprintf("[%d, %d]", r.blockNumber, r.slot)
}
