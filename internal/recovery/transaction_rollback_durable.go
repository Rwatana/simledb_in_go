package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// RollbackTransaction restores one transaction and makes its marker durable.
func RollbackTransaction(factory EncodedRecordIteratorFactory, transaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher, logs DurableLog) (logmanager.LSN, error) {
	lsn, err := RollbackTransactionAndAppend(factory, transaction, target, flush, logs)
	if err != nil {
		return 0, err
	}
	if err := logs.Flush(lsn); err != nil {
		return 0, fmt.Errorf("rollback transaction %d: flush marker through LSN %d: %w", transaction, lsn, err)
	}
	return lsn, nil
}
