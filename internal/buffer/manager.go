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

// Unpin releases one pin from a Buffer owned by the Manager.
func (m *Manager) Unpin(frame *Buffer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if frame == nil {
		return fmt.Errorf("unpin managed buffer: frame must not be nil")
	}
	owned := false
	for _, candidate := range m.frames {
		if candidate == frame {
			owned = true
			break
		}
	}
	if !owned {
		return fmt.Errorf("unpin managed buffer: frame is not owned by this manager")
	}
	becameAvailable, err := frame.unpin()
	if err != nil {
		return fmt.Errorf("unpin managed buffer: %w", err)
	}
	if becameAvailable {
		m.available++
	}
	return nil
}

func (m *Manager) chooseUnpinnedBuffer() (*Buffer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chooseUnpinnedBufferLocked()
}

func (m *Manager) chooseUnpinnedBufferLocked() (*Buffer, bool) {
	for _, frame := range m.frames {
		if !frame.IsPinned() {
			return frame, true
		}
	}
	return nil, false
}

func (m *Manager) pinCacheMiss(block storage.BlockID) (*Buffer, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.findExistingBufferLocked(block); exists {
		return nil, false, fmt.Errorf("pin cache miss %v: block is already cached", block)
	}
	frame, found := m.chooseUnpinnedBufferLocked()
	if !found {
		return nil, false, nil
	}
	assigned, hasAssignment := frame.Block()
	_, dirty := frame.ModifyingTransaction()
	var err error
	switch {
	case !hasAssignment:
		err = frame.assignToBlock(block)
	case dirty:
		err = frame.replaceDirtyWithBlock(block)
	default:
		err = frame.reassignToBlock(block)
	}
	if err != nil {
		return nil, false, fmt.Errorf("pin cache miss %v using frame assigned to %v: %w", block, assigned, err)
	}
	frame.pin()
	m.available--
	return frame, true, nil
}
