package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesCreateIndexAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser(" CrEaTe InDeX Student_Name_Idx ON Student (Name) returning")
	data, err := parser.ParseCreateIndex()
	if err != nil {
		t.Fatalf("ParseCreateIndex() error = %v", err)
	}
	if got, want := data.IndexName(), "student_name_idx"; got != want {
		t.Errorf("IndexName() = %q, want %q", got, want)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.FieldName(), "name"; got != want {
		t.Errorf("FieldName() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseCreateIndex() consumed the token following the command")
	}
}

func TestParserReportsCreateIndexErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing CREATE keyword", input: "select id from student", context: "parse create index CREATE keyword", byteOffset: 0},
		{name: "missing INDEX keyword", input: "create student_name_idx on student (name)", context: "parse create index INDEX keyword", byteOffset: len("create ")},
		{name: "missing index name", input: "create index", context: "parse create index name", byteOffset: len("create index")},
		{name: "missing ON keyword", input: "create index ix student (name)", context: "parse create index ON keyword", byteOffset: len("create index ix ")},
		{name: "missing table name", input: "create index ix on", context: "parse create index table name", byteOffset: len("create index ix on")},
		{name: "missing opening delimiter", input: "create index ix on student name)", context: "parse create index opening delimiter", byteOffset: len("create index ix on student ")},
		{name: "missing field name", input: "create index ix on student (", context: "parse create index field name", byteOffset: len("create index ix on student (")},
		{name: "missing closing delimiter", input: "create index ix on student (name", context: "parse create index closing delimiter", byteOffset: len("create index ix on student (name")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseCreateIndex()
			if err == nil {
				t.Fatal("ParseCreateIndex() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseCreateIndex() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsCreateIndex(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseCreateIndex()
	if err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseCreateIndex() error = %v, want nil-receiver error", err)
	}
	if got := data.IndexName(); got != "" {
		t.Errorf("ParseCreateIndex() index name = %q, want empty", got)
	}
	if got := data.TableName(); got != "" {
		t.Errorf("ParseCreateIndex() table name = %q, want empty", got)
	}
	if got := data.FieldName(); got != "" {
		t.Errorf("ParseCreateIndex() field name = %q, want empty", got)
	}
}
