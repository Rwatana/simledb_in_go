package transaction

import (
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

// Transaction owns the resources and identity of one database transaction.
type Transaction struct {
	id      recovery.TransactionID
	locks   *concurrency.Manager
	buffers *BufferList
}

// New allocates an identity and creates isolated local ownership backed by shared managers.
func New(allocator *IDAllocator, lockTable *concurrency.LockTable, bufferManager BufferPinner) *Transaction {
	return &Transaction{
		id:      allocator.Next(),
		locks:   concurrency.NewManager(lockTable),
		buffers: NewBufferList(bufferManager),
	}
}

// ID returns the transaction's canonical recovery identifier.
func (t *Transaction) ID() recovery.TransactionID {
	return t.id
}
