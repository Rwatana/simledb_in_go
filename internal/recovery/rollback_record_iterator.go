package recovery

import "fmt"

// RollbackRecordIterator yields one transaction's records newer than START.
type RollbackRecordIterator struct {
	source      EncodedRecordIterator
	transaction TransactionID
	stopped     bool
}

// NewRollbackRecordIterator creates a transaction-bounded rollback iterator.
func NewRollbackRecordIterator(source EncodedRecordIterator, transaction TransactionID) *RollbackRecordIterator {
	return &RollbackRecordIterator{source: source, transaction: transaction}
}

// Next returns the next matching record, excluding the matching START boundary.
func (i *RollbackRecordIterator) Next() (Record, bool, error) {
	if i.stopped {
		return nil, false, nil
	}
	record, ok, err := NextTransactionRecord(i.source, i.transaction)
	if err != nil {
		return nil, false, fmt.Errorf("iterate rollback transaction %d: select next record: %w", i.transaction, err)
	}
	if !ok {
		return nil, false, fmt.Errorf("iterate rollback transaction %d: reached log beginning before START", i.transaction)
	}
	if record.Operation() == OperationStart {
		i.stopped = true
		return nil, false, nil
	}
	return record, true, nil
}
