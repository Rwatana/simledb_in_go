package plan

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/parse"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestHeuristicQueryPlannerBuildsLowestOutputJoinThenProjects(t *testing.T) {
	transaction := &tablePlanTransaction{}
	studentSchema := newPlannerTestSchema("student_department_id")
	studentSchema.AddIntegerField("student_id")
	studentSchema.AddVarcharField("student_name", 24)
	departmentSchema := newPlannerTestSchema("department_id")
	departmentSchema.AddVarcharField("department_name", 24)
	metadataProbe := &heuristicQueryPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"student":    record.NewLayout(studentSchema),
			"department": record.NewLayout(departmentSchema),
		},
		statistics: map[string]*metadata.StatInfo{
			"student":    metadata.NewStatInfo(3, 12),
			"department": metadata.NewStatInfo(2, 6),
		},
	}
	joinPredicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_department_id"),
		query.NewFieldExpression("department_id"),
	))
	factoryCalls := 0
	planner := NewHeuristicQueryPlanner(metadataProbe, func(left, right Plan) Plan {
		factoryCalls++
		return NewProductPlan(left, right)
	})
	data := parse.NewQueryData([]string{"student_name", "department_id"}, []string{"student", "department"}, joinPredicate)

	got, err := planner.CreatePlan(data, transaction)
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	project, ok := got.(*ProjectPlan)
	if !ok {
		t.Fatalf("CreatePlan() type = %T, want *ProjectPlan", got)
	}
	if got, want := project.Schema().Fields(), []string{"student_name", "department_id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("projected fields = %v, want %v", got, want)
	}
	joinSelection, ok := project.child.(*SelectPlan)
	if !ok {
		t.Fatalf("project child type = %T, want join *SelectPlan", project.child)
	}
	product, ok := joinSelection.child.(*ProductPlan)
	if !ok {
		t.Fatalf("join selection child type = %T, want product fallback", joinSelection.child)
	}
	left, leftOK := product.left.(*TablePlan)
	right, rightOK := product.right.(*TablePlan)
	if !leftOK || !rightOK || left.tableName != "department" || right.tableName != "student" {
		t.Errorf("left-deep join inputs = (%T, %T), want lowest-output department then student", product.left, product.right)
	}
	if factoryCalls != 0 {
		t.Errorf("product factory calls for applicable join = %d, want 0", factoryCalls)
	}
	if got, want := metadataProbe.calls, []string{
		"layout:student", "statistics:student", "indexes:student",
		"layout:department", "statistics:department", "indexes:department",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("metadata requests = %v, want %v", got, want)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("plan creation accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestHeuristicQueryPlannerUsesFactoryForLowestProductFallback(t *testing.T) {
	transaction := &tablePlanTransaction{}
	metadataProbe := heuristicMetadataForCounts(map[string]int{"department": 8, "student": 4, "course": 2})
	factoryCalls := make([]string, 0)
	planner := NewHeuristicQueryPlanner(metadataProbe, func(left, right Plan) Plan {
		table := right.(*TablePlan)
		factoryCalls = append(factoryCalls, table.tableName)
		return NewProductPlan(left, right)
	})
	data := parse.NewQueryData([]string{"course_id", "department_id"}, []string{"department", "student", "course"}, query.Predicate{})

	got, err := planner.CreatePlan(data, transaction)
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	project, ok := got.(*ProjectPlan)
	if !ok {
		t.Fatalf("CreatePlan() type = %T, want *ProjectPlan", got)
	}
	outer, ok := project.child.(*ProductPlan)
	if !ok {
		t.Fatalf("project child type = %T, want outer product", project.child)
	}
	inner, ok := outer.left.(*ProductPlan)
	if !ok {
		t.Fatalf("outer product left type = %T, want first product", outer.left)
	}
	first, firstOK := inner.left.(*TablePlan)
	second, secondOK := inner.right.(*TablePlan)
	last, lastOK := outer.right.(*TablePlan)
	if !firstOK || !secondOK || !lastOK || first.tableName != "course" || second.tableName != "student" || last.tableName != "department" {
		t.Errorf("product order = (%T, %T, %T), want course, student, department", inner.left, inner.right, outer.right)
	}
	if got, want := factoryCalls, []string{"department", "student", "department"}; !reflect.DeepEqual(got, want) {
		t.Errorf("product factory calls = %v, want candidates in each source order: %v", got, want)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("plan creation accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestHeuristicQueryPlannerDefaultsToProductPlanFactory(t *testing.T) {
	transaction := &tablePlanTransaction{}
	planner := NewHeuristicQueryPlanner(heuristicMetadataForCounts(map[string]int{"left": 1, "right": 2}), nil)
	data := parse.NewQueryData([]string{"left_id"}, []string{"left", "right"}, query.Predicate{})

	got, err := planner.CreatePlan(data, transaction)
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	project, ok := got.(*ProjectPlan)
	if !ok {
		t.Fatalf("CreatePlan() type = %T, want *ProjectPlan", got)
	}
	if _, ok := project.child.(*ProductPlan); !ok {
		t.Errorf("project child type = %T, want default *ProductPlan", project.child)
	}
}

func TestHeuristicQueryPlannerRejectsUnavailableInputsAndMetadata(t *testing.T) {
	transaction := &tablePlanTransaction{}
	data := parse.NewQueryData([]string{"id"}, []string{"student"}, query.Predicate{})
	var typedNilMetadata *heuristicQueryPlannerMetadataProbe
	validMetadata := heuristicMetadataForCounts(map[string]int{"student": 1})
	var nilPlanner *HeuristicQueryPlanner
	for _, test := range []struct {
		name    string
		planner *HeuristicQueryPlanner
		data    parse.QueryData
		tx      record.TableScanTransaction
		want    string
	}{
		{name: "nil planner", planner: nilPlanner, data: data, tx: transaction, want: "planner is nil"},
		{name: "nil metadata", planner: NewHeuristicQueryPlanner(nil, nil), data: data, tx: transaction, want: "metadata is nil"},
		{name: "typed nil metadata", planner: NewHeuristicQueryPlanner(typedNilMetadata, nil), data: data, tx: transaction, want: "metadata is nil"},
		{name: "nil transaction", planner: NewHeuristicQueryPlanner(validMetadata, nil), data: data, want: "transaction is nil"},
		{name: "no tables", planner: NewHeuristicQueryPlanner(validMetadata, nil), data: parse.NewQueryData([]string{"id"}, nil, query.Predicate{}), tx: transaction, want: "query has no tables"},
		{name: "layout error", planner: NewHeuristicQueryPlanner(&heuristicQueryPlannerMetadataProbe{layoutErrors: map[string]error{"student": errors.New("layout unavailable")}}, nil), data: data, tx: transaction, want: `initialize table "student"`},
		{name: "statistics error", planner: NewHeuristicQueryPlanner(&heuristicQueryPlannerMetadataProbe{layouts: validMetadata.layouts, statisticsErrors: map[string]error{"student": errors.New("statistics unavailable")}}, nil), data: data, tx: transaction, want: `initialize table "student"`},
		{name: "index error", planner: NewHeuristicQueryPlanner(&heuristicQueryPlannerMetadataProbe{layouts: validMetadata.layouts, statistics: validMetadata.statistics, indexErrors: map[string]error{"student": errors.New("indexes unavailable")}}, nil), data: data, tx: transaction, want: `initialize table "student"`},
		{name: "projection error", planner: NewHeuristicQueryPlanner(validMetadata, nil), data: parse.NewQueryData([]string{"missing"}, []string{"student"}, query.Predicate{}), tx: transaction, want: "projection"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := test.planner.CreatePlan(test.data, test.tx)
			if plan != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("CreatePlan() = (%v, %v), want nil plan and error containing %q", plan, err, test.want)
			}
		})
	}
}

func TestHeuristicQueryPlannerRejectsNilProductFactoryResult(t *testing.T) {
	transaction := &tablePlanTransaction{}
	planner := NewHeuristicQueryPlanner(heuristicMetadataForCounts(map[string]int{"left": 1, "right": 2}), func(Plan, Plan) Plan {
		return nil
	})
	data := parse.NewQueryData([]string{"left_id"}, []string{"left", "right"}, query.Predicate{})

	plan, err := planner.CreatePlan(data, transaction)
	if plan != nil || err == nil || !strings.Contains(err.Error(), "choose next query product") {
		t.Errorf("CreatePlan() = (%v, %v), want product-factory error", plan, err)
	}
}

type heuristicQueryPlannerMetadataProbe struct {
	layouts          map[string]*record.Layout
	statistics       map[string]*metadata.StatInfo
	indexes          map[string]map[string]*metadata.IndexInfo
	layoutErrors     map[string]error
	statisticsErrors map[string]error
	indexErrors      map[string]error
	calls            []string
}

func (probe *heuristicQueryPlannerMetadataProbe) Layout(tableName string, _ record.TableScanTransaction) (*record.Layout, error) {
	probe.calls = append(probe.calls, "layout:"+tableName)
	return probe.layouts[tableName], probe.layoutErrors[tableName]
}

func (probe *heuristicQueryPlannerMetadataProbe) Statistics(tableName string, _ *record.Layout, _ record.TableScanTransaction) (*metadata.StatInfo, error) {
	probe.calls = append(probe.calls, "statistics:"+tableName)
	return probe.statistics[tableName], probe.statisticsErrors[tableName]
}

func (probe *heuristicQueryPlannerMetadataProbe) IndexInfo(tableName string, _ record.TableScanTransaction) (map[string]*metadata.IndexInfo, error) {
	probe.calls = append(probe.calls, "indexes:"+tableName)
	return probe.indexes[tableName], probe.indexErrors[tableName]
}

func heuristicMetadataForCounts(counts map[string]int) *heuristicQueryPlannerMetadataProbe {
	probe := &heuristicQueryPlannerMetadataProbe{
		layouts:    make(map[string]*record.Layout, len(counts)),
		statistics: make(map[string]*metadata.StatInfo, len(counts)),
	}
	for tableName, records := range counts {
		probe.layouts[tableName] = record.NewLayout(newPlannerTestSchema(tableName + "_id"))
		probe.statistics[tableName] = metadata.NewStatInfo(1, records)
	}
	return probe
}
