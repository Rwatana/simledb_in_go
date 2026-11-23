package parse

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestDeleteDataStoresTableNameAndPredicate(t *testing.T) {
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("age"),
		query.NewConstantExpression(query.NewIntConstant(18)),
	))
	data := NewDeleteData("student", predicate)

	if got, want := data.TableName(), "student"; got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
	if got, want := data.Predicate().String(), "age=18"; got != want {
		t.Errorf("Predicate().String() = %q, want %q", got, want)
	}
}

func TestDeleteDataZeroValueHasEmptyAccessors(t *testing.T) {
	var data DeleteData
	if got := data.TableName(); got != "" {
		t.Errorf("TableName() = %q, want empty", got)
	}
	if got := data.Predicate().String(); got != "" {
		t.Errorf("Predicate().String() = %q, want empty", got)
	}
}
