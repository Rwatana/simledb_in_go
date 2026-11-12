package recovery

import "testing"

func TestRecordTransactionReturnsEveryOwnedRecordTransaction(t *testing.T) {
	tests := []struct {
		name   string
		record Record
	}{
		{name: "start", record: StartRecord{transaction: 7}},
		{name: "commit", record: CommitRecord{transaction: 7}},
		{name: "rollback", record: RollbackRecord{transaction: 7}},
		{name: "setint", record: SetIntRecord{transaction: 7}},
		{name: "setstring", record: SetStringRecord{transaction: 7}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction, owned := RecordTransaction(test.record)
			if !owned || transaction != 7 {
				t.Fatalf("RecordTransaction() = (%d, %t), want (7, true)", transaction, owned)
			}
		})
	}
}

func TestRecordTransactionReportsUnownedCheckpointAndNil(t *testing.T) {
	for _, record := range []Record{CheckpointRecord{}, nil} {
		transaction, owned := RecordTransaction(record)
		if owned || transaction != 0 {
			t.Fatalf("RecordTransaction(%T) = (%d, %t), want (0, false)", record, transaction, owned)
		}
	}
}
