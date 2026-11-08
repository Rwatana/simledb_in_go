package log

import (
	"fmt"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// Iterator reads a snapshot of one log block from newest record to oldest.
type Iterator struct {
	page     *storage.Page
	position int
	files    *storagefile.Manager
	block    storage.BlockID
}

// Iterator returns an in-memory snapshot iterator for the current tail block.
// Creating the iterator does not flush the tail to disk.
func (m *Manager) Iterator() (*Iterator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	page := storage.NewPageFromBytes(m.page.Snapshot())
	position, err := blockBoundary(page, m.block)
	if err != nil {
		return nil, fmt.Errorf("create iterator for current log block: %w", err)
	}
	return &Iterator{page: page, position: position, files: m.files, block: m.block}, nil
}

// Next returns the next older record in the iterator's block snapshot.
func (i *Iterator) Next() ([]byte, bool, error) {
	for i.position == i.page.Capacity() {
		if i.block.Number() == 0 {
			return nil, false, nil
		}
		if err := i.moveToPreviousBlock(); err != nil {
			return nil, false, err
		}
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

func (i *Iterator) moveToPreviousBlock() error {
	block := storage.NewBlockID(i.block.FileName(), i.block.Number()-1)
	page, err := i.files.Read(block)
	if err != nil {
		return fmt.Errorf("iterate log: read previous block %v: %w", block, err)
	}
	position, err := blockBoundary(page, block)
	if err != nil {
		return fmt.Errorf("iterate log: %w", err)
	}
	i.block = block
	i.page = page
	i.position = position
	return nil
}

func blockBoundary(page *storage.Page, block storage.BlockID) (int, error) {
	boundary, err := page.GetInt(0)
	if err != nil {
		return 0, fmt.Errorf("read boundary from %v: %w", block, err)
	}
	if boundary < boundarySize || boundary > int32(page.Capacity()) {
		return 0, fmt.Errorf("boundary %d in %v outside [%d, %d]", boundary, block, boundarySize, page.Capacity())
	}
	return int(boundary), nil
}
