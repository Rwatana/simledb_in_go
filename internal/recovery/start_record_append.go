package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// LogAppender is the recovery package's record-append port.
type LogAppender interface {
	Append(record []byte) (logmanager.LSN, error)
}

// AppendStartRecord appends one encoded transaction start record.
func AppendStartRecord(appender LogAppender, transaction TransactionID) (logmanager.LSN, error) {
	lsn, err := appender.Append(EncodeStartRecord(transaction))
	if err != nil {
		return 0, fmt.Errorf("append start record for transaction %d: %w", transaction, err)
	}
	return lsn, nil
}
