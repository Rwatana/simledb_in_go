package recovery

import (
	"fmt"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// AppendSetStringRecord appends one encoded string update recovery record.
func AppendSetStringRecord(appender LogAppender, transaction TransactionID, block storage.BlockID, offset int32, oldValue string) (logmanager.LSN, error) {
	encoded, err := EncodeSetStringRecord(transaction, block, offset, oldValue)
	if err != nil {
		return 0, err
	}
	lsn, err := appender.Append(encoded)
	if err != nil {
		return 0, fmt.Errorf("append setstring record for transaction %d block %v: %w", transaction, block, err)
	}
	return lsn, nil
}
