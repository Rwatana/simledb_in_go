package server

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestManagersCreateQueryPlanParsesAndPlansSelect(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	queryPlan, err := managers.CreateQueryPlan("select name from student where id = 1", transaction)
	if err != nil {
		t.Fatalf("CreateQueryPlan() error = %v", err)
	}
	if got, want := queryPlan.Schema().Fields(), []string{"name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateQueryPlan().Schema().Fields() = %v, want %v", got, want)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() after planning error = %v", err)
	}
}

func TestManagersCreateQueryPlanReportsInvalidInputsAndFailures(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	for name, managerSet := range map[string]*Managers{
		"nil":        nil,
		"incomplete": {},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := managerSet.CreateQueryPlan("select id from student", transaction)
			if got != nil || err == nil || !strings.Contains(err.Error(), "manager") {
				t.Fatalf("CreateQueryPlan() = (%v, %v), want manager validation error", got, err)
			}
		})
	}

	if got, err := managers.CreateQueryPlan("select id from student", nil); got != nil || err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("CreateQueryPlan() with nil transaction = (%v, %v), want transaction error", got, err)
	}
	if got, err := managers.CreateQueryPlan("select from", transaction); got != nil || err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("CreateQueryPlan() with invalid SQL = (%v, %v), want parse error", got, err)
	}
	if got, err := managers.CreateQueryPlan("select name from missing_student", transaction); got != nil || err == nil || !strings.Contains(err.Error(), "plan") {
		t.Errorf("CreateQueryPlan() with an unknown table = (%v, %v), want planning error", got, err)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() after rejected queries error = %v", err)
	}
}

func newQueryDispatchManagers(t *testing.T) *Managers {
	t.Helper()
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() setup error = %v", err)
	}
	if err := managers.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 20)
	if rows, err := managers.UpdatePlanner.ExecuteCreateTable(parse.NewCreateTableData("student", schema), transaction); err != nil || rows != 0 {
		t.Fatalf("ExecuteCreateTable() = (%d, %v), want (0, nil)", rows, err)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() setup error = %v", err)
	}
	return managers
}
