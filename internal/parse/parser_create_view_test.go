package parse

import (
	"strconv"
	"strings"
	"testing"
)

func TestParserParsesCreateViewAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser(" CrEaTe ViEw Active_Students As select ID, name from student where active=1 returning")
	data, err := parser.ParseCreateView()
	if err != nil {
		t.Fatalf("ParseCreateView() error = %v", err)
	}
	if got, want := data.ViewName(), "active_students"; got != want {
		t.Errorf("ViewName() = %q, want %q", got, want)
	}
	if got, want := data.ViewDefinition(), "select id, name from student where active=1"; got != want {
		t.Errorf("ViewDefinition() = %q, want %q", got, want)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseCreateView() consumed the token following the query")
	}
}

func TestParserReportsCreateViewErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing CREATE keyword", input: "select id from student", context: "parse create view CREATE keyword", byteOffset: 0},
		{name: "missing VIEW keyword", input: "create student", context: "parse create view VIEW keyword", byteOffset: len("create ")},
		{name: "missing view name", input: "create view", context: "parse create view name", byteOffset: len("create view")},
		{name: "missing AS keyword", input: "create view active_students select id from student", context: "parse create view AS keyword", byteOffset: len("create view active_students ")},
		{name: "missing SELECT query", input: "create view active_students as from student", context: "parse create view definition", byteOffset: len("create view active_students as ")},
		{name: "incomplete query predicate", input: "create view active_students as select id from student where", context: "parse create view definition", byteOffset: len("create view active_students as select id from student where")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseCreateView()
			if err == nil {
				t.Fatal("ParseCreateView() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseCreateView() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsCreateView(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseCreateView()
	if err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseCreateView() error = %v, want nil-receiver error", err)
	}
	if got := data.ViewName(); got != "" {
		t.Errorf("ParseCreateView() view name = %q, want empty", got)
	}
	if got := data.ViewDefinition(); got != "<unset-query>" {
		t.Errorf("ParseCreateView() definition = %q, want <unset-query>", got)
	}
}
