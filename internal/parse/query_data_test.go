package parse

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestQueryDataRetainsOrderedValuesAndRendersPredicate(t *testing.T) {
	fields := []string{"id", "name"}
	tables := []string{"customer", "orders"}
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"),
		query.NewConstantExpression(query.NewIntConstant(7)),
	))
	data := NewQueryData(fields, tables, predicate)

	fields[0] = "changed"
	tables[0] = "changed"
	if want := []string{"id", "name"}; !reflect.DeepEqual(data.Fields(), want) {
		t.Fatalf("Fields() = %v, want %v", data.Fields(), want)
	}
	if want := []string{"customer", "orders"}; !reflect.DeepEqual(data.Tables(), want) {
		t.Fatalf("Tables() = %v, want %v", data.Tables(), want)
	}
	if got := data.Predicate().String(); got != predicate.String() {
		t.Fatalf("Predicate().String() = %q, want %q", got, predicate.String())
	}
	if got, want := data.String(), "select id, name from customer, orders where id=7"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestQueryDataAccessorsReturnIndependentSlices(t *testing.T) {
	data := NewQueryData([]string{"id"}, []string{"customer"}, query.Predicate{})
	fields := data.Fields()
	tables := data.Tables()
	fields[0] = "other"
	tables[0] = "other"
	if got := data.String(); got != "select id from customer" {
		t.Fatalf("String() after accessor mutation = %q, want original values", got)
	}
}

func TestQueryDataWithoutPredicateOmitsWhereClause(t *testing.T) {
	data := NewQueryData([]string{"id"}, []string{"customer"}, query.Predicate{})
	if got, want := data.String(), "select id from customer"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestIncompleteQueryDataAndZeroValueRenderSafely(t *testing.T) {
	tests := []struct {
		name string
		data QueryData
	}{
		{name: "zero value"},
		{name: "empty fields", data: NewQueryData(nil, []string{"customer"}, query.Predicate{})},
		{name: "empty tables", data: NewQueryData([]string{"id"}, nil, query.Predicate{})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.data.String(); got != "<unset-query>" {
				t.Fatalf("String() = %q, want <unset-query>", got)
			}
		})
	}
}

func TestInsertDataRetainsOrderedFieldsAndValues(t *testing.T) {
	fields := []string{"name", "age"}
	values := []query.Constant{query.NewStringConstant("Ada"), query.NewIntConstant(37)}
	data := NewInsertData("student", fields, values)

	fields[0] = "changed"
	values[0] = query.NewStringConstant("changed")
	if got, want := data.TableName(), "student"; got != want {
		t.Fatalf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.Fields(), []string{"name", "age"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	if got, want := data.Values(), []query.Constant{query.NewStringConstant("Ada"), query.NewIntConstant(37)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Values() = %v, want %v", got, want)
	}
}

func TestInsertDataAccessorsReturnIndependentSlices(t *testing.T) {
	data := NewInsertData("student", []string{"name"}, []query.Constant{query.NewStringConstant("Ada")})
	fields := data.Fields()
	values := data.Values()
	fields[0] = "other"
	values[0] = query.NewStringConstant("other")

	if got := data.Fields()[0]; got != "name" {
		t.Errorf("Fields()[0] after accessor mutation = %q, want name", got)
	}
	if got := data.Values()[0]; got != query.NewStringConstant("Ada") {
		t.Errorf("Values()[0] after accessor mutation = %v, want Ada", got)
	}
}

func TestInsertDataZeroValueHasEmptyAccessors(t *testing.T) {
	var data InsertData
	if got := data.TableName(); got != "" {
		t.Errorf("TableName() = %q, want empty", got)
	}
	if got := data.Fields(); got != nil {
		t.Errorf("Fields() = %v, want nil", got)
	}
	if got := data.Values(); got != nil {
		t.Errorf("Values() = %v, want nil", got)
	}
}

func TestModifyDataStoresUpdateTargetValueAndPredicate(t *testing.T) {
	newValue := query.NewFieldExpression("grade")
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("active"),
		query.NewConstantExpression(query.NewIntConstant(1)),
	))
	data := NewModifyData("student", "score", newValue, predicate)

	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.TargetField(), "score"; got != want {
		t.Errorf("TargetField() = %q, want %q", got, want)
	}
	if got := data.NewValue(); got != newValue {
		t.Errorf("NewValue() = %v, want %v", got, newValue)
	}
	if got, want := data.Predicate().String(), "active=1"; got != want {
		t.Errorf("Predicate().String() = %q, want %q", got, want)
	}
}

func TestModifyDataZeroValueHasSafeAccessors(t *testing.T) {
	var data ModifyData
	if got := data.TableName(); got != "" {
		t.Errorf("TableName() = %q, want empty", got)
	}
	if got := data.TargetField(); got != "" {
		t.Errorf("TargetField() = %q, want empty", got)
	}
	if got := data.NewValue().String(); got != "<unset-expression>" {
		t.Errorf("NewValue().String() = %q, want <unset-expression>", got)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("Predicate().String() = %q, want empty", got)
	}
}

func TestCreateTableDataCopiesSchemaOnConstructionAndAccess(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 12)
	data := NewCreateTableData("student", schema)

	schema.AddIntegerField("source_mutation")
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	stored := data.Schema()
	if got, want := stored.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, want)
	}
	if got, want := stored.LengthOf("name"); !want || got != 12 {
		t.Errorf("Schema().LengthOf(name) = (%d, %t), want (12, true)", got, want)
	}

	stored.AddIntegerField("accessor_mutation")
	storedAgain := data.Schema()
	if got, want := storedAgain.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Schema().Fields() after accessor mutation = %v, want %v", got, want)
	}
}

func TestCreateTableDataZeroValueHasSafeAccessors(t *testing.T) {
	var data CreateTableData
	if got := data.TableName(); got != "" {
		t.Errorf("TableName() = %q, want empty", got)
	}
	schema := data.Schema()
	if got := schema.Fields(); got != nil {
		t.Errorf("Schema().Fields() = %v, want nil", got)
	}
}

func TestCreateViewDataStoresNameAndRenderedDefinition(t *testing.T) {
	queryData := NewQueryData(
		[]string{"id", "name"},
		[]string{"student"},
		query.NewPredicateWithTerm(query.NewTerm(
			query.NewFieldExpression("active"),
			query.NewConstantExpression(query.NewIntConstant(1)),
		)),
	)
	data := NewCreateViewData("active_students", queryData)

	if got, want := data.ViewName(), "active_students"; got != want {
		t.Errorf("ViewName() = %q, want %q", got, want)
	}
	if got, want := data.ViewDefinition(), "select id, name from student where active=1"; got != want {
		t.Errorf("ViewDefinition() = %q, want %q", got, want)
	}
}

func TestCreateViewDataZeroValueHasSafeAccessors(t *testing.T) {
	var data CreateViewData
	if got := data.ViewName(); got != "" {
		t.Errorf("ViewName() = %q, want empty", got)
	}
	if got := data.ViewDefinition(); got != "<unset-query>" {
		t.Errorf("ViewDefinition() = %q, want <unset-query>", got)
	}
}

func TestCreateIndexDataStoresIndexTableAndFieldNames(t *testing.T) {
	data := NewCreateIndexData("student_name_idx", "student", "name")
	if got, want := data.IndexName(), "student_name_idx"; got != want {
		t.Errorf("IndexName() = %q, want %q", got, want)
	}
	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.FieldName(), "name"; got != want {
		t.Errorf("FieldName() = %q, want %q", got, want)
	}
}

func TestCreateIndexDataZeroValueHasEmptyAccessors(t *testing.T) {
	var data CreateIndexData
	if got := data.IndexName(); got != "" {
		t.Errorf("IndexName() = %q, want empty", got)
	}
	if got := data.TableName(); got != "" {
		t.Errorf("TableName() = %q, want empty", got)
	}
	if got := data.FieldName(); got != "" {
		t.Errorf("FieldName() = %q, want empty", got)
	}
}
