package transaction

import (
	"sync"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

func TestIDAllocatorReturnsSequentialRecoveryTransactionIDs(t *testing.T) {
	var allocator IDAllocator

	for want := recovery.TransactionID(1); want <= 3; want++ {
		got := allocator.Next()
		var _ recovery.TransactionID = got
		if got != want {
			t.Fatalf("Next() = %d, want %d", got, want)
		}
	}
}

func TestIDAllocatorReturnsUniqueConcurrentIDs(t *testing.T) {
	const count = 64
	var allocator IDAllocator
	results := make(chan recovery.TransactionID, count)
	var group sync.WaitGroup
	group.Add(count)
	for range count {
		go func() {
			defer group.Done()
			results <- allocator.Next()
		}()
	}
	group.Wait()
	close(results)

	seen := make(map[recovery.TransactionID]bool, count)
	for id := range results {
		if id <= 0 || id > count {
			t.Fatalf("allocated ID = %d, want within [1,%d]", id, count)
		}
		if seen[id] {
			t.Fatalf("allocated duplicate ID %d", id)
		}
		seen[id] = true
	}
	if len(seen) != count {
		t.Fatalf("unique ID count = %d, want %d", len(seen), count)
	}
}
