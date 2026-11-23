package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesFieldAndPreservesFollowingTokens(t *testing.T) {
	parser := NewParser("  MiXeD, 'value'")
	field, err := parser.ParseField()
	if err != nil {
		t.Fatalf("ParseField() error = %v", err)
	}
	if field != "mixed" {
		t.Fatalf("ParseField() = %q, want %q", field, "mixed")
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter after field was consumed")
	}
	if err := parser.lexer.EatDelimiter(','); err != nil {
		t.Fatalf("EatDelimiter(',') after field error = %v", err)
	}
	if !parser.lexer.MatchStringConstant() {
		t.Fatal("string following delimiter was consumed")
	}
}

func TestParserRejectsReservedAndNonFieldTokensWithoutConsuming(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "reserved word", input: "  SELECT", byteOffset: 2},
		{name: "integer", input: "42"},
		{name: "string", input: "'field'"},
		{name: "delimiter", input: ","},
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t", byteOffset: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser(test.input)
			for i := 0; i < 2; i++ {
				if _, err := parser.ParseField(); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("ParseField() error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
			if test.name == "reserved word" && !parser.lexer.MatchKeyword("select") {
				t.Fatal("failed field parse consumed the reserved word")
			}
		})
	}
}

func TestParserNilReceiverRejectsField(t *testing.T) {
	var parser *Parser
	field, err := parser.ParseField()
	if err == nil {
		t.Fatal("ParseField() error = nil for nil receiver")
	}
	if field != "" {
		t.Fatalf("ParseField() = %q, want empty string", field)
	}
}
