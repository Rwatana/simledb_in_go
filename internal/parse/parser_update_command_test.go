package parse

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParserDispatchesUpdateCommandsAndPreservesFollowingToken(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		commandType string
	}{
		{name: "insert", input: "insert into student (id) values (1) returning", commandType: "InsertData"},
		{name: "delete", input: "delete from student where id=1 returning", commandType: "DeleteData"},
		{name: "update", input: "update student set id=2 returning", commandType: "ModifyData"},
		{name: "create table", input: "create table student (id int) returning", commandType: "CreateTableData"},
		{name: "create view", input: "create view active as select id from student returning", commandType: "CreateViewData"},
		{name: "create index", input: "create index student_id_idx on student (id) returning", commandType: "CreateIndexData"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := NewParser(test.input)
			command, err := parser.ParseUpdateCommand()
			if err != nil {
				t.Fatalf("ParseUpdateCommand() error = %v", err)
			}
			if got := reflect.TypeOf(command).Name(); got != test.commandType {
				t.Errorf("ParseUpdateCommand() type = %s, want %s", got, test.commandType)
			}
			if !parser.lexer.MatchKeyword("returning") {
				t.Fatal("ParseUpdateCommand() consumed the token following the command")
			}
		})
	}
}

func TestParserReportsUpdateCommandErrorsWithContextAndOffsets(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		context    string
		byteOffset int
	}{
		{name: "malformed INSERT", input: "insert", context: "parse update command INSERT", byteOffset: len("insert")},
		{name: "malformed DELETE", input: "delete", context: "parse update command DELETE", byteOffset: len("delete")},
		{name: "malformed UPDATE", input: "update", context: "parse update command UPDATE", byteOffset: len("update")},
		{name: "malformed CREATE INDEX", input: "create", context: "parse update command CREATE INDEX", byteOffset: len("create")},
		{name: "malformed CREATE TABLE", input: "create table student", context: "parse update command CREATE TABLE", byteOffset: len("create table student")},
		{name: "malformed CREATE VIEW", input: "create view active as", context: "parse update command CREATE VIEW", byteOffset: len("create view active as")},
		{name: "malformed CREATE INDEX body", input: "create index student_idx on", context: "parse update command CREATE INDEX", byteOffset: len("create index student_idx on")},
		{name: "unrecognized CREATE subtype", input: "create materialized", context: "parse update command CREATE INDEX", byteOffset: len("create ")},
		{name: "unknown statement", input: "select id from student", context: "parse update command CREATE keyword", byteOffset: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParser(test.input).ParseUpdateCommand()
			if err == nil {
				t.Fatal("ParseUpdateCommand() error = nil")
			}
			if !strings.Contains(err.Error(), test.context) || !strings.Contains(err.Error(), "byte "+strconv.Itoa(test.byteOffset)) {
				t.Fatalf("ParseUpdateCommand() error = %v, want context %q and byte offset %d", err, test.context, test.byteOffset)
			}
		})
	}
}

func TestParserNilReceiverRejectsUpdateCommand(t *testing.T) {
	var parser *Parser
	command, err := parser.ParseUpdateCommand()
	if err == nil || !strings.Contains(err.Error(), "parser is nil") {
		t.Fatalf("ParseUpdateCommand() error = %v, want nil-receiver error", err)
	}
	if command != nil {
		t.Errorf("ParseUpdateCommand() value = %v, want nil", command)
	}
}
