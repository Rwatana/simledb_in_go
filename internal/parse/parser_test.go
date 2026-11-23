package parse

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestParserParsesIntegerConstant(t *testing.T) {
	parser := NewParser(" 42 , 'next'")
	constant, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() error = %v", err)
	}
	value, ok := constant.AsInt()
	if !ok || value != 42 {
		t.Fatalf("ParseConstant() = %v, want integer constant 42", constant)
	}
	if !parser.lexer.MatchDelimiter(',') {
		t.Fatal("delimiter following the integer constant was consumed")
	}
}

func TestParserParsesStringConstantAndRetainsFollowingInput(t *testing.T) {
	parser := NewParser(" 'MiXeD''s value' 7")
	constant, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() error = %v", err)
	}
	value, ok := constant.AsString()
	if !ok || value != "MiXeD's value" {
		t.Fatalf("ParseConstant() = %v, want string constant %q", constant, "MiXeD's value")
	}
	if !parser.lexer.MatchIntConstant() {
		t.Fatal("integer following the string constant was consumed")
	}
	next, err := parser.ParseConstant()
	if err != nil {
		t.Fatalf("ParseConstant() for following integer error = %v", err)
	}
	if integer, ok := next.AsInt(); !ok || integer != 7 {
		t.Fatalf("ParseConstant() for following integer = %v, want 7", next)
	}
}

func TestParserRejectsInvalidConstantWithoutConsuming(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		byteOffset int
	}{
		{name: "empty", input: ""},
		{name: "whitespace", input: " \t", byteOffset: 2},
		{name: "identifier", input: " name", byteOffset: 1},
		{name: "delimiter", input: ","},
		{name: "unterminated string", input: "'broken"},
		{name: "integer overflow", input: "2147483648"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser(test.input)
			for i := 0; i < 2; i++ {
				if _, err := parser.ParseConstant(); err == nil || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
					t.Fatalf("ParseConstant() error = %v, want stable byte offset %d", err, test.byteOffset)
				}
			}
		})
	}
}

func TestParserNilReceiverRejectsConstant(t *testing.T) {
	var parser *Parser
	constant, err := parser.ParseConstant()
	if err == nil {
		t.Fatal("ParseConstant() error = nil for nil receiver")
	}
	if _, ok := constant.AsInt(); ok {
		t.Fatalf("ParseConstant() integer constant = %v, want unset zero value", constant)
	}
	if _, ok := constant.AsString(); ok {
		t.Fatalf("ParseConstant() string constant = %v, want unset zero value", constant)
	}
}

func TestParserParseInsertPreservesOrderedFieldsValuesAndFollowingInput(t *testing.T) {
	parser := NewParser("InSeRt InTo StUdEnT (NaMe, age) VaLuEs ('Ada', 37) where")
	data, err := parser.ParseInsert()
	if err != nil {
		t.Fatalf("ParseInsert() error = %v", err)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.Fields(), []string{"name", "age"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Fields() = %v, want %v", got, want)
	}
	if got, want := data.Values(), []query.Constant{query.NewStringConstant("Ada"), query.NewIntConstant(37)}; !reflect.DeepEqual(got, want) {
		t.Errorf("Values() = %v, want %v", got, want)
	}
	if !parser.lexer.MatchKeyword("where") {
		t.Fatal("keyword following INSERT was consumed")
	}
}

func TestParserParseInsertRejectsMalformedSyntaxWithContext(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		context string
	}{
		{name: "insert keyword", input: "select x", context: "INSERT keyword"},
		{name: "into keyword", input: "insert student", context: "INTO keyword"},
		{name: "table name", input: "insert into", context: "table name"},
		{name: "field-list opening delimiter", input: "insert into student name) values (1)", context: "field-list opening delimiter"},
		{name: "first field", input: "insert into student () values (1)", context: "field list"},
		{name: "field after comma", input: "insert into student (name, ) values (1)", context: "field list"},
		{name: "field-list closing delimiter", input: "insert into student (name values (1)", context: "field-list closing delimiter"},
		{name: "values keyword", input: "insert into student (name) 1", context: "VALUES keyword"},
		{name: "value-list opening delimiter", input: "insert into student (name) values 1", context: "value-list opening delimiter"},
		{name: "first value", input: "insert into student (name) values ()", context: "value list"},
		{name: "value after comma", input: "insert into student (name, age) values (1,)", context: "value list"},
		{name: "value-list closing delimiter", input: "insert into student (name) values (1", context: "value-list closing delimiter"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseInsert()
			if err == nil || !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte ") {
				t.Fatalf("ParseInsert() error = %v, want %q context and a byte offset", err, test.context)
			}
		})
	}
}

func TestParserParseInsertNilReceiver(t *testing.T) {
	var parser *Parser
	if _, err := parser.ParseInsert(); err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseInsert() error = %v, want nil-receiver error", err)
	}
}
