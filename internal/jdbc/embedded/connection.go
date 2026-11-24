// Package embedded adapts an in-process SimpleDB manager set to Go's SQL driver contracts.
package embedded

import (
	"database/sql/driver"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/server"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

// Connection owns access to one server manager set for an embedded SQL client.
type Connection struct {
	managers *server.Managers
	closed   bool
}

var _ driver.Conn = (*Connection)(nil)

// NewConnection creates an embedded connection backed by managers.
func NewConnection(managers *server.Managers) (*Connection, error) {
	if managers == nil {
		return nil, fmt.Errorf("create embedded connection: manager set is nil")
	}
	return &Connection{managers: managers}, nil
}

// Prepare reports that statement preparation is not implemented by this connection yet.
func (connection *Connection) Prepare(_ string) (driver.Stmt, error) {
	if connection == nil {
		return nil, fmt.Errorf("prepare embedded statement: connection is nil")
	}
	if connection.closed {
		return nil, fmt.Errorf("prepare embedded statement: connection is closed")
	}
	return nil, driver.ErrSkip
}

// Close marks the connection closed without completing transactions already returned to callers.
func (connection *Connection) Close() error {
	if connection == nil {
		return fmt.Errorf("close embedded connection: connection is nil")
	}
	connection.closed = true
	return nil
}

// Begin starts a server transaction that the caller completes through the returned driver transaction.
func (connection *Connection) Begin() (driver.Tx, error) {
	if connection == nil {
		return nil, fmt.Errorf("begin embedded transaction: connection is nil")
	}
	if connection.closed {
		return nil, fmt.Errorf("begin embedded transaction: connection is closed")
	}
	if connection.managers == nil {
		return nil, fmt.Errorf("begin embedded transaction: manager set is nil")
	}
	created, err := connection.managers.NewTransaction()
	if err != nil {
		return nil, fmt.Errorf("begin embedded transaction: %w", err)
	}
	return &transactionAdapter{transaction: created}, nil
}

type transactionAdapter struct {
	transaction *transaction.Transaction
}

var _ driver.Tx = (*transactionAdapter)(nil)

func (adapter *transactionAdapter) Commit() error {
	if adapter == nil || adapter.transaction == nil {
		return fmt.Errorf("commit embedded transaction: transaction is nil")
	}
	if _, err := adapter.transaction.Commit(); err != nil {
		return fmt.Errorf("commit embedded transaction: %w", err)
	}
	return nil
}

func (adapter *transactionAdapter) Rollback() error {
	if adapter == nil || adapter.transaction == nil {
		return fmt.Errorf("rollback embedded transaction: transaction is nil")
	}
	if _, err := adapter.transaction.Rollback(); err != nil {
		return fmt.Errorf("rollback embedded transaction: %w", err)
	}
	return nil
}
