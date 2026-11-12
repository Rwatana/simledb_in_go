package recovery

import "fmt"

// EncodedRecordIteratorFactory creates a newest-to-oldest recovery iterator.
type EncodedRecordIteratorFactory func() (EncodedRecordIterator, error)

// RollbackTransactionHistory restores one transaction's updates back to START.
func RollbackTransactionHistory(factory EncodedRecordIteratorFactory, transaction TransactionID, target RollbackUndoTarget) error {
	iterator, err := factory()
	if err != nil {
		return fmt.Errorf("rollback transaction %d history: create log iterator: %w", transaction, err)
	}
	bounded := NewRollbackRecordIterator(iterator, transaction)
	if err := UndoRollbackRecords(bounded, target); err != nil {
		return fmt.Errorf("rollback transaction %d history: %w", transaction, err)
	}
	return nil
}
