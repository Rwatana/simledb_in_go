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

func (statement *statement) Exec(_ []driver.Value) (driver.Result, error) {
	if statement == nil {
		return nil, fmt.Errorf("execute embedded statement: statement is nil")
	}
	if statement.closed {
		return nil, fmt.Errorf("execute embedded statement: statement is closed")
	}
	return nil, driver.ErrSkip
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
