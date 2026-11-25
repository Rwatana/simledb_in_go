package plan

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

var _ TablePlannerMetadata = (*metadata.MetadataManager)(nil)

func TestNewTablePlannerRetainsResolvedTableState(t *testing.T) {
	schema := newPlannerTestSchema("id")
	layout := record.NewLayout(schema)
	statistics := metadata.NewStatInfo(3, 12)
	index := &metadata.IndexInfo{}
	indexes := map[string]*metadata.IndexInfo{"id": index}
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"),
		query.NewConstantExpression(query.NewIntConstant(7)),
	))
	probe := &tablePlannerMetadataProbe{layout: layout, statistics: statistics, indexes: indexes}
	transaction := &tablePlanTransaction{}

	planner, err := NewTablePlanner("student", predicate, transaction, probe)
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}
	if planner.tableName != "student" || planner.transaction != transaction || planner.predicate.String() != predicate.String() {
		t.Errorf("retained table planner state = (%q, %p, %q), want (student, %p, %q)", planner.tableName, planner.transaction, planner.predicate.String(), transaction, predicate.String())
	}
	if planner.tablePlan == nil || planner.tablePlan.Schema() != schema || planner.tablePlan.statistics != statistics {
		t.Errorf("retained table plan = %#v, want the resolved schema and statistics", planner.tablePlan)
	}
	if planner.schema != schema {
		t.Errorf("retained schema = %p, want %p", planner.schema, schema)
	}
	if planner.indexes["id"] != index {
		t.Errorf("retained index = %p, want %p", planner.indexes["id"], index)
	}
	if got, want := strings.Join(probe.calls, ","), "layout,statistics,indexes"; got != want {
		t.Errorf("metadata calls = %q, want %q", got, want)
	}
	for _, tableName := range probe.tableNames {
		if tableName != "student" {
			t.Errorf("metadata table name = %q, want student", tableName)
		}
	}
	for _, gotTransaction := range probe.transactions {
		if gotTransaction != transaction {
			t.Errorf("metadata transaction = %p, want %p", gotTransaction, transaction)
		}
	}
	if len(probe.layouts) != 1 || probe.layouts[0] != layout {
		t.Errorf("statistics layouts = %v, want the resolved layout %p", probe.layouts, layout)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("planner initialization accessed table storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestNewTablePlannerRejectsUnavailableDependencies(t *testing.T) {
	if planner, err := NewTablePlanner("student", query.Predicate{}, &tablePlanTransaction{}, nil); planner != nil || err == nil {
		t.Errorf("NewTablePlanner() with nil metadata = (%v, %v), want (nil, error)", planner, err)
	}
	var nilMetadata *tablePlannerMetadataProbe
	if planner, err := NewTablePlanner("student", query.Predicate{}, &tablePlanTransaction{}, nilMetadata); planner != nil || err == nil {
		t.Errorf("NewTablePlanner() with typed nil metadata = (%v, %v), want (nil, error)", planner, err)
	}
	probe := &tablePlannerMetadataProbe{}
	if planner, err := NewTablePlanner("student", query.Predicate{}, nil, probe); planner != nil || err == nil {
		t.Errorf("NewTablePlanner() with nil transaction = (%v, %v), want (nil, error)", planner, err)
	}
	var nilTransaction *tablePlanTransaction
	if planner, err := NewTablePlanner("student", query.Predicate{}, nilTransaction, probe); planner != nil || err == nil {
		t.Errorf("NewTablePlanner() with typed nil transaction = (%v, %v), want (nil, error)", planner, err)
	}
	if len(probe.calls) != 0 {
		t.Errorf("metadata calls for missing transaction = %v, want none", probe.calls)
	}

	layout := record.NewLayout(newPlannerTestSchema("id"))
	planner, err := NewTablePlanner("student", query.Predicate{}, &tablePlanTransaction{}, &tablePlannerMetadataProbe{
		layout:     layout,
		statistics: metadata.NewStatInfo(1, 1),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() without registered indexes: %v", err)
	}
	if planner.indexes == nil || len(planner.indexes) != 0 {
		t.Errorf("index metadata = %v, want an initialized empty map", planner.indexes)
	}
}

func TestNewTablePlannerPreservesMetadataFailures(t *testing.T) {
	layoutFailure := errors.New("layout unavailable")
	statisticsFailure := errors.New("statistics unavailable")
	indexesFailure := errors.New("indexes unavailable")
	for _, test := range []struct {
		name      string
		probe     *tablePlannerMetadataProbe
		wantCalls string
		wantError error
		wantText  string
	}{
		{name: "layout error", probe: &tablePlannerMetadataProbe{layoutErr: layoutFailure}, wantCalls: "layout", wantError: layoutFailure, wantText: "layout"},
		{name: "nil layout", probe: &tablePlannerMetadataProbe{}, wantCalls: "layout", wantText: "layout is nil"},
		{name: "nil schema", probe: &tablePlannerMetadataProbe{layout: record.NewLayoutFromMetadata(nil, nil, 0)}, wantCalls: "layout", wantText: "schema is nil"},
		{name: "statistics error", probe: &tablePlannerMetadataProbe{layout: record.NewLayout(newPlannerTestSchema("id")), statisticsErr: statisticsFailure}, wantCalls: "layout,statistics", wantError: statisticsFailure, wantText: "statistics"},
		{name: "nil statistics", probe: &tablePlannerMetadataProbe{layout: record.NewLayout(newPlannerTestSchema("id"))}, wantCalls: "layout,statistics", wantText: "statistics are nil"},
		{name: "index error", probe: &tablePlannerMetadataProbe{layout: record.NewLayout(newPlannerTestSchema("id")), statistics: metadata.NewStatInfo(1, 1), indexesErr: indexesFailure}, wantCalls: "layout,statistics,indexes", wantError: indexesFailure, wantText: "index"},
	} {
		t.Run(test.name, func(t *testing.T) {
			planner, err := NewTablePlanner("student", query.Predicate{}, &tablePlanTransaction{}, test.probe)
			if planner != nil || err == nil {
				t.Fatalf("NewTablePlanner() = (%v, %v), want (nil, error)", planner, err)
			}
			if test.wantError != nil && !errors.Is(err, test.wantError) {
				t.Errorf("NewTablePlanner() error = %v, want wrapped %v", err, test.wantError)
			}
			if !strings.Contains(err.Error(), test.wantText) {
				t.Errorf("NewTablePlanner() error = %q, want context %q", err, test.wantText)
			}
			if got := strings.Join(test.probe.calls, ","); got != test.wantCalls {
				t.Errorf("metadata calls = %q, want %q", got, test.wantCalls)
			}
		})
	}
}

func TestTablePlannerMakeSelectPlanUsesMatchingIndex(t *testing.T) {
	index := &metadata.IndexInfo{}
	transaction := &tablePlanTransaction{}
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"),
		query.NewConstantExpression(query.NewIntConstant(7)),
	))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(newPlannerTestSchema("id")),
		statistics: metadata.NewStatInfo(3, 12),
		indexes:    map[string]*metadata.IndexInfo{"id": index},
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	selection, ok := planner.MakeSelectPlan().(*SelectPlan)
	if !ok {
		t.Fatalf("MakeSelectPlan() type = %T, want *SelectPlan", planner.MakeSelectPlan())
	}
	if got, want := selection.predicate.String(), predicate.String(); got != want {
		t.Errorf("selection predicate = %q, want %q", got, want)
	}
	got, ok := selection.child.(*IndexSelectPlan)
	if !ok {
		t.Fatalf("selection child type = %T, want *IndexSelectPlan", selection.child)
	}
	if got.child != planner.tablePlan || got.indexInfo != index {
		t.Errorf("indexed selection state = (child %p, index %p), want (%p, %p)", got.child, got.indexInfo, planner.tablePlan, index)
	}
	if value, ok := got.searchKey.AsInt(); !ok || value != 7 {
		t.Errorf("indexed selection key = (%d, %t), want (7, true)", value, ok)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("selection planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeSelectPlanRecognizesReversedConstantEquality(t *testing.T) {
	index := &metadata.IndexInfo{}
	planner, err := NewTablePlanner("student", query.NewPredicateWithTerm(query.NewTerm(
		query.NewConstantExpression(query.NewStringConstant("Ada")),
		query.NewFieldExpression("name"),
	)), &tablePlanTransaction{}, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(newPlannerTestSchema("name")),
		statistics: metadata.NewStatInfo(1, 1),
		indexes:    map[string]*metadata.IndexInfo{"name": index},
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	selection, ok := planner.MakeSelectPlan().(*SelectPlan)
	if !ok {
		t.Fatalf("MakeSelectPlan() type = %T, want *SelectPlan", planner.MakeSelectPlan())
	}
	got, ok := selection.child.(*IndexSelectPlan)
	if !ok {
		t.Fatalf("selection child type = %T, want *IndexSelectPlan", selection.child)
	}
	if got.indexInfo != index {
		t.Errorf("indexed selection index = %p, want %p", got.indexInfo, index)
	}
	if value, ok := got.searchKey.AsString(); !ok || value != "Ada" {
		t.Errorf("indexed selection key = (%q, %t), want (Ada, true)", value, ok)
	}
}

func TestTablePlannerMakeSelectPlanFallsBackWithoutMatchingIndex(t *testing.T) {
	for _, test := range []struct {
		name       string
		predicate  query.Predicate
		indexes    map[string]*metadata.IndexInfo
		wantSelect bool
	}{
		{name: "no indexes", predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(7)))), wantSelect: true},
		{name: "different indexed field", predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(7)))), indexes: map[string]*metadata.IndexInfo{"name": {}}, wantSelect: true},
		{name: "field equality", predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewFieldExpression("other"))), indexes: map[string]*metadata.IndexInfo{"id": {}}},
		{name: "nil index metadata", predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(7)))), indexes: map[string]*metadata.IndexInfo{"id": nil}, wantSelect: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := &tablePlanTransaction{}
			planner, err := NewTablePlanner("student", test.predicate, transaction, &tablePlannerMetadataProbe{
				layout:     record.NewLayout(newPlannerTestSchema("id")),
				statistics: metadata.NewStatInfo(3, 12),
				indexes:    test.indexes,
			})
			if err != nil {
				t.Fatalf("NewTablePlanner() error = %v", err)
			}

			got := planner.MakeSelectPlan()
			if test.wantSelect {
				selection, ok := got.(*SelectPlan)
				if !ok {
					t.Fatalf("MakeSelectPlan() = %T, want *SelectPlan", got)
				}
				if selection.child != planner.tablePlan || selection.predicate.String() != test.predicate.String() {
					t.Errorf("selection = (child %p, predicate %q), want (%p, %q)", selection.child, selection.predicate.String(), planner.tablePlan, test.predicate.String())
				}
			} else if got != planner.tablePlan {
				t.Errorf("MakeSelectPlan() = %T, want retained table plan %p", got, planner.tablePlan)
			}
			if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
				t.Errorf("selection planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
			}
		})
	}
}

func TestTablePlannerMakeSelectPlanAppliesOnlyTableLocalPredicate(t *testing.T) {
	schema := newPlannerTestSchema("id")
	schema.AddVarcharField("name", 24)
	index := &metadata.IndexInfo{}
	transaction := &tablePlanTransaction{}
	indexedTerm := query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(7)))
	localTerm := query.NewTerm(query.NewFieldExpression("name"), query.NewConstantExpression(query.NewStringConstant("Ada")))
	crossTableTerm := query.NewTerm(query.NewFieldExpression("id"), query.NewFieldExpression("department_id"))
	predicate := query.NewPredicateWithTerm(indexedTerm).
		ConjoinWith(query.NewPredicateWithTerm(localTerm)).
		ConjoinWith(query.NewPredicateWithTerm(crossTableTerm))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(schema),
		statistics: metadata.NewStatInfo(3, 12),
		indexes:    map[string]*metadata.IndexInfo{"id": index},
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	selection, ok := planner.MakeSelectPlan().(*SelectPlan)
	if !ok {
		t.Fatalf("MakeSelectPlan() type = %T, want *SelectPlan", planner.MakeSelectPlan())
	}
	indexSelection, ok := selection.child.(*IndexSelectPlan)
	if !ok {
		t.Fatalf("table-local selection child type = %T, want *IndexSelectPlan", selection.child)
	}
	if indexSelection.indexInfo != index || indexSelection.child != planner.tablePlan {
		t.Errorf("indexed base plan = (%p, %p), want (%p, %p)", indexSelection.indexInfo, indexSelection.child, index, planner.tablePlan)
	}
	if got, want := selection.predicate.String(), indexedTerm.String()+" and "+localTerm.String(); got != want {
		t.Errorf("table-local predicate = %q, want %q", got, want)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("selection planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

type tablePlannerMetadataProbe struct {
	layout        *record.Layout
	statistics    *metadata.StatInfo
	indexes       map[string]*metadata.IndexInfo
	layoutErr     error
	statisticsErr error
	indexesErr    error
	calls         []string
	tableNames    []string
	transactions  []record.TableScanTransaction
	layouts       []*record.Layout
}

func (probe *tablePlannerMetadataProbe) Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error) {
	probe.calls = append(probe.calls, "layout")
	probe.tableNames = append(probe.tableNames, tableName)
	probe.transactions = append(probe.transactions, transaction)
	return probe.layout, probe.layoutErr
}

func (probe *tablePlannerMetadataProbe) Statistics(tableName string, layout *record.Layout, transaction record.TableScanTransaction) (*metadata.StatInfo, error) {
	probe.calls = append(probe.calls, "statistics")
	probe.tableNames = append(probe.tableNames, tableName)
	probe.transactions = append(probe.transactions, transaction)
	probe.layouts = append(probe.layouts, layout)
	return probe.statistics, probe.statisticsErr
}

func (probe *tablePlannerMetadataProbe) IndexInfo(tableName string, transaction record.TableScanTransaction) (map[string]*metadata.IndexInfo, error) {
	probe.calls = append(probe.calls, "indexes")
	probe.tableNames = append(probe.tableNames, tableName)
	probe.transactions = append(probe.transactions, transaction)
	return probe.indexes, probe.indexesErr
}

func TestTablePlannerMakeJoinPlanBuildsProductForCrossSchemaPredicate(t *testing.T) {
	transaction := &tablePlanTransaction{}
	currentSchema := newPlannerTestSchema("department_id")
	current := NewTablePlan(transaction, "department", record.NewLayout(currentSchema), metadata.NewStatInfo(2, 6))
	tableSchema := newPlannerTestSchema("student_department_id")
	tableSchema.AddVarcharField("student_name", 24)
	joinTerm := query.NewTerm(query.NewFieldExpression("student_department_id"), query.NewFieldExpression("department_id"))
	localTerm := query.NewTerm(query.NewFieldExpression("student_name"), query.NewConstantExpression(query.NewStringConstant("Ada")))
	predicate := query.NewPredicateWithTerm(joinTerm).ConjoinWith(query.NewPredicateWithTerm(localTerm))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	joinPlan, ok := planner.MakeJoinPlan(current).(*SelectPlan)
	if !ok {
		t.Fatalf("MakeJoinPlan() type = %T, want *SelectPlan", planner.MakeJoinPlan(current))
	}
	product, ok := joinPlan.child.(*ProductPlan)
	if !ok {
		t.Fatalf("join selection child type = %T, want *ProductPlan", joinPlan.child)
	}
	if product.left != current {
		t.Errorf("product left = %p, want current plan %p", product.left, current)
	}
	tableSelection, ok := product.right.(*SelectPlan)
	if !ok {
		t.Fatalf("product right type = %T, want table-local *SelectPlan", product.right)
	}
	if tableSelection.child != planner.tablePlan || tableSelection.predicate.String() != localTerm.String() {
		t.Errorf("table-side selection = (child %p, predicate %q), want (%p, %q)", tableSelection.child, tableSelection.predicate.String(), planner.tablePlan, localTerm.String())
	}
	if got, want := joinPlan.predicate.String(), joinTerm.String(); got != want {
		t.Errorf("join predicate = %q, want %q", got, want)
	}
	if got, want := strings.Join(joinPlan.Schema().Fields(), ","), "department_id,student_department_id,student_name"; got != want {
		t.Errorf("join output fields = %q, want %q", got, want)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeJoinPlanReturnsNilWithoutCrossSchemaPredicate(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_name"),
		query.NewConstantExpression(query.NewStringConstant("Ada")),
	))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(newPlannerTestSchema("student_name")),
		statistics: metadata.NewStatInfo(3, 12),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}
	if got := planner.MakeJoinPlan(current); got != nil {
		t.Errorf("MakeJoinPlan() = %T, want nil without a cross-schema predicate", got)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeJoinPlanReturnsNilForNilCurrentPlan(t *testing.T) {
	planner, err := NewTablePlanner("student", query.Predicate{}, &tablePlanTransaction{}, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(newPlannerTestSchema("student_id")),
		statistics: metadata.NewStatInfo(1, 2),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}
	if got := planner.MakeJoinPlan(nil); got != nil {
		t.Errorf("MakeJoinPlan(nil) = %T, want nil", got)
	}
}
