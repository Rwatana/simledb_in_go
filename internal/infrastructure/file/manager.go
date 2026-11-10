// Package file persists fixed-size database pages in operating-system files.
package file

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Manager reads and writes fixed-size database blocks in a directory.
type Manager struct {
	directory string
	blockSize int
	mu        sync.Mutex
}

// NewManager creates the database directory and returns a Manager for blockSize-byte blocks.
func NewManager(directory string, blockSize int) (*Manager, error) {
	if directory == "" {
		return nil, fmt.Errorf("create file manager: database directory must not be empty")
	}
	if blockSize <= 0 {
		return nil, fmt.Errorf("create file manager with block size %d: block size must be positive", blockSize)
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create database directory %q: %w", directory, err)
	}
	return &Manager{directory: directory, blockSize: blockSize}, nil
}

// BlockSize returns the number of bytes in each database block.
func (m *Manager) BlockSize() int {
	return m.blockSize
}

// Read returns a Page containing the complete block identified by block.
func (m *Manager) Read(block storage.BlockID) (*storage.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, offset, err := m.location(block)
	if err != nil {
		return nil, fmt.Errorf("read block %v: %w", block, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read block %v: open %q: %w", block, path, err)
	}
	defer file.Close()

	contents := make([]byte, m.blockSize)
	read, err := file.ReadAt(contents, offset)
	if err != nil {
		return nil, fmt.Errorf("read block %v at byte offset %d: read %d of %d bytes: %w", block, offset, read, m.blockSize, err)
	}
	if read != m.blockSize {
		return nil, fmt.Errorf("read block %v at byte offset %d: read %d of %d bytes", block, offset, read, m.blockSize)
	}
	return storage.NewPageFromBytes(contents), nil
}

// Write stores the complete page in the block identified by block.
func (m *Manager) Write(block storage.BlockID, page *storage.Page) error {
	if page == nil {
		return fmt.Errorf("write block %v: page must not be nil", block)
	}
	if page.Capacity() != m.blockSize {
		return fmt.Errorf("write block %v: page capacity %d does not match block size %d", block, page.Capacity(), m.blockSize)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path, offset, err := m.location(block)
	if err != nil {
		return fmt.Errorf("write block %v: %w", block, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("write block %v: open %q: %w", block, path, err)
	}
	defer file.Close()

	contents := page.Snapshot()
	written, err := file.WriteAt(contents, offset)
	if err != nil {
		return fmt.Errorf("write block %v at byte offset %d: wrote %d of %d bytes: %w", block, offset, written, len(contents), err)
	}
	if written != len(contents) {
		return fmt.Errorf("write block %v at byte offset %d: wrote %d of %d bytes", block, offset, written, len(contents))
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("write block %v: sync %q: %w", block, path, err)
	}
	return nil
}

func (m *Manager) location(block storage.BlockID) (string, int64, error) {
	fileName := block.FileName()
	if fileName == "" || filepath.Base(fileName) != fileName || fileName == "." {
		return "", 0, fmt.Errorf("invalid database file name %q", fileName)
	}
	blockNumber := block.Number()
	if blockNumber < 0 {
		return "", 0, fmt.Errorf("block number %d must not be negative", blockNumber)
	}
	if int64(blockNumber) > math.MaxInt64/int64(m.blockSize) {
		return "", 0, fmt.Errorf("block number %d overflows byte offset for block size %d", blockNumber, m.blockSize)
	}
	return filepath.Join(m.directory, fileName), int64(blockNumber) * int64(m.blockSize), nil
}
