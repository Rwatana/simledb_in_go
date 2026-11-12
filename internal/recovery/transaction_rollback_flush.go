package recovery

import "fmt"

// TransactionBufferFlusher persists buffers modified by one transaction.
type TransactionBufferFlusher func(TransactionID) error

// RollbackTransactionChanges restores and persists one transaction's updates.
func RollbackTransactionChanges(factory EncodedRecordIteratorFactory, transaction TransactionID, target RollbackUndoTarget, flush TransactionBufferFlusher) error {
	if err := RollbackTransactionHistory(factory, transaction, target); err != nil {
		return err
	}
	if err := flush(transaction); err != nil {
		return fmt.Errorf("rollback transaction %d changes: flush restored buffers: %w", transaction, err)
	}
	return nil
}
