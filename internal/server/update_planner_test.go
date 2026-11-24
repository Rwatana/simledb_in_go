package server

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestManagersComposeBasicUpdatePlannerWithTheirMetadata(t *testing.T) {
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	if managers.UpdatePlanner == nil {
		t.Fatal("NewManagers().UpdatePlanner = nil, want an update planner")
	}

	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	if err := managers.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("course_id")
	schema.AddVarcharField("title", 30)
	if rows, err := managers.UpdatePlanner.ExecuteCreateTable(parse.NewCreateTableData("course", schema), transaction); err != nil || rows != 0 {
		t.Fatalf("ExecuteCreateTable() = (%d, %v), want (0, nil)", rows, err)
	}
	layout, err := managers.Metadata.Layout("course", transaction)
	if err != nil {
		t.Fatalf("Metadata.Layout() after planner execution error = %v", err)
	}
	if got, want := layout.Schema().Fields(), []string{"course_id", "title"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("registered fields = %v, want %v", got, want)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}
