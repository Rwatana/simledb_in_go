package recovery

import "fmt"

// RollbackRecordSource yields decoded records newer than a transaction's START.
type RollbackRecordSource interface {
	Next() (Record, bool, error)
}

// RollbackUndoTarget supports both value types restored during rollback.
type RollbackUndoTarget interface {
	IntegerUndoTarget
	StringUndoTarget
}

// UndoRollbackRecords restores update records in newest-to-oldest source order.
func UndoRollbackRecords(source RollbackRecordSource, target RollbackUndoTarget) error {
	for {
		record, ok, err := source.Next()
		if err != nil {
			return fmt.Errorf("undo rollback records: next record: %w", err)
		}
		if !ok {
			return nil
		}
		switch update := record.(type) {
		case SetIntRecord:
			if err := update.Undo(target); err != nil {
				return fmt.Errorf("undo rollback records: SETINT: %w", err)
			}
		case SetStringRecord:
			if err := update.Undo(target); err != nil {
				return fmt.Errorf("undo rollback records: SETSTRING: %w", err)
			}
		}
	}
}
