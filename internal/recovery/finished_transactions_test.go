package recovery

import "testing"

func TestFinishedTransactionsTracksCommitAndRollbackIdempotently(t *testing.T) {
	var finished FinishedTransactions
	finished.Observe(CommitRecord{transaction: 7})
	finished.Observe(RollbackRecord{transaction: 8})
	finished.Observe(CommitRecord{transaction: 7})

	if !finished.Contains(7) || !finished.Contains(8) {
		t.Fatalf("finished = tx7 %t tx8 %t, want both true", finished.Contains(7), finished.Contains(8))
	}
}

func TestFinishedTransactionsIgnoresNonCompletionRecords(t *testing.T) {
	var finished FinishedTransactions
	for _, record := range []Record{
		CheckpointRecord{},
		StartRecord{transaction: 7},
		SetIntRecord{transaction: 8},
		SetStringRecord{transaction: 9},
	} {
		finished.Observe(record)
	}
	for _, transaction := range []TransactionID{7, 8, 9} {
		if finished.Contains(transaction) {
			t.Fatalf("transaction %d marked finished by non-completion record", transaction)
		}
	}
}

func TestFinishedTransactionsReportsUnseenTransactionUnfinished(t *testing.T) {
	var finished FinishedTransactions
	if finished.Contains(42) {
		t.Fatal("unseen transaction reported finished")
	}
}
