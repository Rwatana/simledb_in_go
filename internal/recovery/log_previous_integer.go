package recovery

import (
	"fmt"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// UpdateView exposes the assigned page needed to log an update.
type UpdateView interface {
	Block() (storage.BlockID, bool)
	Contents() *storage.Page
}

// LogPreviousInteger appends the current integer as undo information before update.
func LogPreviousInteger(appender LogAppender, transaction TransactionID, view UpdateView, offset int32) (logmanager.LSN, error) {
	block, assigned := view.Block()
	if !assigned {
		return 0, fmt.Errorf("log previous integer for transaction %d: buffer is not assigned", transaction)
	}
	page := view.Contents()
	if page == nil {
		return 0, fmt.Errorf("log previous integer for transaction %d block %v: buffer contents are nil", transaction, block)
	}
	oldValue, err := page.GetInt(int(offset))
	if err != nil {
		return 0, fmt.Errorf("log previous integer for transaction %d block %v offset %d: %w", transaction, block, offset, err)
	}
	return AppendSetIntRecord(appender, transaction, block, offset, oldValue)
}
