// Package log appends and reads database recovery log records.
package log

import (
	"fmt"
	"math"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

const boundarySize = 4

// Manager owns the in-memory tail page of one database log file.
type Manager struct {
	files    *storagefile.Manager
	fileName string
	page     *storage.Page
	block    storage.BlockID
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
