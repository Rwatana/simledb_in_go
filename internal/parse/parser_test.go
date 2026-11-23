package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesIntegerConstant(t *testing.T) {
	parser := NewParser(" 42 , 'next'")
	constant, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() error = %v", err)
	}
	value, ok := constant.AsInt()
	if !ok || value != 42 {
		t.Fatalf("ParseConstant() = %v, want integer constant 42", constant)
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter following the integer constant was consumed")
	}
}

func TestParserParsesStringConstantAndRetainsFollowingInput(t *testing.T) {
	parser := NewParser(" 'MiXeD''s value' 7")
	constant, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() error = %v", err)
	}
	value, ok := constant.AsString()
	if !ok || value != "MiXeD's value" {
		t.Fatalf("ParseConstant() = %v, want string constant %q", constant, "MiXeD's value")
	}
	if !parser.lexer.MatchIntConstant() {
		t.Fatal("integer following the string constant was consumed")
	}
	next, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() for following integer error = %v", err)
	}
	if integer, ok := next.AsInt(); !ok || integer != 7 {
		t.Fatalf("ParseConstant() for following integer = %v, want 7", next)
	}
}

func TestParserRejectsInvalidConstantWithoutConsuming(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t", byteOffset: 2},
		{name: "identifier", input: " name", byteOffset: 1},
		{name: "delimiter", input: ","},
		{name: "unterminated string", input: "'broken"},
		{name: "integer overflow", input: "2147483648"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser(test.input)
			for i := 0; i < 2; i++ {
				if _, err := parser.ParseConstant(); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("ParseConstant() error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
		})
	}
}

func TestParserNilReceiverRejectsConstant(t *testing.T) {
	var parser *Parser
	constant, err := parser.ParseConstant()
	if err == nil {
		t.Fatal("ParseConstant() error = nil for nil receiver")
	}
	if _, ok := constant.AsInt(); ok {
		t.Fatalf("ParseConstant() integer constant = %v, want unset zero value", constant)
	}
	if _, ok := constant.AsString(); ok {
		t.Fatalf("ParseConstant() string constant = %v, want unset zero value", constant)
	}
}
