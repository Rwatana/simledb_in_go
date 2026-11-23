package recovery

import "fmt"

// UndoRestartRecords restores unfinished updates from a checkpoint-bounded source.
func UndoRestartRecords(source RollbackRecordSource, target RollbackUndoTarget) error {
	var finished FinishedTransactions
	for {
		record, ok, err := source.Next()
		if err != nil {
			return fmt.Errorf("undo restart recovery records: next record: %w", err)
		}
		if !ok {
			return nil
		}
		if !finished.ShouldUndo(record) {
			continue
		}
		switch update := record.(type) {
		case SetIntRecord:
			if err := update.Undo(target); err != nil {
				return fmt.Errorf("undo restart recovery records: SETINT: %w", err)
			}
		case SetStringRecord:
			if err := update.Undo(target); err != nil {
				return fmt.Errorf("undo restart recovery records: SETSTRING: %w", err)
			}
		}
	}
}
