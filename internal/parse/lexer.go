// Package parse reads SQL tokens and statements for the database engine.
package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// Lexer reads tokens from SQL text without consuming a token during a match check.
type Lexer struct {
	input       string
	position    int
	tokenReady  bool
	tokenStart  int
	tokenEnd    int
	intToken    bool
	stringToken bool
	stringValue string
	wordToken   bool
	wordValue   string
	identifier  bool
}

// NewLexer creates a lexer positioned before the first token in input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: input}
}

// MatchIntConstant reports whether the next token consists of decimal digits.
// It skips leading SQL whitespace for recognition but does not consume the token.
func (lexer *Lexer) MatchIntConstant() bool {
	if lexer == nil {
		return false
	}
	lexer.peekToken()
	return lexer.intToken
}

// EatIntConstant consumes and returns the next non-negative decimal int32 token.
// It leaves the input unchanged when the next token is invalid or out of range.
func (lexer *Lexer) EatIntConstant() (int32, error) {
	if lexer == nil {
		return 0, fmt.Errorf("read integer constant: lexer is nil")
	}
	lexer.peekToken()
	if !lexer.intToken {
		return 0, fmt.Errorf("read integer constant at byte %d: next token is not an integer", lexer.tokenStart)
	}
	value, err := strconv.ParseInt(lexer.input[lexer.tokenStart:lexer.tokenEnd], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("read integer constant at byte %d: %w", lexer.tokenStart, err)
	}
	lexer.position = lexer.tokenEnd
	lexer.tokenReady = false
	return int32(value), nil
}

// MatchStringConstant reports whether the next token is a complete single-quoted string.
func (lexer *Lexer) MatchStringConstant() bool {
	if lexer == nil {
		return false
	}
	lexer.peekToken()
	return lexer.stringToken
}

// EatStringConstant consumes and returns the next single-quoted SQL string.
// Doubled single quotes inside the literal decode to one quote.
func (lexer *Lexer) EatStringConstant() (string, error) {
	if lexer == nil {
		return "", fmt.Errorf("read string constant: lexer is nil")
	}
	lexer.peekToken()
	if !lexer.stringToken {
		return "", fmt.Errorf("read string constant at byte %d: next token is not a string", lexer.tokenStart)
	}
	value := lexer.stringValue
	lexer.position = lexer.tokenEnd
	lexer.tokenReady = false
	return value, nil
}

// MatchIdentifier reports whether the next token is an identifier rather than a reserved word.
func (lexer *Lexer) MatchIdentifier() bool {
	if lexer == nil {
		return false
	}
	lexer.peekToken()
	return lexer.identifier
}

// EatIdentifier consumes the next identifier and returns its lower-case form.
func (lexer *Lexer) EatIdentifier() (string, error) {
	if lexer == nil {
		return "", fmt.Errorf("read identifier: lexer is nil")
	}
	lexer.peekToken()
	if !lexer.identifier {
		return "", fmt.Errorf("read identifier at byte %d: next token is not an identifier", lexer.tokenStart)
	}
	value := lexer.wordValue
	lexer.position = lexer.tokenEnd
	lexer.tokenReady = false
	return value, nil
}

func (lexer *Lexer) peekToken() {
	if lexer.tokenReady {
		return
	}
	start := lexer.position
	for start < len(lexer.input) && strings.ContainsRune(" \t\n\r\f\v", rune(lexer.input[start])) {
		start++
	}
	lexer.tokenStart = start
	lexer.tokenEnd = start
	lexer.intToken = false
	lexer.stringToken = false
	lexer.stringValue = ""
	lexer.wordToken = false
	lexer.wordValue = ""
	lexer.identifier = false
	if start < len(lexer.input) && isDecimalDigit(lexer.input[start]) {
		end := start + 1
		for end < len(lexer.input) && isDecimalDigit(lexer.input[end]) {
			end++
		}
		lexer.tokenEnd = end
		lexer.intToken = true
	} else if start < len(lexer.input) && lexer.input[start] == '\'' {
		var value strings.Builder
		for end := start + 1; end < len(lexer.input); {
			if lexer.input[end] != '\'' {
				value.WriteByte(lexer.input[end])
				end++
				continue
			}
			if end+1 < len(lexer.input) && lexer.input[end+1] == '\'' {
				value.WriteByte('\'')
				end += 2
				continue
			}
			lexer.tokenEnd = end + 1
			lexer.stringValue = value.String()
			lexer.stringToken = true
			break
		}
	} else if start < len(lexer.input) && isIdentifierStart(lexer.input[start]) {
		end := start + 1
		for end < len(lexer.input) && isIdentifierPart(lexer.input[end]) {
			end++
		}
		lexer.tokenEnd = end
		lexer.wordValue = strings.ToLower(lexer.input[start:end])
		lexer.wordToken = true
		lexer.identifier = !isSQLKeyword(lexer.wordValue)
	}
	lexer.tokenReady = true
}

func isDecimalDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func isIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || isDecimalDigit(value)
}

func isSQLKeyword(value string) bool {
	switch value {
	case "select", "from", "where", "and", "insert", "into", "values", "delete", "update", "set",
		"create", "table", "int", "varchar", "view", "as", "index", "on":
		return true
	default:
		return false
	}
}
