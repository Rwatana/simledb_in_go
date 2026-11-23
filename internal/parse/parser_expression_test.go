package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesFieldExpressionAndPreservesFollowingTokens(t *testing.T) {
	parser := NewParser(" MiXeD, 'tail'")
	expression, err := parser.ParseExpression()
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}
	if !expression.IsFieldName() {
		t.Fatal("ParseExpression() returned a constant expression, want a field")
	}
	if field, ok := expression.AsFieldName(); !ok || field != "mixed" {
		t.Fatalf("AsFieldName() = (%q, %t), want (%q, true)", field, ok, "mixed")
	}
	if expression.String() != "mixed" {
		t.Fatalf("String() = %q, want %q", expression.String(), "mixed")
	}
	if _, ok := expression.AsConstant(); ok {
		t.Fatal("AsConstant() reported a value for a field expression")
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter after field expression was consumed")
	}
	if err := parser.lexer.EatDelimiter(','); err != nil {
		t.Fatalf("EatDelimiter(',') error = %v", err)
	}
	if !parser.lexer.MatchStringConstant() {
		t.Fatal("string after delimiter was consumed")
	}
}

func TestParserParsesIntegerAndStringConstantExpressions(t *testing.T) {
	t.Run("integer", func(t *testing.T) {
		parser := NewParser(" 42 7")
		expression, err := parser.ParseExpression()
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}
		constant, ok := expression.AsConstant()
		if !ok || constant.String() != "42" {
			t.Fatalf("AsConstant() = (%q, %t), want (42, true)", constant.String(), ok)
		}
		if expression.IsFieldName() {
			t.Fatal("integer expression is also marked as a field")
		}
		if !parser.lexer.MatchIntConstant() {
			t.Fatal("integer following the expression was consumed")
		}
	})

	t.Run("string", func(t *testing.T) {
		parser := NewParser(" 'it''s' 7")
		expression, err := parser.ParseExpression()
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}
		constant, ok := expression.AsConstant()
		if value, _ := constant.AsString(); !ok || value != "it's" {
			t.Fatalf("AsConstant() = (%q, %t), want (%q, true)", value, ok, "it's")
		}
		if !parser.lexer.MatchIntConstant() {
			t.Fatal("integer following the expression was consumed")
		}
	})
}

func TestParserRejectsInvalidExpressionTokensWithoutConsuming(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "reserved word", input: " SELECT", byteOffset: 1},
		{name: "delimiter", input: ","},
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t", byteOffset: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser(test.input)
			for attempt := 0; attempt < 2; attempt++ {
				if _, err := parser.ParseExpression(); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("ParseExpression() error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
			if test.name == "reserved word" && !parser.lexer.MatchKeyword("select") {
				t.Fatal("failed expression parse consumed the reserved word")
			}
		})
	}
}

func TestParserNilReceiverRejectsExpression(t *testing.T) {
	var parser *Parser
	expression, err := parser.ParseExpression()
	if err == nil {
		t.Fatal("ParseExpression() error = nil for nil receiver")
	}
	if expression.String() != "<unset-expression>" {
		t.Fatalf("ParseExpression() = %q, want zero expression", expression.String())
	}
}
