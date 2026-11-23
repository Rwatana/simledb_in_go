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
