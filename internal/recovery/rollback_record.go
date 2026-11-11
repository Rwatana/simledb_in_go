package recovery

const (
	// OperationRollback identifies a transaction rollback record.
	OperationRollback Operation = 3
)

// RollbackRecord marks the rolled-back completion of one transaction.
type RollbackRecord struct {
	transaction TransactionID
}

// Operation returns OperationRollback.
func (RollbackRecord) Operation() Operation {
	return OperationRollback
}

// TransactionID returns the transaction stored in the record.
func (r RollbackRecord) TransactionID() TransactionID {
	return r.transaction
}

// EncodeRollbackRecord returns the rollback record's eight-byte on-disk form.
func EncodeRollbackRecord(transaction TransactionID) []byte {
	return encodeTransactionRecord(OperationRollback, transaction)
}

// DecodeRollbackRecord interprets an exact rollback record encoding.
func DecodeRollbackRecord(encoded []byte) (RollbackRecord, error) {
	transaction, err := decodeTransactionRecord(encoded, OperationRollback, "rollback")
	if err != nil {
		return RollbackRecord{}, err
	}
	return RollbackRecord{transaction: transaction}, nil
}
