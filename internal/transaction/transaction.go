package transaction

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

// BufferManager provides the transaction-scoped buffer operations needed by its lifecycle.
type BufferManager interface {
	BufferPinner
	FlushAll(transaction buffer.TransactionID) error
}

// Transaction owns the resources and identity of one database transaction.
type Transaction struct {
	id            recovery.TransactionID
	locks         *concurrency.Manager
	buffers       *BufferList
	bufferManager BufferManager
	logs          recovery.DurableLog
}

// New allocates an identity, appends its START record, and creates isolated local ownership backed by shared managers.
func New(allocator *IDAllocator, lockTable *concurrency.LockTable, bufferManager BufferManager, logs recovery.DurableLog) (*Transaction, error) {
	id := allocator.Next()
	if _, err := recovery.AppendStartRecord(logs, id); err != nil {
		return nil, fmt.Errorf("start transaction %d: %w", id, err)
	}
	return &Transaction{
		id:            id,
		locks:         concurrency.NewManager(lockTable),
		buffers:       NewBufferList(bufferManager),
		bufferManager: bufferManager,
		logs:          logs,
	}, nil
}

// ID returns the transaction's canonical recovery identifier.
func (t *Transaction) ID() recovery.TransactionID {
	return t.id
}

// Commit flushes this transaction's modified buffers and makes its COMMIT marker durable.
func (t *Transaction) Commit() (logmanager.LSN, error) {
	flush := func(transaction recovery.TransactionID) error {
		return t.bufferManager.FlushAll(buffer.TransactionID(transaction))
	}
	lsn, err := recovery.CommitTransaction(flush, t.logs, t.id)
	if err != nil {
		return 0, err
	}
	t.locks.Release()
	if err := t.buffers.UnpinAll(); err != nil {
		return lsn, fmt.Errorf("commit transaction %d: unpin buffers after durable commit: %w", t.id, err)
	}
	return lsn, nil
}
