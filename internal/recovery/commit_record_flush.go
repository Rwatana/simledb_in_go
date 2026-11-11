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
	lsn, err := AppendCommitRecord(logs, transaction)
	if err != nil {
		return 0, err
	}
	if err := logs.Flush(lsn); err != nil {
		return 0, fmt.Errorf("flush commit record for transaction %d through LSN %d: %w", transaction, lsn, err)
	}
	return lsn, nil
}
