package recovery

import (
	"fmt"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

// CommitTransaction flushes one transaction's buffers before appending and flushing its COMMIT marker.
func CommitTransaction(flush TransactionBufferFlusher, logs DurableLog, transaction TransactionID) (logmanager.LSN, error) {
	if err := flush(transaction); err != nil {
		return 0, fmt.Errorf("commit transaction %d: flush modified buffers: %w", transaction, err)
	}
	return AppendAndFlushCommitRecord(logs, transaction)
}
