package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesUpdateAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser(" UpDaTe StUdEnT SeT Score = grade WHERE active=1 and age=18 returning")
	data, err := parser.ParseUpdate()
	if err != nil {
		t.Fatalf("ParseUpdate() error = %v", err)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.TargetField(), "score"; got != want {
		t.Errorf("TargetField() = %q, want %q", got, want)
	}
	if got, want := data.NewValue().String(), "grade"; got != want {
		t.Errorf("NewValue().String() = %q, want %q", got, want)
	}
	if got, want := data.Predicate().String(), "active=1 and age=18"; got != want {
		t.Errorf("Predicate().String() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseUpdate() consumed the following keyword")
	}
}

func TestParserParsesUpdateWithoutPredicate(t *testing.T) {
	parser := NewParser("update student set score = 100 returning")
	data, err := parser.ParseUpdate()
	if err != nil {
		t.Fatalf("ParseUpdate() error = %v", err)
	}
	if got, want := data.NewValue().String(), "100"; got != want {
		t.Errorf("NewValue().String() = %q, want %q", got, want)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("Predicate().String() = %q, want empty", got)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseUpdate() consumed the following keyword")
	}
}

func TestParserReportsUpdateErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing update keyword", input: "select student", context: "parse update UPDATE keyword", byteOffset: 0},
		{name: "missing table name", input: "update", context: "parse update table name", byteOffset: len("update")},
		{name: "missing set keyword", input: "update student score", context: "parse update SET keyword", byteOffset: len("update student ")},
		{name: "missing target field", input: "update student set", context: "parse update target field", byteOffset: len("update student set")},
		{name: "missing equals delimiter", input: "update student set score 1", context: "parse update equals delimiter", byteOffset: len("update student set score ")},
		{name: "missing new expression", input: "update student set score =", context: "parse update new expression", byteOffset: len("update student set score =")},
		{name: "missing predicate term", input: "update student set score = 1 where", context: "parse update WHERE predicate", byteOffset: len("update student set score = 1 where")},
		{name: "incomplete predicate conjunction", input: "update student set score = 1 where active=1 and", context: "parse update WHERE predicate", byteOffset: len("update student set score = 1 where active=1 and")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseUpdate()
			if err == nil {
				t.Fatal("ParseUpdate() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseUpdate() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsUpdate(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseUpdate()
	if err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseUpdate() error = %v, want nil-receiver error", err)
	}
	if got := data.TableName(); got != "" {
		t.Errorf("ParseUpdate() table name = %q, want empty", got)
	}
	if got := data.TargetField(); got != "" {
		t.Errorf("ParseUpdate() target field = %q, want empty", got)
	}
}
