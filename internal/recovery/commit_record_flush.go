package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// DurableLog is the recovery package's append-and-flush port.
type DurableLog interface {
	LogAppender
	Flush(lsn logmanager.LSN) error
}

// AppendAndFlushCommitRecord appends a COMMIT record and makes its LSN durable.
func AppendAndFlushCommitRecord(logs DurableLog, transaction TransactionID) (logmanager.LSN, error) {
	return appendAndFlushTransactionRecord(logs, transaction, "commit", AppendCommitRecord)
}

func appendAndFlushTransactionRecord(logs DurableLog, transaction TransactionID, name string, appendRecord func(LogAppender, TransactionID) (logmanager.LSN, error)) (logmanager.LSN, error) {
	lsn, err := appendRecord(logs, transaction)
	if err != nil {
		return 0, err
	}
	if err := logs.Flush(lsn); err != nil {
		return 0, fmt.Errorf("flush %s record for transaction %d through LSN %d: %w", name, transaction, lsn, err)
	}
	return lsn, nil
}
