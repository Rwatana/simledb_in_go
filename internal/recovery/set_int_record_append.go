package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// AppendSetIntRecord appends one encoded integer update recovery record.
func AppendSetIntRecord(appender LogAppender, transaction TransactionID, block storage.BlockID, offset, oldValue int32) (logmanager.LSN, error) {
	encoded, err := EncodeSetIntRecord(transaction, block, offset, oldValue)
	if err != nil {
		return 0, err
	}
	lsn, err := appender.Append(encoded)
	if err != nil {
		return 0, fmt.Errorf("append setint record for transaction %d block %v: %w", transaction, block, err)
	}
	return lsn, nil
}
