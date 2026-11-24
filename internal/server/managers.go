// Package server assembles the database managers used by a SimpleDB runtime.
package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

const logFileName = "simpledb.log"

// Managers contains the file, log, buffer, and metadata managers for one database runtime.
type Managers struct {
	// Files reads and writes fixed-size database blocks.
	Files *storagefile.Manager
	// Logs appends and reads the database recovery log.
	Logs *logmanager.Manager
	// Buffers owns the configured pool of memory frames.
	Buffers *buffer.Manager
	// Metadata coordinates the in-memory table, view, statistics, and index metadata managers.
	Metadata *metadata.MetadataManager
	ids      *transaction.IDAllocator
	locks    *concurrency.LockTable
}

// NewManagers creates the file, log, buffer, and metadata managers using the supplied settings.
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

	return &Managers{
		Files:    files,
		Logs:     logs,
		Buffers:  buffers,
		Metadata: metadata.NewMetadataManager(),
		ids:      &transaction.IDAllocator{},
		locks:    &concurrency.LockTable{},
	}, nil
}

// NewTransaction creates a transaction using this manager set's shared storage and identity state.
func (m *Managers) NewTransaction() (*transaction.Transaction, error) {
	if m == nil || m.Files == nil || m.Logs == nil || m.Buffers == nil || m.ids == nil || m.locks == nil {
		return nil, fmt.Errorf("create server transaction: manager set is incomplete")
	}

	iterator := func() (recovery.EncodedRecordIterator, error) {
		return m.Logs.Iterator()
	}
	created, err := transaction.NewWithFileManager(m.ids, m.locks, m.Files, m.Buffers, m.Logs, iterator)
	if err != nil {
		return nil, fmt.Errorf("create server transaction: %w", err)
	}
	return created, nil
}
