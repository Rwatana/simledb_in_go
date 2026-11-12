package recovery

import logmanager "github.com/Rwatana/simledb_in_go/internal/log"

// AppendRollbackRecord appends one encoded transaction rollback record.
func AppendRollbackRecord(appender LogAppender, transaction TransactionID) (logmanager.LSN, error) {
	return appendTransactionRecord(appender, transaction, "rollback", EncodeRollbackRecord(transaction))
}
