package log

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Iterator reads a snapshot of one log block from newest record to oldest.
type Iterator struct {
	page     *storage.Page
	position int
}

// Iterator returns an in-memory snapshot iterator for the current tail block.
// Creating the iterator does not flush the tail to disk.
func (m *Manager) Iterator() (*Iterator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	page := storage.NewPageFromBytes(m.page.Snapshot())
	boundary, err := page.GetInt(0)
	if err != nil {
		return nil, fmt.Errorf("create iterator for current log block %v: read boundary: %w", m.block, err)
	}
	if boundary < boundarySize || boundary > int32(page.Capacity()) {
		return nil, fmt.Errorf("create iterator for current log block %v: boundary %d outside [%d, %d]", m.block, boundary, boundarySize, page.Capacity())
	}
	return &Iterator{page: page, position: int(boundary)}, nil
}

// Next returns the next older record in the iterator's block snapshot.
func (i *Iterator) Next() ([]byte, bool, error) {
	if i.position == i.page.Capacity() {
		return nil, false, nil
	}
	if i.position > i.page.Capacity() {
		return nil, false, fmt.Errorf("iterate log block: position %d exceeds page capacity %d", i.position, i.page.Capacity())
	}

	record, err := i.page.GetBytes(i.position)
	if err != nil {
		return nil, false, fmt.Errorf("iterate log block at offset %d: %w", i.position, err)
	}
	i.position += boundarySize + len(record)
	return record, true, nil
}
