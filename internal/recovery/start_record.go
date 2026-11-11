package recovery

// TransactionID identifies a transaction in a recovery log record.
// Its four-byte width is part of the on-disk record format.
type TransactionID int32

const (
	// OperationStart identifies a transaction start record.
	OperationStart Operation = 1
)

// StartRecord marks the beginning of one transaction's recovery history.
type StartRecord struct {
	transaction TransactionID
}

// Operation returns OperationStart.
func (StartRecord) Operation() Operation {
	return OperationStart
}

// TransactionID returns the transaction stored in the record.
func (r StartRecord) TransactionID() TransactionID {
	return r.transaction
}

// EncodeStartRecord returns the start record's eight-byte on-disk form.
func EncodeStartRecord(transaction TransactionID) []byte {
	return encodeTransactionRecord(OperationStart, transaction)
}

// DecodeStartRecord interprets an exact start record encoding.
func DecodeStartRecord(encoded []byte) (StartRecord, error) {
	transaction, err := decodeTransactionRecord(encoded, OperationStart, "start")
	if err != nil {
		return StartRecord{}, err
	}
	return StartRecord{transaction: transaction}, nil
}
