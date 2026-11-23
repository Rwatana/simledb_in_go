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
	ops       fileOperations
}

type fileOperations interface {
	Stat(string) (os.FileInfo, error)
	Open(string) (fileHandle, error)
	OpenFile(string, int, os.FileMode) (fileHandle, error)
}

type fileHandle interface {
	ReadAt([]byte, int64) (int, error)
	WriteAt([]byte, int64) (int, error)
	Sync() error
	Close() error
}

type osFileOperations struct{}

func (osFileOperations) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osFileOperations) Open(name string) (fileHandle, error) {
	return os.Open(name)
}

func (osFileOperations) OpenFile(name string, flag int, perm os.FileMode) (fileHandle, error) {
	return os.OpenFile(name, flag, perm)
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
	return &Manager{directory: directory, blockSize: blockSize, ops: osFileOperations{}}, nil
}

// BlockSize returns the number of bytes in each database block.
func (m *Manager) BlockSize() int {
	return m.blockSize
}

// Length returns the number of complete logical blocks in fileName.
// A file that does not exist has length zero.
func (m *Manager) Length(fileName string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, err := m.filePath(fileName)
	if err != nil {
		return 0, fmt.Errorf("get length of %q: %w", fileName, err)
	}
	info, err := m.ops.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("get length of %q: stat %q: %w", fileName, path, err)
	}
	blocks, err := m.blocksForSize(fileName, info.Size())
	if err != nil {
		return 0, fmt.Errorf("get length of %q: %w", fileName, err)
	}
	return blocks, nil
}

// Read returns a Page containing the complete block identified by block.
func (m *Manager) Read(block storage.BlockID) (*storage.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, offset, err := m.location(block)
	if err != nil {
		return nil, fmt.Errorf("read block %v: %w", block, err)
	}
	file, err := m.ops.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read block %v: open %q: %w", block, path, err)
	}
	defer file.Close()

	contents := make([]byte, m.blockSize)
	read, err := file.ReadAt(contents, offset)
	if err != nil {
		return nil, fmt.Errorf("read block %v at byte offset %d: read %d of %d bytes: %w", block, offset, read, m.blockSize, err)
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
	file, err := m.ops.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("write block %v: open %q: %w", block, path, err)
	}
	defer file.Close()

	contents := page.Snapshot()
	written, err := file.WriteAt(contents, offset)
	if err != nil {
		return fmt.Errorf("write block %v at byte offset %d: wrote %d of %d bytes: %w", block, offset, written, len(contents), err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("write block %v: sync %q: %w", block, path, err)
	}
	return nil
}

func (m *Manager) location(block storage.BlockID) (string, int64, error) {
	path, err := m.filePath(block.FileName())
	if err != nil {
		return "", 0, err
	}
	blockNumber := block.Number()
	if blockNumber < 0 {
		return "", 0, fmt.Errorf("block number %d must not be negative", blockNumber)
	}
	if int64(blockNumber) > math.MaxInt64/int64(m.blockSize) {
		return "", 0, fmt.Errorf("block number %d overflows byte offset for block size %d", blockNumber, m.blockSize)
	}
	return path, int64(blockNumber) * int64(m.blockSize), nil
}

func (m *Manager) filePath(fileName string) (string, error) {
	if fileName == "" || filepath.Base(fileName) != fileName || fileName == "." {
		return "", fmt.Errorf("invalid database file name %q", fileName)
	}
	return filepath.Join(m.directory, fileName), nil
}

func (m *Manager) blocksForSize(fileName string, size int64) (int, error) {
	return m.blocksForSizeWithLimit(fileName, size, int64(math.MaxInt))
}

func (m *Manager) blocksForSizeWithLimit(fileName string, size, maxInt int64) (int, error) {
	if size < 0 || size%int64(m.blockSize) != 0 {
		return 0, fmt.Errorf("database file %q size %d is not a multiple of block size %d", fileName, size, m.blockSize)
	}
	blocks := size / int64(m.blockSize)
	if blocks > maxInt {
		return 0, fmt.Errorf("database file %q has %d blocks, exceeding platform int capacity", fileName, blocks)
	}
	return int(blocks), nil
}
