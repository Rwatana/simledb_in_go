package recovery

import "fmt"

// RecoverRestartHistory undoes unfinished updates newer than CHECKPOINT.
func RecoverRestartHistory(factory EncodedRecordIteratorFactory, target RollbackUndoTarget) error {
	iterator, err := factory()
	if err != nil {
		return fmt.Errorf("restart recovery history: create log iterator: %w", err)
	}
	bounded := NewRestartRecordIterator(iterator)
	if err := UndoRestartRecords(bounded, target); err != nil {
		return fmt.Errorf("restart recovery history: %w", err)
	}
	return nil
}
