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

func TestFinishedTransactionsSelectsOnlyUnfinishedUpdatesInScanOrder(t *testing.T) {
	var finished FinishedTransactions
	tests := []struct {
		name   string
		record Record
		want   bool
	}{
		{name: "commit", record: CommitRecord{transaction: 7}},
		{name: "older update after commit", record: SetIntRecord{transaction: 7}},
		{name: "unfinished string", record: SetStringRecord{transaction: 8}, want: true},
		{name: "rollback", record: RollbackRecord{transaction: 9}},
		{name: "older update after rollback", record: SetStringRecord{transaction: 9}},
		{name: "unfinished integer", record: SetIntRecord{transaction: 10}, want: true},
		{name: "start", record: StartRecord{transaction: 10}},
		{name: "checkpoint", record: CheckpointRecord{}},
		{name: "nil", record: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := finished.ShouldUndo(test.record); got != test.want {
				t.Fatalf("ShouldUndo(%T) = %t, want %t", test.record, got, test.want)
			}
		})
	}
}
