package buffer

import (
	"sync"
	"testing"
)

func TestMarkModifiedTracksAndReplacesTheTransaction(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	if transaction, modified := frame.ModifyingTransaction(); modified {
		t.Errorf("fresh ModifyingTransaction() = (%d, true), want (_, false)", transaction)
	}

	frame.MarkModified(TransactionID(0))
	transaction, modified := frame.ModifyingTransaction()
	if !modified || transaction != 0 {
		t.Errorf("after transaction 0: ModifyingTransaction() = (%d, %t), want (0, true)", transaction, modified)
	}

	frame.MarkModified(TransactionID(17))
	transaction, modified = frame.ModifyingTransaction()
	if !modified || transaction != 17 {
		t.Errorf("after transaction 17: ModifyingTransaction() = (%d, %t), want (17, true)", transaction, modified)
	}
}

func TestModificationMetadataIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	const goroutines = 32
	var wait sync.WaitGroup
	wait.Add(goroutines)
	for index := range goroutines {
		go func(transaction TransactionID) {
			defer wait.Done()
			frame.MarkModified(transaction)
			if got, modified := frame.ModifyingTransaction(); !modified || got < 0 || got >= goroutines {
				t.Errorf("ModifyingTransaction() = (%d, %t), want a concurrently written transaction", got, modified)
			}
		}(TransactionID(index))
	}
	wait.Wait()

	transaction, modified := frame.ModifyingTransaction()
	if !modified || transaction < 0 || transaction >= goroutines {
		t.Errorf("final ModifyingTransaction() = (%d, %t), want one of [0, %d)", transaction, modified, goroutines)
	}
}
