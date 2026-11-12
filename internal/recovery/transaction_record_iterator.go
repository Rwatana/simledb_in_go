package recovery

import "fmt"

// EncodedRecordIterator yields encoded recovery records from newest to oldest.
type EncodedRecordIterator interface {
	Next() ([]byte, bool, error)
}

// NextTransactionRecord returns the next record owned by transaction.
func NextTransactionRecord(iterator EncodedRecordIterator, transaction TransactionID) (Record, bool, error) {
	for {
		encoded, ok, err := iterator.Next()
		if err != nil {
			return nil, false, fmt.Errorf("select transaction %d recovery record: iterator: %w", transaction, err)
		}
		if !ok {
			return nil, false, nil
		}
		record, err := DecodeRecord(encoded)
		if err != nil {
			return nil, false, fmt.Errorf("select transaction %d recovery record: decode candidate: %w", transaction, err)
		}
		owner, owned := RecordTransaction(record)
		if owned && owner == transaction {
			return record, true, nil
		}
	}
}
