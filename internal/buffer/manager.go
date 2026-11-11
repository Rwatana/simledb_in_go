package buffer

import (
	"fmt"
	"sync"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Manager owns a fixed-size pool of Buffer frames.
type Manager struct {
	frames    []*Buffer
	mu        sync.Mutex
	available int
}

// NewManager creates capacity fresh, unassigned Buffer frames.
func NewManager(files *storagefile.Manager, logs *logmanager.Manager, capacity int) (*Manager, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("create buffer manager: capacity %d must be positive", capacity)
	}
	if files == nil {
		return nil, fmt.Errorf("create buffer manager: file manager must not be nil")
	}
	if logs == nil {
		return nil, fmt.Errorf("create buffer manager: log manager must not be nil")
	}
	frames := make([]*Buffer, capacity)
	for index := range frames {
		frame, err := New(files, logs)
		if err != nil {
			return nil, fmt.Errorf("create buffer manager frame %d: %w", index, err)
		}
		frames[index] = frame
	}
	return &Manager{frames: frames, available: capacity}, nil
}

// Capacity returns the immutable number of frames in the pool.
func (m *Manager) Capacity() int {
	return len(m.frames)
}

// Available returns the number of currently unpinned frames.
func (m *Manager) Available() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.available
}

func (m *Manager) findExistingBuffer(block storage.BlockID) (*Buffer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.findExistingBufferLocked(block)
}

func (m *Manager) findExistingBufferLocked(block storage.BlockID) (*Buffer, bool) {
	for _, frame := range m.frames {
		assigned, exists := frame.Block()
		if exists && assigned == block {
			return frame, true
		}
	}
	return nil, false
}

func (m *Manager) pinExistingBuffer(block storage.BlockID) (*Buffer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	frame, found := m.findExistingBufferLocked(block)
	if !found {
		return nil, false
	}
	if frame.pin() {
		m.available--
	}
	return frame, true
}

func (m *Manager) chooseUnpinnedBuffer() (*Buffer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, frame := range m.frames {
		if !frame.IsPinned() {
			return frame, true
		}
	}
	return nil, false
}
