package transaction

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// ErrTransactionCompleted reports an operation attempted after a durable transaction outcome.
var ErrTransactionCompleted = errors.New("transaction is completed")

type transactionState uint8

const (
	transactionActive transactionState = iota
	transactionCommitted
	transactionRolledBack
)

// BufferManager provides the transaction-scoped buffer operations needed by its lifecycle.
type BufferManager interface {
	BufferPinner
	FlushAll(transaction buffer.TransactionID) error
}

// FileSizer returns logical block counts for database files.
type FileSizer interface {
	Length(fileName string) (int, error)
}

// FileManager provides the file metadata and growth operations used by a transaction.
type FileManager interface {
	FileSizer
	Append(fileName string) (storage.BlockID, error)
}

// Transaction owns the resources and identity of one database transaction.
type Transaction struct {
	id            recovery.TransactionID
	locks         *concurrency.Manager
	buffers       *BufferList
	bufferManager BufferManager
	logs          recovery.DurableLog
	iterator      recovery.EncodedRecordIteratorFactory
	fileSizer     FileSizer
	fileManager   FileManager
	state         transactionState
}

// New allocates an identity, appends its START record, and creates isolated local ownership backed by shared managers.
// An optional iterator factory enables transaction rollback.
func New(allocator *IDAllocator, lockTable *concurrency.LockTable, bufferManager BufferManager, logs recovery.DurableLog, iterator ...recovery.EncodedRecordIteratorFactory) (*Transaction, error) {
	if len(iterator) > 1 {
		return nil, fmt.Errorf("create transaction: expected at most one recovery iterator factory, got %d", len(iterator))
	}
	var iteratorFactory recovery.EncodedRecordIteratorFactory
	if len(iterator) == 1 {
		iteratorFactory = iterator[0]
	}
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
		iterator:      iteratorFactory,
	}, nil
}

// NewWithFileSizer creates a transaction that can read serialized file lengths.
func NewWithFileSizer(allocator *IDAllocator, lockTable *concurrency.LockTable, fileSizer FileSizer, bufferManager BufferManager, logs recovery.DurableLog, iterator ...recovery.EncodedRecordIteratorFactory) (*Transaction, error) {
	if fileSizer == nil {
		return nil, fmt.Errorf("create transaction with file sizer: file sizer must not be nil")
	}
	transaction, err := New(allocator, lockTable, bufferManager, logs, iterator...)
	if err != nil {
		return nil, err
	}
	transaction.fileSizer = fileSizer
	return transaction, nil
}

// NewWithFileManager creates a transaction that can read file lengths and append blocks.
func NewWithFileManager(allocator *IDAllocator, lockTable *concurrency.LockTable, fileManager FileManager, bufferManager BufferManager, logs recovery.DurableLog, iterator ...recovery.EncodedRecordIteratorFactory) (*Transaction, error) {
	if fileManager == nil {
		return nil, fmt.Errorf("create transaction with file manager: file manager must not be nil")
	}
	transaction, err := NewWithFileSizer(allocator, lockTable, fileManager, bufferManager, logs, iterator...)
	if err != nil {
		return nil, err
	}
	transaction.fileManager = fileManager
	return transaction, nil
}

// ID returns the transaction's canonical recovery identifier.
func (t *Transaction) ID() recovery.TransactionID {
	return t.id
}

// Size returns the logical block count while retaining a shared lock on the file's EOF sentinel.
func (t *Transaction) Size(fileName string) (int, error) {
	if t.fileSizer == nil {
		return 0, fmt.Errorf("get transaction file size for %q: file sizer is not configured", fileName)
	}
	eof := storage.NewBlockID(fileName, -1)
	if err := t.locks.LockShared(eof); err != nil {
		return 0, fmt.Errorf("get transaction file size for %q: acquire EOF shared lock: %w", fileName, err)
	}
	length, err := t.fileSizer.Length(fileName)
	if err != nil {
		return 0, fmt.Errorf("get transaction file size for %q: %w", fileName, err)
	}
	return length, nil
}

// Append adds a block while retaining an exclusive lock on the file's EOF sentinel.
func (t *Transaction) Append(fileName string) (storage.BlockID, error) {
	if t.fileManager == nil {
		return storage.BlockID{}, fmt.Errorf("append transaction file %q: file manager is not configured", fileName)
	}
	eof := storage.NewBlockID(fileName, -1)
	if err := t.locks.LockExclusive(eof); err != nil {
		return storage.BlockID{}, fmt.Errorf("append transaction file %q: acquire EOF exclusive lock: %w", fileName, err)
	}
	block, err := t.fileManager.Append(fileName)
	if err != nil {
		return storage.BlockID{}, fmt.Errorf("append transaction file %q: %w", fileName, err)
	}
	return block, nil
}

// Pin acquires a buffer for block and records it as transaction-local ownership.
func (t *Transaction) Pin(block storage.BlockID) error {
	if t.state != transactionActive {
		return fmt.Errorf("pin block %v for transaction %d: %w", block, t.id, ErrTransactionCompleted)
	}
	return t.buffers.Pin(block)
}

// Unpin releases one transaction-local buffer pin for block.
func (t *Transaction) Unpin(block storage.BlockID) error {
	return t.buffers.Unpin(block)
}

// GetInt reads a four-byte integer from a pinned block while retaining a shared lock.
func (t *Transaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	if t.state != transactionActive {
		return 0, fmt.Errorf("get integer from block %v at offset %d for transaction %d: %w", block, offset, t.id, ErrTransactionCompleted)
	}
	frame, ok := t.buffers.Buffer(block)
	if !ok {
		return 0, fmt.Errorf("get integer from transaction buffer %v at offset %d: %w", block, offset, ErrBufferNotPinned)
	}
	if err := t.locks.LockShared(block); err != nil {
		return 0, fmt.Errorf("get integer from transaction buffer %v at offset %d: acquire shared lock: %w", block, offset, err)
	}
	value, err := frame.Contents().GetInt(offset)
	if err != nil {
		return 0, fmt.Errorf("get integer from transaction buffer %v at offset %d: %w", block, offset, err)
	}
	return value, nil
}

// GetString reads a length-prefixed string from a pinned block while retaining a shared lock.
func (t *Transaction) GetString(block storage.BlockID, offset int) (string, error) {
	if t.state != transactionActive {
		return "", fmt.Errorf("get string from block %v at offset %d for transaction %d: %w", block, offset, t.id, ErrTransactionCompleted)
	}
	frame, ok := t.buffers.Buffer(block)
	if !ok {
		return "", fmt.Errorf("get string from transaction buffer %v at offset %d: %w", block, offset, ErrBufferNotPinned)
	}
	if err := t.locks.LockShared(block); err != nil {
		return "", fmt.Errorf("get string from transaction buffer %v at offset %d: acquire shared lock: %w", block, offset, err)
	}
	value, err := frame.Contents().GetBytes(offset)
	if err != nil {
		return "", fmt.Errorf("get string from transaction buffer %v at offset %d: %w", block, offset, err)
	}
	return string(value), nil
}

// SetInt writes a four-byte integer to a pinned block and optionally records its old value.
func (t *Transaction) SetInt(block storage.BlockID, offset, value int32, logUpdate bool) error {
	if t.state != transactionActive {
		return fmt.Errorf("set integer in block %v at offset %d for transaction %d: %w", block, offset, t.id, ErrTransactionCompleted)
	}
	frame, ok := t.buffers.Buffer(block)
	if !ok {
		return fmt.Errorf("set integer in transaction buffer %v at offset %d: %w", block, offset, ErrBufferNotPinned)
	}
	if err := t.locks.LockExclusive(block); err != nil {
		return fmt.Errorf("set integer in transaction buffer %v at offset %d: acquire exclusive lock: %w", block, offset, err)
	}
	var lsn logmanager.LSN
	if logUpdate {
		var err error
		lsn, err = recovery.LogPreviousInteger(t.logs, t.id, frame, offset)
		if err != nil {
			return fmt.Errorf("set integer in transaction buffer %v at offset %d: log previous value: %w", block, offset, err)
		}
	}
	if err := frame.Contents().SetInt(int(offset), value); err != nil {
		return fmt.Errorf("set integer in transaction buffer %v at offset %d: %w", block, offset, err)
	}
	frame.MarkModified(buffer.TransactionID(t.id))
	if lsn > 0 {
		if err := frame.RecordRecoveryLSN(lsn); err != nil {
			return fmt.Errorf("set integer in transaction buffer %v at offset %d: record recovery LSN %d: %w", block, offset, lsn, err)
		}
	}
	return nil
}

// SetString writes a length-prefixed string to a pinned block and optionally records its old value.
func (t *Transaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	if t.state != transactionActive {
		return fmt.Errorf("set string in block %v at offset %d for transaction %d: %w", block, offset, t.id, ErrTransactionCompleted)
	}
	frame, ok := t.buffers.Buffer(block)
	if !ok {
		return fmt.Errorf("set string in transaction buffer %v at offset %d: %w", block, offset, ErrBufferNotPinned)
	}
	if err := t.locks.LockExclusive(block); err != nil {
		return fmt.Errorf("set string in transaction buffer %v at offset %d: acquire exclusive lock: %w", block, offset, err)
	}
	var lsn logmanager.LSN
	if logUpdate {
		var err error
		lsn, err = recovery.LogPreviousString(t.logs, t.id, frame, offset)
		if err != nil {
			return fmt.Errorf("set string in transaction buffer %v at offset %d: log previous value: %w", block, offset, err)
		}
	}
	if err := frame.Contents().SetBytes(int(offset), []byte(value)); err != nil {
		return fmt.Errorf("set string in transaction buffer %v at offset %d: %w", block, offset, err)
	}
	frame.MarkModified(buffer.TransactionID(t.id))
	if lsn > 0 {
		if err := frame.RecordRecoveryLSN(lsn); err != nil {
			return fmt.Errorf("set string in transaction buffer %v at offset %d: record recovery LSN %d: %w", block, offset, lsn, err)
		}
	}
	return nil
}

// Recover flushes this transaction's dirty buffers and makes restart recovery's CHECKPOINT durable.
func (t *Transaction) Recover() (logmanager.LSN, error) {
	if t.iterator == nil {
		return 0, fmt.Errorf("recover transaction %d: recovery iterator factory is not configured", t.id)
	}
	if err := t.bufferManager.FlushAll(buffer.TransactionID(t.id)); err != nil {
		return 0, fmt.Errorf("recover transaction %d: flush buffers before history scan: %w", t.id, err)
	}
	flush := func(transaction recovery.TransactionID) error {
		return t.bufferManager.FlushAll(buffer.TransactionID(transaction))
	}
	return recovery.RecoverRestart(t.iterator, t.id, t, flush, t.logs)
}

// Rollback restores this transaction's logged updates and makes its ROLLBACK marker durable.
func (t *Transaction) Rollback() (logmanager.LSN, error) {
	if t.state != transactionActive {
		return 0, fmt.Errorf("rollback transaction %d: %w", t.id, ErrTransactionCompleted)
	}
	if t.iterator == nil {
		return 0, fmt.Errorf("rollback transaction %d: recovery iterator factory is not configured", t.id)
	}
	flush := func(transaction recovery.TransactionID) error {
		return t.bufferManager.FlushAll(buffer.TransactionID(transaction))
	}
	lsn, err := recovery.RollbackTransaction(t.iterator, t.id, t, flush, t.logs)
	if err != nil {
		return 0, err
	}
	t.state = transactionRolledBack
	t.locks.Release()
	if err := t.buffers.UnpinAll(); err != nil {
		return lsn, fmt.Errorf("rollback transaction %d: unpin buffers after durable rollback: %w", t.id, err)
	}
	return lsn, nil
}

// Commit flushes this transaction's modified buffers and makes its COMMIT marker durable.
func (t *Transaction) Commit() (logmanager.LSN, error) {
	if t.state != transactionActive {
		return 0, fmt.Errorf("commit transaction %d: %w", t.id, ErrTransactionCompleted)
	}
	flush := func(transaction recovery.TransactionID) error {
		return t.bufferManager.FlushAll(buffer.TransactionID(transaction))
	}
	lsn, err := recovery.CommitTransaction(flush, t.logs, t.id)
	if err != nil {
		return 0, err
	}
	t.state = transactionCommitted
	t.locks.Release()
	if err := t.buffers.UnpinAll(); err != nil {
		return lsn, fmt.Errorf("commit transaction %d: unpin buffers after durable commit: %w", t.id, err)
	}
	return lsn, nil
}
