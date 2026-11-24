// Package server assembles the database managers used by a SimpleDB runtime.
package server

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/plan"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

const logFileName = "simpledb.log"

// Managers contains the file, log, buffer, metadata, and SELECT planning components for one database runtime.
type Managers struct {
	// Files reads and writes fixed-size database blocks.
	Files *storagefile.Manager
	// Logs appends and reads the database recovery log.
	Logs *logmanager.Manager
	// Buffers owns the configured pool of memory frames.
	Buffers *buffer.Manager
	// Metadata coordinates the in-memory table, view, statistics, and index metadata managers.
	Metadata *metadata.MetadataManager
	// QueryPlanner creates SELECT plans using this manager set's metadata facade.
	QueryPlanner plan.QueryPlanner
	ids          *transaction.IDAllocator
	locks        *concurrency.LockTable
}

// NewManagers creates the file, log, buffer, metadata, and basic SELECT planning components using the supplied settings.
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

	managers := &Managers{
		Files:    files,
		Logs:     logs,
		Buffers:  buffers,
		Metadata: metadata.NewMetadataManager(),
		ids:      &transaction.IDAllocator{},
		locks:    &concurrency.LockTable{},
	}
	managers.QueryPlanner = plan.NewBasicQueryPlanner(managers.Metadata)
	return managers, nil
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

// InitializeCatalogs creates the metadata catalog files using transaction without committing it.
func (m *Managers) InitializeCatalogs(transaction *transaction.Transaction) error {
	if m == nil || m.Files == nil || m.Logs == nil || m.Buffers == nil || m.Metadata == nil || m.ids == nil || m.locks == nil {
		return fmt.Errorf("initialize server metadata catalogs: manager set is incomplete")
	}
	if transaction == nil {
		return fmt.Errorf("initialize server metadata catalogs: transaction is nil")
	}
	adapter := tableScanTransaction{transaction: transaction}
	if err := m.Metadata.InitializeCatalogs(adapter); err != nil {
		return fmt.Errorf("initialize server metadata catalogs: %w", err)
	}
	return nil
}

type tableScanTransaction struct {
	transaction *transaction.Transaction
}

var _ record.TableScanTransaction = tableScanTransaction{}

func (t tableScanTransaction) Size(fileName string) (int, error) {
	return t.transaction.Size(fileName)
}

func (t tableScanTransaction) Append(fileName string) (storage.BlockID, error) {
	return t.transaction.Append(fileName)
}

func (t tableScanTransaction) Unpin(block storage.BlockID) error {
	return t.transaction.Unpin(block)
}

func (t tableScanTransaction) Pin(block storage.BlockID) error {
	return t.transaction.Pin(block)
}

func (t tableScanTransaction) BlockSize() (int, error) {
	return t.transaction.BlockSize()
}

func (t tableScanTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	return t.transaction.GetInt(block, offset)
}

func (t tableScanTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	return t.transaction.GetString(block, offset)
}

func (t tableScanTransaction) SetInt(block storage.BlockID, offset, value int32, logUpdate bool) error {
	return t.transaction.SetInt(block, offset, value, logUpdate)
}

func (t tableScanTransaction) SetString(block storage.BlockID, offset int32, value string, logUpdate bool) error {
	return t.transaction.SetString(block, offset, value, logUpdate)
}
