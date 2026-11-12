package recovery

import logmanager "github.com/Rwatana/simledb_in_go/internal/log"

// RecoverRestartAndAppend restores changes and appends a new CHECKPOINT.
func RecoverRestartAndAppend(factory EncodedRecordIteratorFactory, recoveryTransaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher, appender LogAppender) (logmanager.LSN, error) {
	if err := RecoverRestartChanges(factory, recoveryTransaction, target, flush); err != nil {
		return 0, err
	}
	return AppendCheckpointRecord(appender)
}
