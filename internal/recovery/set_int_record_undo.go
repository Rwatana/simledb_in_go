package recovery

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// IntegerUndoTarget exposes the transaction operations needed to undo SETINT.
type IntegerUndoTarget interface {
	Pin(storage.BlockID) error
	SetInt(storage.BlockID, int32, int32, bool) error
	Unpin(storage.BlockID) error
}

// Undo restores the saved integer without logging the restoration.
func (r SetIntRecord) Undo(target IntegerUndoTarget) (undoErr error) {
	if err := target.Pin(r.block); err != nil {
		return fmt.Errorf("undo setint transaction %d: pin block %v: %w", r.transaction, r.block, err)
	}
	defer func() {
		if err := target.Unpin(r.block); err != nil {
			undoErr = errors.Join(undoErr, fmt.Errorf("undo setint transaction %d: unpin block %v: %w", r.transaction, r.block, err))
		}
	}()
	if err := target.SetInt(r.block, r.offset, r.oldValue, false); err != nil {
		return fmt.Errorf("undo setint transaction %d: restore block %v offset %d: %w", r.transaction, r.block, r.offset, err)
	}
	return nil
}
