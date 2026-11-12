package recovery

import logmanager "github.com/Rwatana/simledb_in_go/internal/log"

// AppendAndFlushRollbackRecord appends a ROLLBACK record and makes its LSN durable.
func AppendAndFlushRollbackRecord(logs DurableLog, transaction TransactionID) (logmanager.LSN, error) {
	return appendAndFlushTransactionRecord(logs, transaction, "rollback", AppendRollbackRecord)
}
