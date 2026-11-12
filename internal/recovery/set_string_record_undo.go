package recovery

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// StringUndoTarget exposes the transaction operations needed to undo SETSTRING.
type StringUndoTarget interface {
	Pin(storage.BlockID) error
	SetString(storage.BlockID, int32, string, bool) error
	Unpin(storage.BlockID) error
}

// Undo restores the saved string without logging the restoration.
func (r SetStringRecord) Undo(target StringUndoTarget) (undoErr error) {
	if err := target.Pin(r.block); err != nil {
		return fmt.Errorf("undo setstring transaction %d: pin block %v: %w", r.transaction, r.block, err)
	}
	defer func() {
		if err := target.Unpin(r.block); err != nil {
			undoErr = errors.Join(undoErr, fmt.Errorf("undo setstring transaction %d: unpin block %v: %w", r.transaction, r.block, err))
		}
	}()
	if err := target.SetString(r.block, r.offset, r.oldValue, false); err != nil {
		return fmt.Errorf("undo setstring transaction %d: restore block %v offset %d: %w", r.transaction, r.block, r.offset, err)
	}
	return nil
}
