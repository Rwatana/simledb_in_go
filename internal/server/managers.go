// Package server assembles the database managers used by a SimpleDB runtime.
package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

const logFileName = "simpledb.log"

// Managers contains the file, log, and buffer managers for one database runtime.
type Managers struct {
	// Files reads and writes fixed-size database blocks.
	Files *storagefile.Manager
	// Logs appends and reads the database recovery log.
	Logs *logmanager.Manager
	// Buffers owns the configured pool of memory frames.
	Buffers *buffer.Manager
}

// NewManagers creates the file, log, and buffer managers using the supplied settings.
func NewManagers(directory string, blockSize, bufferCount int) (*Managers, error) {
	files, err := storagefile.NewManager(directory, blockSize)
	if err != nil {
		return nil, fmt.Errorf("initialize server file manager: %w", err)
	}

	logs, err := logmanager.NewManager(files, logFileName)
	if err != nil {
		return nil, fmt.Errorf("initialize server log manager: %w", err)
	}

	buffers, err := buffer.NewManager(files, logs, bufferCount)
	if err != nil {
		return nil, fmt.Errorf("initialize server buffer manager: %w", err)
	}

	return &Managers{Files: files, Logs: logs, Buffers: buffers}, nil
}
