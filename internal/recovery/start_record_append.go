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
	return appendTransactionRecord(appender, transaction, "start", EncodeStartRecord(transaction))
}

func appendTransactionRecord(appender LogAppender, transaction TransactionID, name string, encoded []byte) (logmanager.LSN, error) {
	lsn, err := appender.Append(encoded)
	if err != nil {
		return 0, fmt.Errorf("append %s record for transaction %d: %w", name, transaction, err)
	}
	return lsn, nil
}
