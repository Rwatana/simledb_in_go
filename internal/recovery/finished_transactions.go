package recovery

// FinishedTransactions tracks transactions completed before restart recovery.
// Its zero value is ready for use.
type FinishedTransactions struct {
	transactions map[TransactionID]struct{}
}

// Observe records transaction completion markers and ignores other records.
func (f *FinishedTransactions) Observe(record Record) {
	var transaction TransactionID
	switch completed := record.(type) {
	case CommitRecord:
		transaction = completed.TransactionID()
	case RollbackRecord:
		transaction = completed.TransactionID()
	default:
		return
	}
	if f.transactions == nil {
		f.transactions = make(map[TransactionID]struct{})
	}
	f.transactions[transaction] = struct{}{}
}

// Contains reports whether transaction has a completion marker.
func (f *FinishedTransactions) Contains(transaction TransactionID) bool {
	_, exists := f.transactions[transaction]
	return exists
}
