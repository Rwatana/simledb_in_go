package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// LogPreviousString appends the current string as undo information before update.
func LogPreviousString(appender LogAppender, transaction TransactionID, view UpdateView, offset int32) (logmanager.LSN, error) {
	block, assigned := view.Block()
	if !assigned {
		return 0, fmt.Errorf("log previous string for transaction %d: buffer is not assigned", transaction)
	}
	page := view.Contents()
	if page == nil {
		return 0, fmt.Errorf("log previous string for transaction %d block %v: buffer contents are nil", transaction, block)
	}
	oldValue, err := page.GetBytes(int(offset))
	if err != nil {
		return 0, fmt.Errorf("log previous string for transaction %d block %v offset %d: %w", transaction, block, offset, err)
	}
	return AppendSetStringRecord(appender, transaction, block, offset, string(oldValue))
}
