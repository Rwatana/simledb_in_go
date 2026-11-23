package parse

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
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
