package recovery

const (
	// OperationCommit identifies a transaction commit record.
	OperationCommit Operation = 2
)

// CommitRecord marks the successful completion of one transaction.
type CommitRecord struct {
	transaction TransactionID
}

// Operation returns OperationCommit.
func (CommitRecord) Operation() Operation {
	return OperationCommit
}

// TransactionID returns the transaction stored in the record.
func (r CommitRecord) TransactionID() TransactionID {
	return r.transaction
}

// EncodeCommitRecord returns the commit record's eight-byte on-disk form.
func EncodeCommitRecord(transaction TransactionID) []byte {
	return encodeTransactionRecord(OperationCommit, transaction)
}

// DecodeCommitRecord interprets an exact commit record encoding.
func DecodeCommitRecord(encoded []byte) (CommitRecord, error) {
	transaction, err := decodeTransactionRecord(encoded, OperationCommit, "commit")
	if err != nil {
		return CommitRecord{}, err
	}
	return CommitRecord{transaction: transaction}, nil
}
