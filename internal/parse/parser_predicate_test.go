package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesSingleTermPredicateAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser("age = 1, next")
	predicate, err := parser.ParsePredicate()
	if err != nil {
		t.Fatalf("ParsePredicate() error = %v", err)
	}
	if got, want := predicate.String(), "age=1"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter after predicate was consumed")
	}
}

func TestParserParsesAndPredicateTermsInOrder(t *testing.T) {
	parser := NewParser("age=1 and name='Ada' and active=2 where")
	predicate, err := parser.ParsePredicate()
	if err != nil {
		t.Fatalf("ParsePredicate() error = %v", err)
	}
	if got, want := predicate.String(), "age=1 and name=Ada and active=2"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	for field, want := range map[string]string{"age": "1", "name": "Ada", "active": "2"} {
		constant, ok := predicate.EquatesWithConstant(field)
		if !ok || constant.String() != want {
			t.Fatalf("EquatesWithConstant(%q) = (%q, %t), want (%q, true)", field, constant.String(), ok, want)
		}
	}
	if !parser.lexer.MatchKeyword("where") {
		t.Fatal("keyword after predicate was consumed")
	}
}

func TestParserReportsPredicateErrorsWithTokenOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "invalid first term", input: "select=1", context: "parse predicate term", byteOffset: 0},
		{name: "invalid term after and", input: "a=1 and b =", context: "parse predicate after AND", byteOffset: len("a=1 and b =")},
		{name: "missing term after and", input: "age=1 and", context: "parse predicate after AND", byteOffset: len("age=1 and")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParsePredicate()
			if err == nil {
				t.Fatal("ParsePredicate() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParsePredicate() error = %v, want %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsPredicate(t *testing.T) {
	var parser *Parser
	predicate, err := parser.ParsePredicate()
	if err == nil {
		t.Fatal("ParsePredicate() error = nil for nil receiver")
	}
	if got := predicate.String(); got != "" {
		t.Fatalf("ParsePredicate() = %q, want zero predicate", got)
	}
}
