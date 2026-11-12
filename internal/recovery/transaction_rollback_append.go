package recovery

import logmanager "github.com/Rwatana/simledb_in_go/internal/log"

// RollbackTransactionAndAppend restores changes and appends the ROLLBACK marker.
func RollbackTransactionAndAppend(factory EncodedRecordIteratorFactory, transaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher, appender LogAppender) (logmanager.LSN, error) {
	if err := RollbackTransactionChanges(factory, transaction, target, flush); err != nil {
		return 0, err
	}
	return AppendRollbackRecord(appender, transaction)
}
