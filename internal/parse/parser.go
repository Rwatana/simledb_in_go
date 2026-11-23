package parse

import (
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// Parser converts SQL text into query and update values.
type Parser struct {
	lexer *Lexer
}

// NewParser creates a parser positioned before the first token in input.
func NewParser(input string) *Parser {
	return &Parser{lexer: NewLexer(input)}
}

// ParseField reads and returns the next identifier field name.
func (parser *Parser) ParseField() (string, error) {
	if parser == nil {
		return "", fmt.Errorf("parse field: parser is nil")
	}
	field, err := parser.lexer.EatIdentifier()
	if err != nil {
		return "", fmt.Errorf("parse field: %w", err)
	}
	return field, nil
}

// ParseExpression reads a field reference or a literal query expression.
func (parser *Parser) ParseExpression() (query.Expression, error) {
	if parser == nil {
		return query.Expression{}, fmt.Errorf("parse expression: parser is nil")
	}
	if parser.lexer.MatchIdentifier() {
		// A successful match guarantees this consume succeeds before the lexer changes.
		field, _ := parser.lexer.EatIdentifier()
		return query.NewFieldExpression(field), nil
	}
	constant, err := parser.ParseConstant()
	if err != nil {
		return query.Expression{}, fmt.Errorf("parse expression: %w", err)
	}
	return query.NewConstantExpression(constant), nil
}

// ParseTerm reads two expressions separated by the SQL equality delimiter.
func (parser *Parser) ParseTerm() (query.Term, error) {
	if parser == nil {
		return query.Term{}, fmt.Errorf("parse term: parser is nil")
	}
	lhs, err := parser.ParseExpression()
	if err != nil {
		return query.Term{}, fmt.Errorf("parse term left expression: %w", err)
	}
	if err := parser.lexer.EatDelimiter('='); err != nil {
		return query.Term{}, fmt.Errorf("parse term equals delimiter: %w", err)
	}
	rhs, err := parser.ParseExpression()
	if err != nil {
		return query.Term{}, fmt.Errorf("parse term right expression: %w", err)
	}
	return query.NewTerm(lhs, rhs), nil
}

// ParsePredicate reads one term followed by any conjunctions with the SQL and keyword.
func (parser *Parser) ParsePredicate() (query.Predicate, error) {
	if parser == nil {
		return query.Predicate{}, fmt.Errorf("parse predicate: parser is nil")
	}
	term, err := parser.ParseTerm()
	if err != nil {
		return query.Predicate{}, fmt.Errorf("parse predicate term: %w", err)
	}
	predicate := query.NewPredicateWithTerm(term)
	if !parser.lexer.MatchKeyword("and") {
		return predicate, nil
	}
	// A successful match guarantees this consume succeeds before the lexer changes.
	_ = parser.lexer.EatKeyword("and")
	rest, err := parser.ParsePredicate()
	if err != nil {
		return query.Predicate{}, fmt.Errorf("parse predicate after AND: %w", err)
	}
	return predicate.ConjoinWith(rest), nil
}

// ParseQuery reads a SELECT field list, table list, and optional predicate.
func (parser *Parser) ParseQuery() (QueryData, error) {
	if parser == nil {
		return QueryData{}, fmt.Errorf("parse query: parser is nil")
	}
	if err := parser.lexer.EatKeyword("select"); err != nil {
		return QueryData{}, fmt.Errorf("parse query SELECT keyword: %w", err)
	}
	fields, err := parser.parseQueryNameList("field list")
	if err != nil {
		return QueryData{}, err
	}
	if err := parser.lexer.EatKeyword("from"); err != nil {
		return QueryData{}, fmt.Errorf("parse query FROM keyword: %w", err)
	}
	tables, err := parser.parseQueryNameList("table list")
	if err != nil {
		return QueryData{}, err
	}
	var predicate query.Predicate
	if parser.lexer.MatchKeyword("where") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("where")
		predicate, err = parser.ParsePredicate()
		if err != nil {
			return QueryData{}, fmt.Errorf("parse query WHERE predicate: %w", err)
		}
	}
	return NewQueryData(fields, tables, predicate), nil
}

// ParseDelete reads a DELETE table name and optional predicate.
func (parser *Parser) ParseDelete() (DeleteData, error) {
	if parser == nil {
		return DeleteData{}, fmt.Errorf("parse delete: parser is nil")
	}
	if err := parser.lexer.EatKeyword("delete"); err != nil {
		return DeleteData{}, fmt.Errorf("parse delete DELETE keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("from"); err != nil {
		return DeleteData{}, fmt.Errorf("parse delete FROM keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return DeleteData{}, fmt.Errorf("parse delete table name: %w", err)
	}
	var predicate query.Predicate
	if parser.lexer.MatchKeyword("where") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("where")
		predicate, err = parser.ParsePredicate()
		if err != nil {
			return DeleteData{}, fmt.Errorf("parse delete WHERE predicate: %w", err)
		}
	}
	return NewDeleteData(tableName, predicate), nil
}

// ParseInsert reads an INSERT target, field list, and matching constant list.
func (parser *Parser) ParseInsert() (InsertData, error) {
	if parser == nil {
		return InsertData{}, fmt.Errorf("parse insert: parser is nil")
	}
	if err := parser.lexer.EatKeyword("insert"); err != nil {
		return InsertData{}, fmt.Errorf("parse insert INSERT keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("into"); err != nil {
		return InsertData{}, fmt.Errorf("parse insert INTO keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return InsertData{}, fmt.Errorf("parse insert table name: %w", err)
	}
	if err := parser.lexer.EatDelimiter('('); err != nil {
		return InsertData{}, fmt.Errorf("parse insert field-list opening delimiter: %w", err)
	}
	fields, err := parser.parseInsertFieldList()
	if err != nil {
		return InsertData{}, err
	}
	if err := parser.lexer.EatDelimiter(')'); err != nil {
		return InsertData{}, fmt.Errorf("parse insert field-list closing delimiter: %w", err)
	}
	if err := parser.lexer.EatKeyword("values"); err != nil {
		return InsertData{}, fmt.Errorf("parse insert VALUES keyword: %w", err)
	}
	if err := parser.lexer.EatDelimiter('('); err != nil {
		return InsertData{}, fmt.Errorf("parse insert value-list opening delimiter: %w", err)
	}
	values, err := parser.parseInsertConstantList()
	if err != nil {
		return InsertData{}, err
	}
	if err := parser.lexer.EatDelimiter(')'); err != nil {
		return InsertData{}, fmt.Errorf("parse insert value-list closing delimiter: %w", err)
	}
	return NewInsertData(tableName, fields, values), nil
}

// ParseUpdate reads an UPDATE target, new-value expression, and optional predicate.
func (parser *Parser) ParseUpdate() (ModifyData, error) {
	if parser == nil {
		return ModifyData{}, fmt.Errorf("parse update: parser is nil")
	}
	if err := parser.lexer.EatKeyword("update"); err != nil {
		return ModifyData{}, fmt.Errorf("parse update UPDATE keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return ModifyData{}, fmt.Errorf("parse update table name: %w", err)
	}
	if err := parser.lexer.EatKeyword("set"); err != nil {
		return ModifyData{}, fmt.Errorf("parse update SET keyword: %w", err)
	}
	targetField, err := parser.ParseField()
	if err != nil {
		return ModifyData{}, fmt.Errorf("parse update target field: %w", err)
	}
	if err := parser.lexer.EatDelimiter('='); err != nil {
		return ModifyData{}, fmt.Errorf("parse update equals delimiter: %w", err)
	}
	newValue, err := parser.ParseExpression()
	if err != nil {
		return ModifyData{}, fmt.Errorf("parse update new expression: %w", err)
	}
	var predicate query.Predicate
	if parser.lexer.MatchKeyword("where") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("where")
		predicate, err = parser.ParsePredicate()
		if err != nil {
			return ModifyData{}, fmt.Errorf("parse update WHERE predicate: %w", err)
		}
	}
	return NewModifyData(tableName, targetField, newValue, predicate), nil
}

// ParseCreateView reads a CREATE VIEW name and its SELECT query definition.
func (parser *Parser) ParseCreateView() (CreateViewData, error) {
	if parser == nil {
		return CreateViewData{}, fmt.Errorf("parse create view: parser is nil")
	}
	if err := parser.lexer.EatKeyword("create"); err != nil {
		return CreateViewData{}, fmt.Errorf("parse create view CREATE keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("view"); err != nil {
		return CreateViewData{}, fmt.Errorf("parse create view VIEW keyword: %w", err)
	}
	viewName, err := parser.ParseField()
	if err != nil {
		return CreateViewData{}, fmt.Errorf("parse create view name: %w", err)
	}
	if err := parser.lexer.EatKeyword("as"); err != nil {
		return CreateViewData{}, fmt.Errorf("parse create view AS keyword: %w", err)
	}
	definition, err := parser.ParseQuery()
	if err != nil {
		return CreateViewData{}, fmt.Errorf("parse create view definition: %w", err)
	}
	return NewCreateViewData(viewName, definition), nil
}

// ParseCreateIndex reads the index, table, and field names in a CREATE INDEX statement.
func (parser *Parser) ParseCreateIndex() (CreateIndexData, error) {
	if parser == nil {
		return CreateIndexData{}, fmt.Errorf("parse create index: parser is nil")
	}
	if err := parser.lexer.EatKeyword("create"); err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index CREATE keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("index"); err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index INDEX keyword: %w", err)
	}
	indexName, err := parser.ParseField()
	if err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index name: %w", err)
	}
	if err := parser.lexer.EatKeyword("on"); err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index ON keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index table name: %w", err)
	}
	if err := parser.lexer.EatDelimiter('('); err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index opening delimiter: %w", err)
	}
	fieldName, err := parser.ParseField()
	if err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index field name: %w", err)
	}
	if err := parser.lexer.EatDelimiter(')'); err != nil {
		return CreateIndexData{}, fmt.Errorf("parse create index closing delimiter: %w", err)
	}
	return NewCreateIndexData(indexName, tableName, fieldName), nil
}

// ParseCreateTable reads a table name and its ordered field definitions.
func (parser *Parser) ParseCreateTable() (CreateTableData, error) {
	if parser == nil {
		return CreateTableData{}, fmt.Errorf("parse create table: parser is nil")
	}
	if err := parser.lexer.EatKeyword("create"); err != nil {
		return CreateTableData{}, fmt.Errorf("parse create table CREATE keyword: %w", err)
	}
	if err := parser.lexer.EatKeyword("table"); err != nil {
		return CreateTableData{}, fmt.Errorf("parse create table TABLE keyword: %w", err)
	}
	tableName, err := parser.ParseField()
	if err != nil {
		return CreateTableData{}, fmt.Errorf("parse create table name: %w", err)
	}
	if err := parser.lexer.EatDelimiter('('); err != nil {
		return CreateTableData{}, fmt.Errorf("parse create table opening delimiter: %w", err)
	}
	schema, err := parser.parseCreateTableFieldDefinitions()
	if err != nil {
		return CreateTableData{}, err
	}
	if err := parser.lexer.EatDelimiter(')'); err != nil {
		return CreateTableData{}, fmt.Errorf("parse create table closing delimiter: %w", err)
	}
	return NewCreateTableData(tableName, schema), nil
}

func (parser *Parser) parseCreateTableFieldDefinitions() (record.Schema, error) {
	var schema record.Schema
	for {
		fieldName, err := parser.ParseField()
		if err != nil {
			return record.Schema{}, fmt.Errorf("parse create table field definitions: %w", err)
		}
		definition, err := parser.parseCreateTableFieldType()
		if err != nil {
			return record.Schema{}, err
		}
		schema.AddField(fieldName, definition.Type, definition.Length)
		if !parser.lexer.MatchDelimiter(',') {
			return schema, nil
		}
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatDelimiter(',')
	}
}

func (parser *Parser) parseCreateTableFieldType() (record.FieldDefinition, error) {
	if parser.lexer.MatchKeyword("int") {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatKeyword("int")
		return record.FieldDefinition{Type: record.IntegerField}, nil
	}
	if err := parser.lexer.EatKeyword("varchar"); err != nil {
		return record.FieldDefinition{}, fmt.Errorf("parse create table field type: %w", err)
	}
	if err := parser.lexer.EatDelimiter('('); err != nil {
		return record.FieldDefinition{}, fmt.Errorf("parse create table varchar opening delimiter: %w", err)
	}
	length, err := parser.lexer.EatIntConstant()
	if err != nil {
		return record.FieldDefinition{}, fmt.Errorf("parse create table varchar length: %w", err)
	}
	if err := parser.lexer.EatDelimiter(')'); err != nil {
		return record.FieldDefinition{}, fmt.Errorf("parse create table varchar closing delimiter: %w", err)
	}
	return record.FieldDefinition{Type: record.VarcharField, Length: int(length)}, nil
}

func (parser *Parser) parseInsertFieldList() ([]string, error) {
	field, err := parser.ParseField()
	if err != nil {
		return nil, fmt.Errorf("parse insert field list: %w", err)
	}
	fields := []string{field}
	for parser.lexer.MatchDelimiter(',') {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatDelimiter(',')
		field, err := parser.ParseField()
		if err != nil {
			return nil, fmt.Errorf("parse insert field list: %w", err)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func (parser *Parser) parseInsertConstantList() ([]query.Constant, error) {
	value, err := parser.ParseConstant()
	if err != nil {
		return nil, fmt.Errorf("parse insert value list: %w", err)
	}
	values := []query.Constant{value}
	for parser.lexer.MatchDelimiter(',') {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatDelimiter(',')
		value, err := parser.ParseConstant()
		if err != nil {
			return nil, fmt.Errorf("parse insert value list: %w", err)
		}
		values = append(values, value)
	}
	return values, nil
}

func (parser *Parser) parseQueryNameList(kind string) ([]string, error) {
	name, err := parser.ParseField()
	if err != nil {
		return nil, fmt.Errorf("parse query %s: %w", kind, err)
	}
	names := []string{name}
	for parser.lexer.MatchDelimiter(',') {
		// A successful match guarantees this consume succeeds before the lexer changes.
		_ = parser.lexer.EatDelimiter(',')
		name, err := parser.ParseField()
		if err != nil {
			return nil, fmt.Errorf("parse query %s: %w", kind, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// ParseConstant reads a string or non-negative integer query constant.
func (parser *Parser) ParseConstant() (query.Constant, error) {
	if parser == nil {
		return query.Constant{}, fmt.Errorf("parse constant: parser is nil")
	}
	if parser.lexer.MatchStringConstant() {
		// The preceding match guarantees this consume succeeds while the lexer state is unchanged.
		value, _ := parser.lexer.EatStringConstant()
		return query.NewStringConstant(value), nil
	}
	value, err := parser.lexer.EatIntConstant()
	if err != nil {
		return query.Constant{}, fmt.Errorf("parse integer constant: %w", err)
	}
	return query.NewIntConstant(value), nil
}
