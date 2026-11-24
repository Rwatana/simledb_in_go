package embedded

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func (statement *statement) NumInput() int {
	if statement == nil {
		return -1
	}
	positions, err := parameterPositions(statement.query)
	if err != nil {
		return -1
	}
	return len(positions)
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
	values, err := positionalValues(arguments)
	if err != nil {
		return nil, fmt.Errorf("execute embedded statement with context: %w", err)
	}
	return statement.Exec(values)
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
	values, err := positionalValues(arguments)
	if err != nil {
		return nil, fmt.Errorf("query embedded statement with context: %w", err)
	}
	return statement.Query(values)
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
	boundQuery, err := bindParameters(statement.query, arguments)
	if err != nil {
		return nil, fmt.Errorf("execute embedded statement: %w", err)
	}
	rows, err := statement.connection.managers.ExecuteUpdate(boundQuery, statement.connection.activeTransaction)
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
	boundQuery, err := bindParameters(statement.query, arguments)
	if err != nil {
		return nil, fmt.Errorf("query embedded statement: %w", err)
	}
	queryPlan, err := statement.connection.managers.CreateQueryPlan(boundQuery, statement.connection.activeTransaction)
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

func positionalValues(arguments []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(arguments))
	for index, argument := range arguments {
		if argument.Name != "" {
			return nil, fmt.Errorf("named arguments are not supported")
		}
		values[index] = argument.Value
	}
	return values, nil
}

func bindParameters(query string, arguments []driver.Value) (string, error) {
	positions, err := parameterPositions(query)
	if err != nil {
		return "", fmt.Errorf("scan SQL parameters: %w", err)
	}
	if len(arguments) != len(positions) {
		return "", fmt.Errorf("SQL has %d parameter placeholders but received %d arguments", len(positions), len(arguments))
	}
	if len(positions) == 0 {
		return query, nil
	}
	var bound strings.Builder
	bound.Grow(len(query))
	previous := 0
	for index, position := range positions {
		bound.WriteString(query[previous:position])
		literal, err := parameterLiteral(arguments[index])
		if err != nil {
			return "", fmt.Errorf("bind SQL parameter %d: %w", index+1, err)
		}
		bound.WriteString(literal)
		previous = position + 1
	}
	bound.WriteString(query[previous:])
	return bound.String(), nil
}

func parameterPositions(query string) ([]int, error) {
	positions := make([]int, 0)
	inString := false
	for index := 0; index < len(query); index++ {
		switch query[index] {
		case '\'':
			if inString && index+1 < len(query) && query[index+1] == '\'' {
				index++
				continue
			}
			inString = !inString
		case '?':
			if !inString {
				positions = append(positions, index)
			}
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated SQL string literal")
	}
	return positions, nil
}

func parameterLiteral(value driver.Value) (string, error) {
	switch value := value.(type) {
	case int64:
		return strconv.FormatInt(value, 10), nil
	case string:
		return "'" + strings.ReplaceAll(value, "'", "''") + "'", nil
	default:
		return "", fmt.Errorf("unsupported parameter type %T", value)
	}
}
