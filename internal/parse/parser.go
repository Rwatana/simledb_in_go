package parse

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

// Parser converts SQL text into query and update values.
type Parser struct {
	lexer *Lexer
}

// NewParser creates a parser positioned before the first token in input.
func NewParser(input string) *Parser {
	return &Parser{lexer: NewLexer(input)}
}

// ParseField reads and returns the next identifier field name.
func (parser *Parser) ParseField() (string, error) {
	if parser == nil {
		return "", fmt.Errorf("parse field: parser is nil")
	}
	field, err := parser.lexer.EatIdentifier()
	if err != nil {
		return "", fmt.Errorf("parse field: %w", err)
	}
	return field, nil
}

// ParseExpression reads a field reference or a literal query expression.
func (parser *Parser) ParseExpression() (query.Expression, error) {
	if parser == nil {
		return query.Expression{}, fmt.Errorf("parse expression: parser is nil")
	}
	if parser.lexer.MatchIdentifier() {
		// A successful match guarantees this consume succeeds before the lexer changes.
		field, _ := parser.lexer.EatIdentifier()
		return query.NewFieldExpression(field), nil
	}
	constant, err := parser.ParseConstant()
	if err != nil {
		return query.Expression{}, fmt.Errorf("parse expression: %w", err)
	}
	return query.NewConstantExpression(constant), nil
}

// ParseTerm reads two expressions separated by the SQL equality delimiter.
func (parser *Parser) ParseTerm() (query.Term, error) {
	if parser == nil {
		return query.Term{}, fmt.Errorf("parse term: parser is nil")
	}
	lhs, err := parser.ParseExpression()
	if err != nil {
		return query.Term{}, fmt.Errorf("parse term left expression: %w", err)
	}
	if err := parser.lexer.EatDelimiter('='); err != nil {
		return query.Term{}, fmt.Errorf("parse term equals delimiter: %w", err)
	}
	rhs, err := parser.ParseExpression()
	if err != nil {
		return query.Term{}, fmt.Errorf("parse term right expression: %w", err)
	}
	return query.NewTerm(lhs, rhs), nil
}

// ParsePredicate reads one term followed by any conjunctions with the SQL and keyword.
func (parser *Parser) ParsePredicate() (query.Predicate, error) {
	if parser == nil {
		return query.Predicate{}, fmt.Errorf("parse predicate: parser is nil")
	}
	term, err := parser.ParseTerm()
	if err != nil {
		return query.Predicate{}, fmt.Errorf("parse predicate term: %w", err)
	}
	predicate := query.NewPredicateWithTerm(term)
	if !parser.lexer.MatchKeyword("and") {
		return predicate, nil
	}
	// A successful match guarantees this consume succeeds before the lexer changes.
	_ = parser.lexer.EatKeyword("and")
	rest, err := parser.ParsePredicate()
	if err != nil {
		return query.Predicate{}, fmt.Errorf("parse predicate after AND: %w", err)
	}
	return predicate.ConjoinWith(rest), nil
}

// ParseQuery reads a SELECT field list, table list, and optional predicate.
func (parser *Parser) ParseQuery() (QueryData, error) {
	if parser == nil {
		return QueryData{}, fmt.Errorf("parse query: parser is nil")
	}
	if err := parser.lexer.EatKeyword("select"); err != nil {
		return QueryData{}, fmt.Errorf("parse query SELECT keyword: %w", err)
	}
	fields, err := parser.parseQueryNameList("field list")
	if err != nil {
		return QueryData{}, err
	}
	if err := parser.lexer.EatKeyword("from"); err != nil {
		return QueryData{}, fmt.Errorf("parse query FROM keyword: %w", err)
	}
	tables, err := parser.parseQueryNameList("table list")
	if err != nil {
		return QueryData{}, err
	}
	var predicate query.Predicate
	if parser.lexer.MatchKeyword("where") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("where")
		predicate, err = parser.ParsePredicate()
		if err != nil {
			return QueryData{}, fmt.Errorf("parse query WHERE predicate: %w", err)
		}
	}
	return NewQueryData(fields, tables, predicate), nil
}

// ParseDelete reads a DELETE table name and optional predicate.
func (parser *Parser) ParseDelete() (DeleteData, error) {
	if parser == nil {
		return DeleteData{}, fmt.Errorf("parse delete: parser is nil")
	}
	if err := parser.lexer.EatKeyword("delete"); err != nil {
		return DeleteData{}, fmt.Errorf("parse delete DELETE keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("from"); err != nil {
		return DeleteData{}, fmt.Errorf("parse delete FROM keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return DeleteData{}, fmt.Errorf("parse delete table name: %w", err)
	}
	var predicate query.Predicate
	if parser.lexer.MatchKeyword("where") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("where")
		predicate, err = parser.ParsePredicate()
		if err != nil {
			return DeleteData{}, fmt.Errorf("parse delete WHERE predicate: %w", err)
		}
	}
	return NewDeleteData(tableName, predicate), nil
}

func (parser *Parser) parseQueryNameList(kind string) ([]string, error) {
	name, err := parser.ParseField()
	if err != nil {
		return nil, fmt.Errorf("parse query %s: %w", kind, err)
	}
	names := []string{name}
	for parser.lexer.MatchDelimiter(',') {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatDelimiter(',')
		name, err := parser.ParseField()
		if err != nil {
			return nil, fmt.Errorf("parse query %s: %w", kind, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// ParseConstant reads a string or non-negative integer query constant.
func (parser *Parser) ParseConstant() (query.Constant, error) {
	if parser == nil {
		return query.Constant{}, fmt.Errorf("parse constant: parser is nil")
	}
	if parser.lexer.MatchStringConstant() {
		// The preceding match guarantees this consume succeeds while the lexer state is unchanged.
		value, _ := parser.lexer.EatStringConstant()
		return query.NewStringConstant(value), nil
	}
	value, err := parser.lexer.EatIntConstant()
	if err != nil {
		return query.Constant{}, fmt.Errorf("parse integer constant: %w", err)
	}
	return query.NewIntConstant(value), nil
}
