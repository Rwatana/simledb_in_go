package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesEqualityTermAndPreservesFollowingInput(t *testing.T) {
	parser := NewParser(" Customer = 42, 'tail'")
	term, err := parser.ParseTerm()
	if err != nil {
		t.Fatalf("ParseTerm() error = %v", err)
	}
	constant, ok := term.EquatesWithConstant("customer")
	if value, valueOK := constant.AsInt(); !ok || !valueOK || value != 42 {
		t.Fatalf("EquatesWithConstant(customer) = (%q, %t), want (42, true)", constant.String(), ok)
	}
	if term.String() != "customer=42" {
		t.Fatalf("String() = %q, want %q", term.String(), "customer=42")
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter after term was consumed")
	}
	if err := parser.lexer.EatDelimiter(','); err != nil {
		t.Fatalf("EatDelimiter(',') error = %v", err)
	}
	if !parser.lexer.MatchStringConstant() {
		t.Fatal("string after delimiter was consumed")
	}
}

func TestParserParsesFieldAndLiteralTermsInEitherOrder(t *testing.T) {
	t.Run("literal on left", func(t *testing.T) {
		term, err := NewParser("7 = Age").ParseTerm()
		if err != nil {
			t.Fatalf("ParseTerm() error = %v", err)
		}
		constant, ok := term.EquatesWithConstant("age")
		if value, valueOK := constant.AsInt(); !ok || !valueOK || value != 7 {
			t.Fatalf("EquatesWithConstant(age) = (%q, %t), want (7, true)", constant.String(), ok)
		}
	})

	t.Run("two fields", func(t *testing.T) {
		term, err := NewParser("Left = Right").ParseTerm()
		if err != nil {
			t.Fatalf("ParseTerm() error = %v", err)
		}
		field, ok := term.EquatesWithField("left")
		if !ok || field != "right" {
			t.Fatalf("EquatesWithField(left) = (%q, %t), want (right, true)", field, ok)
		}
	})
}

func TestParserReportsContextualTermErrorsAtCurrentToken(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing equals", input: "left 42", context: "parse term equals delimiter", byteOffset: 5},
		{name: "invalid left", input: "select = age", context: "parse term left expression", byteOffset: 0},
		{name: "invalid right", input: "age = , next", context: "parse term right expression", byteOffset: 6},
		{name: "missing right", input: "age = ", context: "parse term right expression", byteOffset: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseTerm()
			if err == nil {
				t.Fatal("ParseTerm() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseTerm() error = %v, want %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsTerm(t *testing.T) {
	var parser *Parser
	term, err := parser.ParseTerm()
	if err == nil {
		t.Fatal("ParseTerm() error = nil for nil receiver")
	}
	if term.String() != "<unset-expression>=<unset-expression>" {
		t.Fatalf("ParseTerm() = %q, want zero term", term.String())
	}
}
