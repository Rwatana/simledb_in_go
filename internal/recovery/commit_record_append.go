package recovery

import logmanager "github.com/Rwatana/simledb_in_go/internal/log"

// AppendCommitRecord appends one encoded transaction commit record.
func AppendCommitRecord(appender LogAppender, transaction TransactionID) (logmanager.LSN, error) {
	return appendTransactionRecord(appender, transaction, "commit", EncodeCommitRecord(transaction))
}
