package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// RecoverRestart restores unfinished transactions and makes CHECKPOINT durable.
func RecoverRestart(factory EncodedRecordIteratorFactory, recoveryTransaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher, logs DurableLog) (logmanager.LSN, error) {
	lsn, err := RecoverRestartAndAppend(factory, recoveryTransaction, target, flush, logs)
	if err != nil {
		return 0, err
	}
	if err := logs.Flush(lsn); err != nil {
		return 0, fmt.Errorf("restart recovery transaction %d: flush checkpoint through LSN %d: %w", recoveryTransaction, lsn, err)
	}
	return lsn, nil
}
