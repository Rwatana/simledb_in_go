package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesDeleteWithPredicateAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser("  DeLeTe FrOm Student where age=18 and active=1 returning")
	data, err := parser.ParseDelete()
	if err != nil {
		t.Fatalf("ParseDelete() error = %v", err)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.Predicate().String(), "age=18 and active=1"; got != want {
		t.Errorf("Predicate().String() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseDelete() consumed the following keyword")
	}
}

func TestParserParsesDeleteWithoutPredicate(t *testing.T) {
	parser := NewParser("delete from student returning")
	data, err := parser.ParseDelete()
	if err != nil {
		t.Fatalf("ParseDelete() error = %v", err)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("Predicate().String() = %q, want empty", got)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseDelete() consumed the following keyword")
	}
}

func TestParserReportsDeleteErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing delete keyword", input: "from student", context: "parse delete DELETE keyword", byteOffset: 0},
		{name: "missing from keyword", input: "delete student", context: "parse delete FROM keyword", byteOffset: len("delete ")},
		{name: "missing table name", input: "delete from where", context: "parse delete table name", byteOffset: len("delete from ")},
		{name: "missing predicate", input: "delete from student where", context: "parse delete WHERE predicate", byteOffset: len("delete from student where")},
		{name: "incomplete predicate conjunction", input: "delete from student where age=1 and", context: "parse delete WHERE predicate", byteOffset: len("delete from student where age=1 and")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseDelete()
			if err == nil {
				t.Fatal("ParseDelete() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseDelete() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsDelete(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseDelete()
	if err == nil {
		t.Fatal("ParseDelete() error = nil for nil receiver")
	}
	if got := data.TableName(); got != "" {
		t.Errorf("ParseDelete() data table = %q, want empty", got)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("ParseDelete() predicate = %q, want empty", got)
	}
}
