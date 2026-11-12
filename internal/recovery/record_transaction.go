package recovery

type transactionOwnedRecord interface {
	TransactionID() TransactionID
}

// RecordTransaction returns the transaction that owns a recovery record.
func RecordTransaction(record Record) (TransactionID, bool) {
	if record == nil {
		return 0, false
	}
	owned, ok := record.(transactionOwnedRecord)
	if !ok {
		return 0, false
	}
	return owned.TransactionID(), true
}
