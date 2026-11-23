package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesSelectQueryAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser("  SeLeCt name, age from student, course where age=18 and active=1 order")
	data, err := parser.ParseQuery()
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if got, want := data.String(), "select name, age from student, course where age=18 and active=1"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got, want := strings.Join(data.Fields(), ","), "name,age"; got != want {
		t.Errorf("Fields() = %q, want %q", got, want)
	}
	if got, want := strings.Join(data.Tables(), ","), "student,course"; got != want {
		t.Errorf("Tables() = %q, want %q", got, want)
	}
	if got, want := data.Predicate().String(), "age=18 and active=1"; got != want {
		t.Errorf("Predicate().String() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchKeyword("order") {
		t.Fatal("ParseQuery() consumed the following keyword")
	}
}

func TestParserParsesSelectQueryWithoutPredicate(t *testing.T) {
	parser := NewParser("select name from student")
	data, err := parser.ParseQuery()
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if got, want := data.String(), "select name from student"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("Predicate().String() = %q, want empty", got)
	}
}

func TestParserReportsSelectQueryErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing select keyword", input: "name from student", context: "parse query SELECT keyword", byteOffset: 0},
		{name: "missing field", input: "select from student", context: "parse query field list", byteOffset: len("select ")},
		{name: "missing from keyword", input: "select name", context: "parse query FROM keyword", byteOffset: len("select name")},
		{name: "missing table", input: "select name from where", context: "parse query table list", byteOffset: len("select name from ")},
		{name: "missing field after comma", input: "select name, from student", context: "parse query field list", byteOffset: len("select name, ")},
		{name: "missing table after comma", input: "select name from student, where", context: "parse query table list", byteOffset: len("select name from student, ")},
		{name: "incomplete predicate term", input: "select name from student where age = ", context: "parse query WHERE predicate", byteOffset: len("select name from student where age = ")},
		{name: "incomplete predicate conjunction", input: "select name from student where age=1 and", context: "parse query WHERE predicate", byteOffset: len("select name from student where age=1 and")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseQuery()
			if err == nil {
				t.Fatal("ParseQuery() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseQuery() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsSelectQuery(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseQuery()
	if err == nil {
		t.Fatal("ParseQuery() error = nil for nil receiver")
	}
	if got := data.String(); got != "<unset-query>" {
		t.Errorf("ParseQuery() data = %q, want unset query", got)
	}
}
