// Package buffer caches fixed-size database pages in memory.
package buffer

import (
	"fmt"
	"sync"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Buffer owns one in-memory Page and its optional disk block assignment.
type Buffer struct {
	files    *storagefile.Manager
	logs     *logmanager.Manager
	contents *storage.Page
	block    *storage.BlockID
	mu       sync.Mutex
	pins     int
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
	return b.contents
}

// Block returns the assigned disk block, if one exists.
func (b *Buffer) Block() (storage.BlockID, bool) {
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
