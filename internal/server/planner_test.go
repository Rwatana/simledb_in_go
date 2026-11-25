package server

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/materialize"
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/plan"
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

func TestManagersSetQueryPlannerStrategyUsesSharedMetadata(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	if _, ok := managers.QueryPlanner.(*plan.BasicQueryPlanner); !ok {
		t.Fatalf("NewManagers().QueryPlanner has type %T, want *plan.BasicQueryPlanner by default", managers.QueryPlanner)
	}

	for _, test := range []struct {
		name     string
		strategy QueryPlannerStrategy
		wantType any
	}{
		{name: "basic", strategy: QueryPlannerStrategyBasic, wantType: (*plan.BasicQueryPlanner)(nil)},
		{name: "better", strategy: QueryPlannerStrategyBetter, wantType: (*plan.BetterQueryPlanner)(nil)},
		{name: "heuristic", strategy: QueryPlannerStrategyHeuristic, wantType: (*heuristicQueryPlanner)(nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := managers.SetQueryPlannerStrategy(test.strategy); err != nil {
				t.Fatalf("SetQueryPlannerStrategy(%v) error = %v", test.strategy, err)
			}
			if got := reflect.TypeOf(managers.QueryPlanner); got != reflect.TypeOf(test.wantType) {
				t.Fatalf("QueryPlanner type = %v, want %v", got, reflect.TypeOf(test.wantType))
			}

			transaction, err := managers.NewTransaction()
			if err != nil {
				t.Fatalf("NewTransaction() error = %v", err)
			}
			queryPlan, err := managers.CreateQueryPlan("select name from student where id = 1", transaction)
			if err != nil {
				t.Fatalf("CreateQueryPlan() with selected strategy error = %v", err)
			}
			if got, want := queryPlan.Schema().Fields(), []string{"name"}; !reflect.DeepEqual(got, want) {
				t.Errorf("CreateQueryPlan().Schema().Fields() = %v, want %v", got, want)
			}
			if _, err := transaction.Commit(); err != nil {
				t.Fatalf("Commit() after planning error = %v", err)
			}
		})
	}
}

func TestHeuristicProductFactoryUsesTransactionBoundMultibufferProducts(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}
	factory := heuristicProductPlanFactory(transaction)
	if factory == nil {
		t.Fatal("heuristicProductPlanFactory(transaction) = nil, want multibuffer factory")
	}
	if got := factory(nil, nil); reflect.TypeOf(got) != reflect.TypeOf((*materialize.MultibufferProductPlan)(nil)) {
		t.Fatalf("factory() type = %T, want *MultibufferProductPlan", got)
	}

	wrapped := tableScanTransaction{transaction: transaction}
	if got := heuristicProductPlanFactory(wrapped); got != nil {
		t.Fatalf("heuristicProductPlanFactory(tableScanTransaction) = %T, want nil for ordinary ProductPlan fallback", got)
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

func TestManagersHeuristicStrategyPlansConnectedAndDisconnectedSources(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	setup, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() setup error = %v", err)
	}
	var courseSchema record.Schema
	courseSchema.AddIntegerField("course_id")
	if rows, err := managers.UpdatePlanner.ExecuteCreateTable(parse.NewCreateTableData("course", courseSchema), setup); err != nil || rows != 0 {
		t.Fatalf("ExecuteCreateTable(course) = (%d, %v), want (0, nil)", rows, err)
	}
	if _, err := setup.Commit(); err != nil {
		t.Fatalf("Commit() setup error = %v", err)
	}
	if err := managers.SetQueryPlannerStrategy(QueryPlannerStrategyHeuristic); err != nil {
		t.Fatalf("SetQueryPlannerStrategy(heuristic) error = %v", err)
	}

	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() query error = %v", err)
	}
	for _, test := range []struct {
		statement string
		fields    []string
	}{
		{statement: "select name from student, course where id = course_id", fields: []string{"name"}},
		{statement: "select course_id from student, course", fields: []string{"course_id"}},
	} {
		queryPlan, err := managers.CreateQueryPlan(test.statement, transaction)
		if err != nil {
			t.Fatalf("CreateQueryPlan(%q) error = %v", test.statement, err)
		}
		if got := queryPlan.Schema().Fields(); !reflect.DeepEqual(got, test.fields) {
			t.Errorf("CreateQueryPlan(%q).Schema().Fields() = %v, want %v", test.statement, got, test.fields)
		}
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() query error = %v", err)
	}
}

func TestManagersSetQueryPlannerStrategyRejectsInvalidInputsWithoutReplacingPlanner(t *testing.T) {
	managers := newQueryDispatchManagers(t)
	previous := managers.QueryPlanner
	for name, managerSet := range map[string]*Managers{
		"nil":              nil,
		"missing metadata": {QueryPlanner: previous},
	} {
		t.Run(name, func(t *testing.T) {
			if err := managerSet.SetQueryPlannerStrategy(QueryPlannerStrategyBetter); err == nil || !strings.Contains(err.Error(), "manager") {
				t.Fatalf("SetQueryPlannerStrategy() error = %v, want manager validation error", err)
			}
			if managerSet != nil && managerSet.QueryPlanner != previous {
				t.Error("SetQueryPlannerStrategy() replaced the planner after manager validation failed")
			}
		})
	}

	if err := managers.SetQueryPlannerStrategy(QueryPlannerStrategy(255)); err == nil || !strings.Contains(err.Error(), "strategy") {
		t.Fatalf("SetQueryPlannerStrategy(unknown) error = %v, want strategy validation error", err)
	}
	if managers.QueryPlanner != previous {
		t.Error("SetQueryPlannerStrategy() replaced the planner after rejecting an unknown strategy")
	}
}
