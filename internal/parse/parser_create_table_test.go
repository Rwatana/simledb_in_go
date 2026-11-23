package parse

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestParserParsesCreateTableAndPreservesFollowingToken(t *testing.T) {
	parser := NewParser(" CrEaTe TaBlE Student (ID int, Name varchar(24)) returning")
	data, err := parser.ParseCreateTable()
	if err != nil {
		t.Fatalf("ParseCreateTable() error = %v", err)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	schema := data.Schema()
	if got, want := schema.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, want)
	}
	if got, ok := schema.Field("id"); !ok || got.Type != record.IntegerField || got.Length != 0 {
		t.Errorf("Schema().Field(id) = (%+v, %t), want integer field", got, ok)
	}
	if got, ok := schema.Field("name"); !ok || got.Type != record.VarcharField || got.Length != 24 {
		t.Errorf("Schema().Field(name) = (%+v, %t), want varchar(24)", got, ok)
	}
	if !parser.lexer.MatchKeyword("returning") {
		t.Fatal("ParseCreateTable() consumed the token following the command")
	}
}

func TestParserReportsCreateTableErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "missing CREATE keyword", input: "select id from student", context: "parse create table CREATE keyword", byteOffset: 0},
		{name: "missing TABLE keyword", input: "create student (id int)", context: "parse create table TABLE keyword", byteOffset: len("create ")},
		{name: "missing table name", input: "create table", context: "parse create table name", byteOffset: len("create table")},
		{name: "missing opening delimiter", input: "create table student id int)", context: "parse create table opening delimiter", byteOffset: len("create table student ")},
		{name: "missing first field", input: "create table student (", context: "parse create table field definitions", byteOffset: len("create table student (")},
		{name: "missing field type", input: "create table student (id)", context: "parse create table field type", byteOffset: len("create table student (id")},
		{name: "unsupported field type", input: "create table student (id text)", context: "parse create table field type", byteOffset: len("create table student (id ")},
		{name: "missing varchar opening delimiter", input: "create table student (name varchar 12)", context: "parse create table varchar opening delimiter", byteOffset: len("create table student (name varchar ")},
		{name: "invalid varchar length", input: "create table student (name varchar(x))", context: "parse create table varchar length", byteOffset: len("create table student (name varchar(")},
		{name: "missing varchar closing delimiter", input: "create table student (name varchar(12", context: "parse create table varchar closing delimiter", byteOffset: len("create table student (name varchar(12")},
		{name: "missing field after comma", input: "create table student (id int,)", context: "parse create table field definitions", byteOffset: len("create table student (id int,")},
		{name: "missing closing delimiter", input: "create table student (id int", context: "parse create table closing delimiter", byteOffset: len("create table student (id int")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseCreateTable()
			if err == nil {
				t.Fatal("ParseCreateTable() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseCreateTable() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsCreateTable(t *testing.T) {
	var parser *Parser
	data, err := parser.ParseCreateTable()
	if err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseCreateTable() error = %v, want nil-receiver error", err)
	}
	if got := data.TableName(); got != "" {
		t.Errorf("ParseCreateTable() table name = %q, want empty", got)
	}
	schema := data.Schema()
	if got := schema.Fields(); got != nil {
		t.Errorf("ParseCreateTable() fields = %v, want nil", got)
	}
}
