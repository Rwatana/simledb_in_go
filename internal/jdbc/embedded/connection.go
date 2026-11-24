// Package embedded adapts an in-process SimpleDB manager set to Go's SQL driver contracts.
package embedded

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/server"
	"github.com/Rwatana/simledb_in_go/internal/transaction"
)

// Connection owns access to one server manager set for an embedded SQL client.
type Connection struct {
	managers          *server.Managers
	activeTransaction *transaction.Transaction
	closed            bool
}

var _ driver.Conn = (*Connection)(nil)
var _ driver.Execer = (*Connection)(nil)
var _ driver.ExecerContext = (*Connection)(nil)

// NewConnection creates an embedded connection backed by managers.
func NewConnection(managers *server.Managers) (*Connection, error) {
	if managers == nil {
		return nil, fmt.Errorf("create embedded connection: manager set is nil")
	}
	return &Connection{managers: managers}, nil
}

// Prepare creates a statement handle without parsing or executing the SQL text.
func (connection *Connection) Prepare(query string) (driver.Stmt, error) {
	if connection == nil {
		return nil, fmt.Errorf("prepare embedded statement: connection is nil")
	}
	if connection.closed {
		return nil, fmt.Errorf("prepare embedded statement: connection is closed")
	}
	return &statement{connection: connection, query: query}, nil
}

// Close marks the connection closed without completing transactions already returned to callers.
func (connection *Connection) Close() error {
	if connection == nil {
		return fmt.Errorf("close embedded connection: connection is nil")
	}
	connection.closed = true
	return nil
}

// Exec executes a database-level update in its own transaction and commits it on success.
func (connection *Connection) Exec(query string, arguments []driver.Value) (driver.Result, error) {
	if connection == nil {
		return nil, fmt.Errorf("execute embedded autocommit: connection is nil")
	}
	if connection.closed {
		return nil, fmt.Errorf("execute embedded autocommit: connection is closed")
	}
	if connection.managers == nil {
		return nil, fmt.Errorf("execute embedded autocommit: manager set is nil")
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("execute embedded autocommit: arguments are not supported")
	}
	if connection.activeTransaction != nil {
		rows, err := connection.managers.ExecuteUpdate(query, connection.activeTransaction)
		if err != nil {
			return nil, fmt.Errorf("execute embedded statement: %w", err)
		}
		return driver.RowsAffected(rows), nil
	}

	created, err := connection.managers.NewTransaction()
	if err != nil {
		return nil, fmt.Errorf("execute embedded autocommit: %w", err)
	}
	rows, err := connection.managers.ExecuteUpdate(query, created)
	if err != nil {
		_, rollbackErr := created.Rollback()
		return nil, autocommitError("execute embedded autocommit", err, rollbackErr)
	}
	if _, err := created.Commit(); err != nil {
		_, rollbackErr := created.Rollback()
		return nil, autocommitError("execute embedded autocommit: commit failed", err, rollbackErr)
	}
	return driver.RowsAffected(rows), nil
}

// ExecContext executes an update unless its context is canceled before execution begins.
func (connection *Connection) ExecContext(ctx context.Context, query string, arguments []driver.NamedValue) (driver.Result, error) {
	if ctx == nil {
		return nil, fmt.Errorf("execute embedded statement with context: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("execute embedded statement with context: %w", err)
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("execute embedded statement with context: arguments are not supported")
	}
	return connection.Exec(query, nil)
}

func autocommitError(operation string, err, rollbackErr error) error {
	contextual := fmt.Errorf("%s: %w", operation, err)
	if rollbackErr == nil {
		return contextual
	}
	return errors.Join(contextual, fmt.Errorf("rollback embedded transaction: %w", rollbackErr))
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
	if connection.activeTransaction != nil {
		return nil, fmt.Errorf("begin embedded transaction: another transaction is active")
	}
	created, err := connection.managers.NewTransaction()
	if err != nil {
		return nil, fmt.Errorf("begin embedded transaction: %w", err)
	}
	connection.activeTransaction = created
	return &transactionAdapter{transaction: created, connection: connection}, nil
}

type transactionAdapter struct {
	transaction *transaction.Transaction
	connection  *Connection
}

var _ driver.Tx = (*transactionAdapter)(nil)

func (adapter *transactionAdapter) Commit() error {
	if adapter == nil || adapter.transaction == nil {
		return fmt.Errorf("commit embedded transaction: transaction is nil")
	}
	defer func() { adapter.connection.activeTransaction = nil }()
	if _, err := adapter.transaction.Commit(); err != nil {
		return fmt.Errorf("commit embedded transaction: %w", err)
	}
	return nil
}

func (adapter *transactionAdapter) Rollback() error {
	if adapter == nil || adapter.transaction == nil {
		return fmt.Errorf("rollback embedded transaction: transaction is nil")
	}
	defer func() { adapter.connection.activeTransaction = nil }()
	if _, err := adapter.transaction.Rollback(); err != nil {
		return fmt.Errorf("rollback embedded transaction: %w", err)
	}
	return nil
}
