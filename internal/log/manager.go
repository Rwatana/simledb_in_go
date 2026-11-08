// Package log appends and reads database recovery log records.
package log

import (
	"fmt"
	"math"
	"sync"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

const boundarySize = 4

// LSN identifies one log record appended during the current process lifetime.
type LSN uint64

// Manager owns the in-memory tail page of one database log file.
type Manager struct {
	files        *storagefile.Manager
	fileName     string
	page         *storage.Page
	block        storage.BlockID
	mu           sync.Mutex
	latestLSN    LSN
	lastSavedLSN LSN
}

// NewManager loads the existing log tail or initializes its first empty block.
func NewManager(files *storagefile.Manager, fileName string) (*Manager, error) {
	if files == nil {
		return nil, fmt.Errorf("create log manager for %q: file manager must not be nil", fileName)
	}
	blockSize := files.BlockSize()
	if blockSize < boundarySize {
		return nil, fmt.Errorf("create log manager for %q: block size %d cannot store the %d-byte boundary", fileName, blockSize, boundarySize)
	}
	if blockSize > math.MaxInt32 {
		return nil, fmt.Errorf("create log manager for %q: block size %d exceeds four-byte boundary capacity", fileName, blockSize)
	}

	length, err := files.Length(fileName)
	if err != nil {
		return nil, fmt.Errorf("create log manager for %q: %w", fileName, err)
	}
	if length == 0 {
		page, err := storage.NewPage(blockSize)
		if err != nil {
			return nil, fmt.Errorf("create log manager for %q: create tail page: %w", fileName, err)
		}
		if err := page.SetInt(0, int32(blockSize)); err != nil {
			return nil, fmt.Errorf("create log manager for %q: initialize boundary: %w", fileName, err)
		}
		block, err := files.Append(fileName)
		if err != nil {
			return nil, fmt.Errorf("create log manager for %q: append first block: %w", fileName, err)
		}
		if err := files.Write(block, page); err != nil {
			return nil, fmt.Errorf("create log manager for %q: write first block: %w", fileName, err)
		}
		return &Manager{files: files, fileName: fileName, page: page, block: block}, nil
	}

	block := storage.NewBlockID(fileName, length-1)
	page, err := files.Read(block)
	if err != nil {
		return nil, fmt.Errorf("create log manager for %q: read tail block %v: %w", fileName, block, err)
	}
	boundary, err := page.GetInt(0)
	if err != nil {
		return nil, fmt.Errorf("create log manager for %q: read tail boundary: %w", fileName, err)
	}
	if boundary < boundarySize || boundary > int32(blockSize) {
		return nil, fmt.Errorf("create log manager for %q: tail boundary %d outside [%d, %d]", fileName, boundary, boundarySize, blockSize)
	}
	return &Manager{files: files, fileName: fileName, page: page, block: block}, nil
}

// Append stores record in the current tail block and returns its LSN.
// Append returns an error without changing the tail when the record does not fit.
func (m *Manager) Append(record []byte) (LSN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(record) > math.MaxInt32-boundarySize {
		return 0, fmt.Errorf("append log record of %d bytes: encoded length exceeds four-byte boundary capacity", len(record))
	}
	boundary, err := m.page.GetInt(0)
	if err != nil {
		return 0, fmt.Errorf("append log record to %v: read boundary: %w", m.block, err)
	}
	encodedLength := boundarySize + len(record)
	position := int(boundary) - encodedLength
	if position < boundarySize {
		return 0, fmt.Errorf("append log record of %d bytes to %v: encoded length %d does not fit before boundary %d", len(record), m.block, encodedLength, boundary)
	}
	if err := m.page.SetBytes(position, record); err != nil {
		return 0, fmt.Errorf("append log record to %v at offset %d: %w", m.block, position, err)
	}
	if err := m.page.SetInt(0, int32(position)); err != nil {
		return 0, fmt.Errorf("append log record to %v: update boundary to %d: %w", m.block, position, err)
	}
	m.latestLSN++
	return m.latestLSN, nil
}

// Flush ensures that the current tail, including lsn, is stored on disk.
func (m *Manager) Flush(lsn LSN) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lsn > m.latestLSN {
		return fmt.Errorf("flush LSN %d: latest allocated LSN is %d", lsn, m.latestLSN)
	}
	if lsn <= m.lastSavedLSN {
		return nil
	}
	if err := m.files.Write(m.block, m.page); err != nil {
		return fmt.Errorf("flush log tail %v through LSN %d: %w", m.block, lsn, err)
	}
	m.lastSavedLSN = m.latestLSN
	return nil
}
