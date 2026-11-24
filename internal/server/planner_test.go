package server

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestManagersComposeBasicQueryPlannerWithTheirMetadata(t *testing.T) {
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	if managers.QueryPlanner == nil {
		t.Fatal("NewManagers().QueryPlanner = nil, want a SELECT planner")
	}

	setup, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() setup error = %v", err)
	}
	if err := managers.InitializeCatalogs(setup); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 20)
	if err := managers.Metadata.CreateTable("student", &schema, setup); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}
	if _, err := setup.Commit(); err != nil {
		t.Fatalf("Commit() setup error = %v", err)
	}

	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() query error = %v", err)
	}
	queryData, err := parse.NewParser("select name from student").ParseQuery()
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	queryPlan, err := managers.QueryPlanner.CreatePlan(queryData, transaction)
	if err != nil {
		t.Fatalf("CreatePlan() through manager planner error = %v", err)
	}
	if got, want := queryPlan.Schema().Fields(), []string{"name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("CreatePlan().Schema().Fields() = %v, want %v", got, want)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() query error = %v", err)
	}
}
