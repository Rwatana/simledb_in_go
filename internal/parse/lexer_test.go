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
