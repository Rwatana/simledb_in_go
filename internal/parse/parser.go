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
