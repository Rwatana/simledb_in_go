package transaction

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

// Transaction owns the resources and identity of one database transaction.
type Transaction struct {
	id      recovery.TransactionID
	locks   *concurrency.Manager
	buffers *BufferList
}

// New allocates an identity, appends its START record, and creates isolated local ownership backed by shared managers.
func New(allocator *IDAllocator, lockTable *concurrency.LockTable, bufferManager BufferPinner, logs recovery.LogAppender) (*Transaction, error) {
	id := allocator.Next()
	if _, err := recovery.AppendStartRecord(logs, id); err != nil {
		return nil, fmt.Errorf("start transaction %d: %w", id, err)
	}
	return &Transaction{
		id:      id,
		locks:   concurrency.NewManager(lockTable),
		buffers: NewBufferList(bufferManager),
	}, nil
}

// ID returns the transaction's canonical recovery identifier.
func (t *Transaction) ID() recovery.TransactionID {
	return t.id
}
