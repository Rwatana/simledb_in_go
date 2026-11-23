// Package buffer caches fixed-size database pages in memory.
package buffer

import (
	"fmt"
	"sync"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// TransactionID identifies a transaction that modified a Buffer.
type TransactionID int64

// Buffer owns one in-memory Page and its optional disk block assignment.
type Buffer struct {
	files       *storagefile.Manager
	logs        *logmanager.Manager
	contents    *storage.Page
	block       *storage.BlockID
	mu          sync.Mutex
	pins        int
	modifiedBy  *TransactionID
	recoveryLSN *logmanager.LSN
}

// New returns a zero-filled Buffer that is not assigned to a disk block.
func New(files *storagefile.Manager, logs *logmanager.Manager) (*Buffer, error) {
	if files == nil {
		return nil, fmt.Errorf("create buffer: file manager must not be nil")
	}
	if logs == nil {
		return nil, fmt.Errorf("create buffer: log manager must not be nil")
	}
	contents, err := storage.NewPage(files.BlockSize())
	if err != nil {
		return nil, fmt.Errorf("create buffer contents: %w", err)
	}
	return &Buffer{files: files, logs: logs, contents: contents}, nil
}

// Contents returns the mutable Page owned by the Buffer.
func (b *Buffer) Contents() *storage.Page {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.contents
}

// Block returns the assigned disk block, if one exists.
func (b *Buffer) Block() (storage.BlockID, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.block == nil {
		return storage.BlockID{}, false
	}
	return *b.block, true
}

// IsPinned reports whether at least one client is using the Buffer.
func (b *Buffer) IsPinned() bool {
	return b.PinCount() > 0
}

// PinCount returns the number of active pins on the Buffer.
func (b *Buffer) PinCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pins
}

// pin increments the pin count and reports an available-to-pinned transition.
func (b *Buffer) pin() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	becamePinned := b.pins == 0
	b.pins++
	return becamePinned
}

// unpin decrements the pin count and reports a pinned-to-available transition.
func (b *Buffer) unpin() (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pins == 0 {
		return false, fmt.Errorf("unpin buffer: pin count is already zero")
	}
	b.pins--
	return b.pins == 0, nil
}

// MarkModified records the transaction that most recently modified the Buffer.
func (b *Buffer) MarkModified(transaction TransactionID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.modifiedBy = &transaction
}

// ModifyingTransaction returns the transaction that marked the Buffer dirty.
func (b *Buffer) ModifyingTransaction() (TransactionID, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.modifiedBy == nil {
		return 0, false
	}
	return *b.modifiedBy, true
}

// RecordRecoveryLSN associates a positive recovery log LSN with a dirty Buffer.
func (b *Buffer) RecordRecoveryLSN(lsn logmanager.LSN) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if lsn == 0 {
		return fmt.Errorf("record recovery LSN: zero is not an allocated LSN")
	}
	if b.modifiedBy == nil {
		return fmt.Errorf("record recovery LSN %d: buffer is not marked modified", lsn)
	}
	b.recoveryLSN = &lsn
	return nil
}

// RecoveryLSN returns the log record that must be durable before this Buffer.
func (b *Buffer) RecoveryLSN() (logmanager.LSN, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.recoveryLSN == nil {
		return 0, false
	}
	return *b.recoveryLSN, true
}

// assignToBlock loads the first disk block into a clean, unpinned Buffer.
func (b *Buffer) assignToBlock(block storage.BlockID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.block != nil {
		return fmt.Errorf("assign buffer to %v: already assigned to %v", block, *b.block)
	}
	if b.modifiedBy != nil {
		return fmt.Errorf("assign buffer to %v: unassigned buffer is marked modified by transaction %d", block, *b.modifiedBy)
	}
	if b.pins != 0 {
		return fmt.Errorf("assign buffer to %v: unassigned buffer has pin count %d", block, b.pins)
	}
	page, err := b.files.Read(block)
	if err != nil {
		return fmt.Errorf("assign buffer to %v: %w", block, err)
	}
	assigned := block
	b.contents = page
	b.block = &assigned
	return nil
}

// reassignToBlock replaces a clean, unpinned Buffer's disk block and contents.
func (b *Buffer) reassignToBlock(block storage.BlockID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.block == nil {
		return fmt.Errorf("reassign buffer to %v: buffer is not assigned", block)
	}
	if b.modifiedBy != nil {
		return fmt.Errorf("reassign buffer from %v to %v: buffer is modified by transaction %d", *b.block, block, *b.modifiedBy)
	}
	if b.pins != 0 {
		return fmt.Errorf("reassign buffer from %v to %v: buffer has pin count %d", *b.block, block, b.pins)
	}
	page, err := b.files.Read(block)
	if err != nil {
		return fmt.Errorf("reassign buffer from %v to %v: %w", *b.block, block, err)
	}
	assigned := block
	b.contents = page
	b.block = &assigned
	return nil
}

// flush persists a dirty Buffer in write-ahead order and marks it clean.
func (b *Buffer) flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.modifiedBy == nil {
		return nil
	}
	if b.block == nil {
		return fmt.Errorf("flush buffer modified by transaction %d: buffer is not assigned to a block", *b.modifiedBy)
	}
	if b.recoveryLSN != nil {
		if err := b.logs.Flush(*b.recoveryLSN); err != nil {
			return fmt.Errorf("flush buffer %v recovery LSN %d: %w", *b.block, *b.recoveryLSN, err)
		}
	}
	if err := b.files.Write(*b.block, b.contents); err != nil {
		return fmt.Errorf("flush buffer %v: %w", *b.block, err)
	}
	b.modifiedBy = nil
	b.recoveryLSN = nil
	return nil
}
