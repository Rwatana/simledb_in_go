package recovery

import "fmt"

// RecoverRestartChanges restores and persists unfinished transaction updates.
func RecoverRestartChanges(factory EncodedRecordIteratorFactory, recoveryTransaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher) error {
	if err := RecoverRestartHistory(factory, target); err != nil {
		return err
	}
	if err := flush(recoveryTransaction); err != nil {
		return fmt.Errorf("restart recovery transaction %d: flush restored buffers: %w", recoveryTransaction, err)
	}
	return nil
}
