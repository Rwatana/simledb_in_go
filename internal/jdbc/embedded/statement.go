package embedded

import (
	"database/sql/driver"
	"fmt"
)

type statement struct {
	connection *Connection
	query      string
	closed     bool
}

var _ driver.Stmt = (*statement)(nil)

func (statement *statement) Close() error {
	if statement == nil {
		return fmt.Errorf("close embedded statement: statement is nil")
	}
	statement.closed = true
	return nil
}

func (*statement) NumInput() int {
	return -1
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

func (statement *statement) Query(_ []driver.Value) (driver.Rows, error) {
	if statement == nil {
		return nil, fmt.Errorf("query embedded statement: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("query embedded statement: statement is closed")
	}
	return nil, driver.ErrSkip
}
