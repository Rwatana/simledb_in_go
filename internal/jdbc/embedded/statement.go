package embedded

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
)

type statement struct {
	connection *Connection
	query      string
	closed     bool
}

var _ driver.Stmt = (*statement)(nil)
var _ driver.StmtExecContext = (*statement)(nil)
var _ driver.StmtQueryContext = (*statement)(nil)

func (statement *statement) Close() error {
	if statement == nil {
		return fmt.Errorf("close embedded statement: statement is nil")
	}
	statement.closed = true
	return nil
}

func (*statement) NumInput() int {
	return 0
}

// ExecContext executes a prepared update using the statement's caller-owned transaction.
func (statement *statement) ExecContext(ctx context.Context, arguments []driver.NamedValue) (driver.Result, error) {
	if statement == nil {
		return nil, fmt.Errorf("execute embedded statement with context: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("execute embedded statement with context: statement is closed")
	}
	if ctx == nil {
		return nil, fmt.Errorf("execute embedded statement with context: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("execute embedded statement with context: %w", err)
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("execute embedded statement with context: arguments are not supported")
	}
	return statement.Exec(nil)
}

// QueryContext executes a prepared SELECT after checking its caller's context.
func (statement *statement) QueryContext(ctx context.Context, arguments []driver.NamedValue) (driver.Rows, error) {
	if statement == nil {
		return nil, fmt.Errorf("query embedded statement with context: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("query embedded statement with context: statement is closed")
	}
	if ctx == nil {
		return nil, fmt.Errorf("query embedded statement with context: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("query embedded statement with context: %w", err)
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("query embedded statement with context: arguments are not supported")
	}
	return statement.Query(nil)
}

func (statement *statement) Exec(arguments []driver.Value) (driver.Result, error) {
	if statement == nil {
		return nil, fmt.Errorf("execute embedded statement: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("execute embedded statement: statement is closed")
	}
	if statement.connection == nil {
		return nil, fmt.Errorf("execute embedded statement: connection is nil")
	}
	if statement.connection.closed {
		return nil, fmt.Errorf("execute embedded statement: connection is closed")
	}
	if statement.connection.managers == nil {
		return nil, fmt.Errorf("execute embedded statement: manager set is nil")
	}
	if statement.connection.activeTransaction == nil {
		return nil, fmt.Errorf("execute embedded statement: no active transaction")
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("execute embedded statement: arguments are not supported")
	}
	rows, err := statement.connection.managers.ExecuteUpdate(statement.query, statement.connection.activeTransaction)
	if err != nil {
		return nil, fmt.Errorf("execute embedded statement: %w", err)
	}
	return driver.RowsAffected(rows), nil
}

func (statement *statement) Query(arguments []driver.Value) (driver.Rows, error) {
	if statement == nil {
		return nil, fmt.Errorf("query embedded statement: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("query embedded statement: statement is closed")
	}
	if statement.connection == nil {
		return nil, fmt.Errorf("query embedded statement: connection is nil")
	}
	if statement.connection.closed {
		return nil, fmt.Errorf("query embedded statement: connection is closed")
	}
	if statement.connection.managers == nil {
		return nil, fmt.Errorf("query embedded statement: manager set is nil")
	}
	if statement.connection.activeTransaction == nil {
		return nil, fmt.Errorf("query embedded statement: no active transaction")
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("query embedded statement: arguments are not supported")
	}
	queryPlan, err := statement.connection.managers.CreateQueryPlan(statement.query, statement.connection.activeTransaction)
	if err != nil {
		return nil, fmt.Errorf("query embedded statement: create query plan: %w", err)
	}
	if queryPlan == nil {
		return nil, fmt.Errorf("query embedded statement: query planner returned a nil plan")
	}
	scan, err := queryPlan.Open()
	if err != nil {
		return nil, fmt.Errorf("query embedded statement: open query scan: %w", err)
	}
	rows, err := newStatementRows(scan, queryPlan.Schema())
	if err != nil {
		if scan != nil {
			if closeErr := scan.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close rejected query scan: %w", closeErr))
			}
		}
		return nil, fmt.Errorf("query embedded statement: create result rows: %w", err)
	}
	return rows, nil
}
