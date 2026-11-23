package parse

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestLexerMatchesAndConsumesIntConstants(t *testing.T) {
	lexer := NewLexer(" \t12\n34")
	if !lexer.MatchIntConstant() {
		t.Fatal("MatchIntConstant() = false, want true")
	}
	value, err := lexer.EatIntConstant()
	if err != nil {
		t.Fatalf("EatIntConstant() error = %v", err)
	}
	if value != 12 {
		t.Fatalf("EatIntConstant() = %d, want 12", value)
	}
	if !lexer.MatchIntConstant() {
		t.Fatal("MatchIntConstant() after first token = false, want true")
	}
	value, err = lexer.EatIntConstant()
	if err != nil {
		t.Fatalf("EatIntConstant() for second token error = %v", err)
	}
	if value != 34 {
		t.Fatalf("second EatIntConstant() = %d, want 34", value)
	}
	if lexer.MatchIntConstant() {
		t.Fatal("MatchIntConstant() at end = true, want false")
	}
}

func TestLexerRejectsNonIntegerWithoutConsumingIt(t *testing.T) {
	lexer := NewLexer(" \tstudent")
	if lexer.MatchIntConstant() {
		t.Fatal("MatchIntConstant() = true for an identifier")
	}
	if _, err := lexer.EatIntConstant(); err == nil || !strings.Contains(err.Error(), "byte 2") {
		t.Fatalf("EatIntConstant() error = %v, want token byte offset 2", err)
	}
	if _, err := lexer.EatIntConstant(); err == nil || !strings.Contains(err.Error(), "byte 2") {
		t.Fatalf("second EatIntConstant() error = %v, want same unconsumed token at byte 2", err)
	}
}

func TestLexerRejectsOutOfRangeIntegerWithoutConsumingIt(t *testing.T) {
	lexer := NewLexer("2147483648")
	if !lexer.MatchIntConstant() {
		t.Fatal("MatchIntConstant() = false for a decimal integer token")
	}
	if _, err := lexer.EatIntConstant(); !errors.Is(err, strconv.ErrRange) || !strings.Contains(err.Error(), "byte 0") {
		t.Fatalf("EatIntConstant() error = %v, want byte-positioned strconv.ErrRange", err)
	}
	if !lexer.MatchIntConstant() {
		t.Fatal("out-of-range token was consumed after EatIntConstant() failed")
	}
}

func TestLexerNilReceiverIsSafe(t *testing.T) {
	var lexer *Lexer
	if lexer.MatchIntConstant() {
		t.Fatal("nil MatchIntConstant() = true, want false")
	}
	if _, err := lexer.EatIntConstant(); err == nil {
		t.Fatal("nil EatIntConstant() error = nil")
	}
}

func TestLexerEmptyInputHasNoInteger(t *testing.T) {
	for _, input := range []string{"", " \t\n"} {
		lexer := NewLexer(input)
		if lexer.MatchIntConstant() {
			t.Fatalf("MatchIntConstant() = true for input %q", input)
		}
		if _, err := lexer.EatIntConstant(); err == nil {
			t.Fatalf("EatIntConstant() error = nil for input %q", input)
		}
	}
}

func TestLexerMatchesAndConsumesStringConstants(t *testing.T) {
	lexer := NewLexer("  'MiXeD 東京'   'it''s SQL' 42")
	if !lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() = false, want true")
	}
	if !lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() consumed the token")
	}
	value, err := lexer.EatStringConstant()
	if err != nil {
		t.Fatalf("EatStringConstant() error = %v", err)
	}
	if value != "MiXeD 東京" {
		t.Fatalf("EatStringConstant() = %q, want original case and UTF-8 content", value)
	}
	if !lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() for doubled quote = false, want true")
	}
	value, err = lexer.EatStringConstant()
	if err != nil {
		t.Fatalf("EatStringConstant() for doubled quote error = %v", err)
	}
	if value != "it's SQL" {
		t.Fatalf("EatStringConstant() for doubled quote = %q, want %q", value, "it's SQL")
	}
	if !lexer.MatchIntConstant() {
		t.Fatal("integer after string constants was not preserved")
	}
}

func TestLexerConsumesEmptyStringConstant(t *testing.T) {
	lexer := NewLexer("''")
	if !lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() = false for an empty string")
	}
	value, err := lexer.EatStringConstant()
	if err != nil {
		t.Fatalf("EatStringConstant() error = %v", err)
	}
	if value != "" {
		t.Fatalf("EatStringConstant() = %q, want empty string", value)
	}
}

func TestLexerRejectsUnterminatedStringWithoutConsumingIt(t *testing.T) {
	lexer := NewLexer("  'unterminated")
	if lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() = true for an unterminated string")
	}
	for i := 0; i < 2; i++ {
		if _, err := lexer.EatStringConstant(); err == nil || !strings.Contains(err.Error(), "byte 2") {
			t.Fatalf("EatStringConstant() error = %v, want stable byte offset 2", err)
		}
	}
}

func TestLexerRejectsNonStringWithoutConsumingIt(t *testing.T) {
	lexer := NewLexer(" 12")
	if lexer.MatchStringConstant() {
		t.Fatal("MatchStringConstant() = true for an integer")
	}
	if _, err := lexer.EatStringConstant(); err == nil || !strings.Contains(err.Error(), "byte 1") {
		t.Fatalf("EatStringConstant() error = %v, want token byte offset 1", err)
	}
	if !lexer.MatchIntConstant() {
		t.Fatal("rejected string token attempt consumed the integer")
	}
}

func TestLexerNilReceiverIsSafeForStringConstants(t *testing.T) {
	var lexer *Lexer
	if lexer.MatchStringConstant() {
		t.Fatal("nil MatchStringConstant() = true, want false")
	}
	if _, err := lexer.EatStringConstant(); err == nil {
		t.Fatal("nil EatStringConstant() error = nil")
	}
}

func TestLexerEmptyInputHasNoStringConstant(t *testing.T) {
	for _, input := range []string{"", " \t\n"} {
		lexer := NewLexer(input)
		if lexer.MatchStringConstant() {
			t.Fatalf("MatchStringConstant() = true for input %q", input)
		}
		if _, err := lexer.EatStringConstant(); err == nil {
			t.Fatalf("EatStringConstant() error = nil for input %q", input)
		}
	}
}

func TestLexerMatchesAndConsumesIdentifiers(t *testing.T) {
	lexer := NewLexer(" \tStuDent_4 9")
	if !lexer.MatchIdentifier() {
		t.Fatal("MatchIdentifier() = false, want true")
	}
	if !lexer.MatchIdentifier() {
		t.Fatal("MatchIdentifier() consumed the identifier")
	}
	identifier, err := lexer.EatIdentifier()
	if err != nil {
		t.Fatalf("EatIdentifier() error = %v", err)
	}
	if identifier != "student_4" {
		t.Fatalf("EatIdentifier() = %q, want lower-case identifier", identifier)
	}
	if !lexer.MatchIntConstant() {
		t.Fatal("integer after identifier was not preserved")
	}
}

func TestLexerRejectsReservedWordsAsIdentifiers(t *testing.T) {
	keywords := []string{
		"select", "from", "where", "and", "insert", "into", "values", "delete", "update",
		"set", "create", "table", "int", "varchar", "view", "as", "index", "on",
	}
	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			lexer := NewLexer(strings.ToUpper(keyword))
			if lexer.MatchIdentifier() {
				t.Fatalf("MatchIdentifier() = true for reserved word %q", keyword)
			}
			for i := 0; i < 2; i++ {
				if _, err := lexer.EatIdentifier(); err == nil || !strings.Contains(err.Error(), "byte 0") {
					t.Fatalf("EatIdentifier() error = %v, want stable byte offset 0", err)
				}
			}
		})
	}
}

func TestLexerRejectsNonIdentifiersWithoutConsumingThem(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t\n", byteOffset: 3},
		{name: "integer", input: "123"},
		{name: "punctuation", input: "$name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lexer := NewLexer(test.input)
			if lexer.MatchIdentifier() {
				t.Fatalf("MatchIdentifier() = true for input %q", test.input)
			}
			for i := 0; i < 2; i++ {
				if _, err := lexer.EatIdentifier(); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("EatIdentifier() error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
			if test.input == "123" && !lexer.MatchIntConstant() {
				t.Fatal("rejected identifier attempt consumed the integer")
			}
		})
	}
}

func TestLexerNilReceiverIsSafeForIdentifiers(t *testing.T) {
	var lexer *Lexer
	if lexer.MatchIdentifier() {
		t.Fatal("nil MatchIdentifier() = true, want false")
	}
	if _, err := lexer.EatIdentifier(); err == nil {
		t.Fatal("nil EatIdentifier() error = nil")
	}
}

func TestLexerMatchesAndConsumesDelimiters(t *testing.T) {
	for _, delimiter := range "!#$%&()*+,-./:;<=>?@[\\]^`{|}~" {
		t.Run(string(delimiter), func(t *testing.T) {
			lexer := NewLexer(" " + string(delimiter) + "_next")
			if !lexer.MatchDelimiter(byte(delimiter)) {
				t.Fatalf("MatchDelimiter(%q) = false, want true", delimiter)
			}
			if !lexer.MatchDelimiter(byte(delimiter)) {
				t.Fatal("MatchDelimiter() consumed the delimiter")
			}
			if err := lexer.EatDelimiter(byte(delimiter)); err != nil {
				t.Fatalf("EatDelimiter(%q) error = %v", delimiter, err)
			}
			if !lexer.MatchIdentifier() {
				t.Fatal("identifier after delimiter was not preserved")
			}
		})
	}
}

func TestLexerRejectsMismatchedDelimiterWithoutConsumingIt(t *testing.T) {
	lexer := NewLexer("  ,")
	if lexer.MatchDelimiter(';') {
		t.Fatal("MatchDelimiter(';') = true for a comma token")
	}
	if err := lexer.EatDelimiter(';'); err == nil || !strings.Contains(err.Error(), "byte 2") {
		t.Fatalf("EatDelimiter(';') error = %v, want byte offset 2", err)
	}
	if !lexer.MatchDelimiter(',') {
		t.Fatal("mismatched delimiter attempt consumed the comma")
	}
}

func TestLexerRejectsNonDelimitersWithoutConsumingThem(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t", byteOffset: 2},
		{name: "identifier", input: " word", byteOffset: 1},
		{name: "integer", input: "42"},
		{name: "string", input: "'value'"},
		{name: "double-quoted", input: `"value"`},
		{name: "control-byte", input: "\x01"},
		{name: "non-ASCII", input: "é"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lexer := NewLexer(test.input)
			if lexer.MatchDelimiter(',') {
				t.Fatalf("MatchDelimiter(',') = true for %q", test.input)
			}
			for i := 0; i < 2; i++ {
				if err := lexer.EatDelimiter(','); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("EatDelimiter(',') error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
		})
	}
}

func TestLexerNilReceiverIsSafeForDelimiters(t *testing.T) {
	var lexer *Lexer
	if lexer.MatchDelimiter(',') {
		t.Fatal("nil MatchDelimiter(',') = true, want false")
	}
	if err := lexer.EatDelimiter(','); err == nil {
		t.Fatal("nil EatDelimiter(',') error = nil")
	}
}
