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

func TestTablePlannerMakeJoinPlanPrefersMatchingIndexJoin(t *testing.T) {
	for _, test := range []struct {
		name     string
		joinTerm query.Term
	}{
		{
			name: "indexed table field on left",
			joinTerm: query.NewTerm(
				query.NewFieldExpression("student_department_id"),
				query.NewFieldExpression("department_id"),
			),
		},
		{
			name: "indexed table field on right",
			joinTerm: query.NewTerm(
				query.NewFieldExpression("department_id"),
				query.NewFieldExpression("student_department_id"),
			),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := &tablePlanTransaction{}
			currentSchema := newPlannerTestSchema("department_id")
			current := NewTablePlan(transaction, "department", record.NewLayout(currentSchema), metadata.NewStatInfo(2, 6))
			tableSchema := newPlannerTestSchema("student_department_id")
			tableSchema.AddVarcharField("student_name", 24)
			indexInfo, err := metadata.NewIndexInfo(
				"student_department_idx",
				"student_department_id",
				tableSchema,
				transaction,
				metadata.NewStatInfo(3, 12),
			)
			if err != nil {
				t.Fatalf("NewIndexInfo() error = %v", err)
			}
			localTerm := query.NewTerm(
				query.NewFieldExpression("student_name"),
				query.NewConstantExpression(query.NewStringConstant("Ada")),
			)
			predicate := query.NewPredicateWithTerm(test.joinTerm).ConjoinWith(query.NewPredicateWithTerm(localTerm))
			planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
				layout:     record.NewLayout(tableSchema),
				statistics: metadata.NewStatInfo(3, 12),
				indexes:    map[string]*metadata.IndexInfo{"student_department_id": indexInfo},
			})
			if err != nil {
				t.Fatalf("NewTablePlanner() error = %v", err)
			}

			joinPlan, ok := planner.MakeJoinPlan(current).(*SelectPlan)
			if !ok {
				t.Fatalf("MakeJoinPlan() type = %T, want outer join-predicate *SelectPlan", planner.MakeJoinPlan(current))
			}
			if got, want := joinPlan.predicate.String(), test.joinTerm.String(); got != want {
				t.Errorf("outer predicate = %q, want join predicate %q", got, want)
			}
			localSelection, ok := joinPlan.child.(*SelectPlan)
			if !ok {
				t.Fatalf("join selection child type = %T, want table-local *SelectPlan", joinPlan.child)
			}
			if got, want := localSelection.predicate.String(), localTerm.String(); got != want {
				t.Errorf("table-local predicate = %q, want %q", got, want)
			}
			indexJoin, ok := localSelection.child.(*IndexJoinPlan)
			if !ok {
				t.Fatalf("table-local selection child type = %T, want *IndexJoinPlan", localSelection.child)
			}
			if indexJoin.left != current || indexJoin.right != planner.tablePlan || indexJoin.indexInfo != indexInfo || indexJoin.joinField != "department_id" {
				t.Errorf("index join = (left %p, right %p, index %p, field %q), want (%p, %p, %p, department_id)",
					indexJoin.left, indexJoin.right, indexJoin.indexInfo, indexJoin.joinField,
					current, planner.tablePlan, indexInfo)
			}
			if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
				t.Errorf("index join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
			}
		})
	}
}

func TestTablePlannerMakeJoinPlanFallsBackWhenIndexIsNotOnJoinedField(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	tableSchema := newPlannerTestSchema("student_department_id")
	tableSchema.AddIntegerField("student_code")
	indexInfo, err := metadata.NewIndexInfo(
		"student_code_idx",
		"student_code",
		tableSchema,
		transaction,
		metadata.NewStatInfo(3, 12),
	)
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	planner, err := NewTablePlanner("student", query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_department_id"),
		query.NewFieldExpression("department_id"),
	)), transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
		indexes: map[string]*metadata.IndexInfo{
			"student_code":          indexInfo,
			"student_department_id": nil,
		},
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
		t.Fatalf("join selection child type = %T, want product fallback", joinPlan.child)
	}
	if product.left != current {
		t.Errorf("product left = %p, want current plan %p", product.left, current)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("product join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeJoinPlanUsesIndexJoinWithoutLocalSelection(t *testing.T) {
	transaction := &tablePlanTransaction{}
	currentSchema := newPlannerTestSchema("department_id")
	current := NewTablePlan(transaction, "department", record.NewLayout(currentSchema), metadata.NewStatInfo(2, 6))
	tableSchema := newPlannerTestSchema("student_department_id")
	indexInfo, err := metadata.NewIndexInfo(
		"student_department_idx",
		"student_department_id",
		tableSchema,
		transaction,
		metadata.NewStatInfo(3, 12),
	)
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	joinTerm := query.NewTerm(query.NewFieldExpression("student_department_id"), query.NewFieldExpression("department_id"))
	planner, err := NewTablePlanner("student", query.NewPredicateWithTerm(joinTerm), transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
		indexes:    map[string]*metadata.IndexInfo{"student_department_id": indexInfo},
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	joinPlan, ok := planner.MakeJoinPlan(current).(*SelectPlan)
	if !ok {
		t.Fatalf("MakeJoinPlan() type = %T, want *SelectPlan", planner.MakeJoinPlan(current))
	}
	indexJoin, ok := joinPlan.child.(*IndexJoinPlan)
	if !ok {
		t.Fatalf("join selection child type = %T, want *IndexJoinPlan without a local selection", joinPlan.child)
	}
	if indexJoin.left != current || indexJoin.right != planner.tablePlan || indexJoin.indexInfo != indexInfo || indexJoin.joinField != "department_id" {
		t.Errorf("index join = (left %p, right %p, index %p, field %q), want (%p, %p, %p, department_id)",
			indexJoin.left, indexJoin.right, indexJoin.indexInfo, indexJoin.joinField,
			current, planner.tablePlan, indexInfo)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("index join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeJoinPlanRequiresOppositeFieldInCurrentSchema(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	tableSchema := newPlannerTestSchema("student_department_id")
	tableSchema.AddIntegerField("student_code")
	indexInfo, err := metadata.NewIndexInfo(
		"student_code_idx",
		"student_code",
		tableSchema,
		transaction,
		metadata.NewStatInfo(3, 12),
	)
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	joinTerm := query.NewTerm(query.NewFieldExpression("student_department_id"), query.NewFieldExpression("department_id"))
	unavailableOuterTerm := query.NewTerm(query.NewFieldExpression("student_code"), query.NewFieldExpression("outside_field"))
	predicate := query.NewPredicateWithTerm(joinTerm).ConjoinWith(query.NewPredicateWithTerm(unavailableOuterTerm))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
		indexes:    map[string]*metadata.IndexInfo{"student_code": indexInfo},
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
		t.Fatalf("join selection child type = %T, want product fallback for an unavailable outer field", joinPlan.child)
	}
	if product.left != current {
		t.Errorf("product left = %p, want current plan %p", product.left, current)
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("join planning accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeProductPlanFiltersTableBeforeFactory(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	tableSchema := newPlannerTestSchema("student_department_id")
	tableSchema.AddVarcharField("student_name", 24)
	localTerm := query.NewTerm(
		query.NewFieldExpression("student_name"),
		query.NewConstantExpression(query.NewStringConstant("Ada")),
	)
	joinTerm := query.NewTerm(query.NewFieldExpression("student_department_id"), query.NewFieldExpression("department_id"))
	predicate := query.NewPredicateWithTerm(localTerm).ConjoinWith(query.NewPredicateWithTerm(joinTerm))
	planner, err := NewTablePlanner("student", predicate, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}

	wantPlan := &productPlanProbe{}
	var gotLeft, gotRight Plan
	factoryCalls := 0
	factory := ProductPlanFactory(func(left, right Plan) Plan {
		factoryCalls++
		gotLeft, gotRight = left, right
		return wantPlan
	})
	if got := planner.MakeProductPlan(current, factory); got != wantPlan {
		t.Fatalf("MakeProductPlan() = %T, want factory result %T", got, wantPlan)
	}
	if factoryCalls != 1 {
		t.Fatalf("product factory calls = %d, want 1", factoryCalls)
	}
	if gotLeft != current {
		t.Errorf("factory left plan = %p, want current plan %p", gotLeft, current)
	}
	tableSelection, ok := gotRight.(*SelectPlan)
	if !ok {
		t.Fatalf("factory right plan type = %T, want table-local *SelectPlan", gotRight)
	}
	if tableSelection.child != planner.tablePlan || tableSelection.predicate.String() != localTerm.String() {
		t.Errorf("factory table selection = (child %p, predicate %q), want (%p, %q)",
			tableSelection.child, tableSelection.predicate.String(), planner.tablePlan, localTerm.String())
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("product plan construction accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestTablePlannerMakeProductPlanPassesBaseTableWithoutLocalPredicate(t *testing.T) {
	transaction := &tablePlanTransaction{}
	tableSchema := newPlannerTestSchema("student_id")
	planner, err := NewTablePlanner("student", query.Predicate{}, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(tableSchema),
		statistics: metadata.NewStatInfo(3, 12),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	wantPlan := &productPlanProbe{}
	var gotLeft, gotRight Plan
	factoryCalls := 0
	factory := ProductPlanFactory(func(left, right Plan) Plan {
		factoryCalls++
		gotLeft, gotRight = left, right
		return wantPlan
	})

	if got := planner.MakeProductPlan(current, factory); got != wantPlan {
		t.Fatalf("MakeProductPlan() = %T, want factory result %T", got, wantPlan)
	}
	if factoryCalls != 1 || gotLeft != current || gotRight != planner.tablePlan {
		t.Errorf("factory call = %d, inputs = (%p, %p), want one call with (%p, %p)",
			factoryCalls, gotLeft, gotRight, current, planner.tablePlan)
	}
}

func TestTablePlannerMakeProductPlanReturnsNilForUnavailableInputs(t *testing.T) {
	transaction := &tablePlanTransaction{}
	planner, err := NewTablePlanner("student", query.Predicate{}, transaction, &tablePlannerMetadataProbe{
		layout:     record.NewLayout(newPlannerTestSchema("student_id")),
		statistics: metadata.NewStatInfo(3, 12),
	})
	if err != nil {
		t.Fatalf("NewTablePlanner() error = %v", err)
	}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(2, 6))
	factoryCalls := 0
	factory := ProductPlanFactory(func(Plan, Plan) Plan {
		factoryCalls++
		return &productPlanProbe{}
	})
	var typedNilPlanner *TablePlanner
	var typedNilCurrent *TablePlan
	cases := []struct {
		name    string
		planner *TablePlanner
		current Plan
		factory ProductPlanFactory
	}{
		{name: "nil planner", planner: nil, current: current, factory: factory},
		{name: "typed nil planner", planner: typedNilPlanner, current: current, factory: factory},
		{name: "nil current", planner: planner, current: nil, factory: factory},
		{name: "typed nil current", planner: planner, current: typedNilCurrent, factory: factory},
		{name: "nil factory", planner: planner, current: current, factory: nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.planner.MakeProductPlan(test.current, test.factory); got != nil {
				t.Errorf("MakeProductPlan() = %T, want nil for %s", got, test.name)
			}
		})
	}
	if factoryCalls != 0 {
		t.Errorf("product factory calls = %d, want 0 for unavailable inputs", factoryCalls)
	}
}

func TestChooseLowestSelectPlanUsesFilteredOutputAndRemovesWinner(t *testing.T) {
	transaction := &tablePlanTransaction{}
	filtered := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"),
		query.NewConstantExpression(query.NewIntConstant(7)),
	))
	planners := []*TablePlanner{
		newSelectCandidate("large_filtered", 100, filtered, transaction),
		newSelectCandidate("small_unfiltered", 3, query.Predicate{}, transaction),
		newSelectCandidate("largest_unfiltered", 40, query.Predicate{}, transaction),
	}
	original := append([]*TablePlanner(nil), planners...)

	gotPlan, remaining, err := chooseLowestSelectPlan(planners)
	if err != nil {
		t.Fatalf("chooseLowestSelectPlan() error = %v", err)
	}
	selected, ok := gotPlan.(*SelectPlan)
	if !ok {
		t.Fatalf("selected plan type = %T, want filtered *SelectPlan", gotPlan)
	}
	selectedTable, ok := selected.child.(*TablePlan)
	if !ok {
		t.Fatalf("selected plan child type = %T, want *TablePlan", selected.child)
	}
	if selectedTable.tableName != "large_filtered" || selected.RecordsOutput() != 2 {
		t.Errorf("selected plan = (%q, %d rows), want (large_filtered, 2 rows)", selectedTable.tableName, selected.RecordsOutput())
	}
	if len(remaining) != 2 || remaining[0] != planners[1] || remaining[1] != planners[2] {
		t.Errorf("remaining planners = %v, want the unselected candidates in source order", remaining)
	}
	for index, planner := range original {
		if planners[index] != planner {
			t.Errorf("input planner %d changed from %p to %p", index, planner, planners[index])
		}
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("selection choice accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestChooseLowestSelectPlanKeepsFirstCandidateOnTies(t *testing.T) {
	transaction := &tablePlanTransaction{}
	planners := []*TablePlanner{
		newSelectCandidate("first", 5, query.Predicate{}, transaction),
		newSelectCandidate("second", 5, query.Predicate{}, transaction),
	}

	gotPlan, remaining, err := chooseLowestSelectPlan(planners)
	if err != nil {
		t.Fatalf("chooseLowestSelectPlan() error = %v", err)
	}
	if got := gotPlan.(*TablePlan).tableName; got != "first" {
		t.Errorf("selected table = %q, want first source on an equal estimate", got)
	}
	if len(remaining) != 1 || remaining[0] != planners[1] {
		t.Errorf("remaining planners = %v, want only the second source", remaining)
	}
}

func TestChooseLowestSelectPlanRejectsUnavailableCandidates(t *testing.T) {
	for _, test := range []struct {
		name     string
		planners []*TablePlanner
		wantText string
	}{
		{name: "empty", planners: nil, wantText: "no table planners"},
		{name: "nil planner", planners: []*TablePlanner{nil}, wantText: "table planner at index 0 is nil"},
		{name: "nil table plan", planners: []*TablePlanner{{tableName: "student"}}, wantText: `table plan for "student" is nil`},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, remaining, err := chooseLowestSelectPlan(test.planners)
			if plan != nil || remaining != nil || err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Errorf("chooseLowestSelectPlan() = (%v, %v, %v), want no plan, no remainder, and error containing %q", plan, remaining, err, test.wantText)
			}
		})
	}
}

func newSelectCandidate(tableName string, records int, predicate query.Predicate, transaction record.TableScanTransaction) *TablePlanner {
	return newSelectCandidateWithField(tableName, "id", records, predicate, transaction)
}

func newSelectCandidateWithField(tableName, fieldName string, records int, predicate query.Predicate, transaction record.TableScanTransaction) *TablePlanner {
	schema := newPlannerTestSchema(fieldName)
	return &TablePlanner{
		tableName:   tableName,
		predicate:   predicate,
		transaction: transaction,
		tablePlan:   NewTablePlan(transaction, tableName, record.NewLayout(schema), metadata.NewStatInfo(1, records)),
		schema:      schema,
		indexes:     make(map[string]*metadata.IndexInfo),
	}
}

func TestChooseLowestJoinPlanSelectsMinimumApplicableAndRemovesWinner(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(1, 10))
	joinPredicate := func(tableField string) query.Predicate {
		return query.NewPredicateWithTerm(query.NewTerm(
			query.NewFieldExpression(tableField),
			query.NewFieldExpression("department_id"),
		))
	}
	planners := []*TablePlanner{
		newSelectCandidate("unrelated", 1, query.Predicate{}, transaction),
		newSelectCandidateWithField("first_join", "first_department_id", 100, joinPredicate("first_department_id"), transaction),
		newSelectCandidateWithField("best_join", "best_department_id", 4, joinPredicate("best_department_id"), transaction),
	}
	original := append([]*TablePlanner(nil), planners...)

	gotPlan, remaining, err := chooseLowestJoinPlan(planners, current)
	if err != nil {
		t.Fatalf("chooseLowestJoinPlan() error = %v", err)
	}
	selection, ok := gotPlan.(*SelectPlan)
	if !ok {
		t.Fatalf("selected join plan type = %T, want *SelectPlan", gotPlan)
	}
	product, ok := selection.child.(*ProductPlan)
	if !ok {
		t.Fatalf("selected join input type = %T, want product fallback", selection.child)
	}
	selectedTable, ok := product.right.(*TablePlan)
	if !ok || selectedTable.tableName != "best_join" || product.left != current {
		t.Errorf("selected product = (%T, %T), want current on left and best_join on right", product.left, product.right)
	}
	if len(remaining) != 2 || remaining[0] != planners[0] || remaining[1] != planners[1] {
		t.Errorf("remaining planners = %v, want unrelated and first_join in source order", remaining)
	}
	for index, planner := range original {
		if planners[index] != planner {
			t.Errorf("input planner %d changed from %p to %p", index, planner, planners[index])
		}
	}
	if transaction.sizeCalls != 0 || len(transaction.pins) != 0 {
		t.Errorf("join choice accessed storage: size calls = %d, pins = %v", transaction.sizeCalls, transaction.pins)
	}
}

func TestChooseLowestJoinPlanKeepsFirstCandidateOnTies(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(1, 10))
	joinPredicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_department_id"),
		query.NewFieldExpression("department_id"),
	))
	planners := []*TablePlanner{
		newSelectCandidateWithField("first", "student_department_id", 4, joinPredicate, transaction),
		newSelectCandidateWithField("second", "student_department_id", 4, joinPredicate, transaction),
	}

	gotPlan, remaining, err := chooseLowestJoinPlan(planners, current)
	if err != nil {
		t.Fatalf("chooseLowestJoinPlan() error = %v", err)
	}
	selection, ok := gotPlan.(*SelectPlan)
	if !ok {
		t.Fatalf("selected join plan type = %T, want *SelectPlan", gotPlan)
	}
	product, ok := selection.child.(*ProductPlan)
	if !ok {
		t.Fatalf("selected join input type = %T, want product fallback", selection.child)
	}
	selectedTable, ok := product.right.(*TablePlan)
	if !ok {
		t.Fatalf("selected join right plan type = %T, want *TablePlan", product.right)
	}
	if got := selectedTable.tableName; got != "first" {
		t.Errorf("selected table = %q, want first source on an equal estimate", got)
	}
	if len(remaining) != 1 || remaining[0] != planners[1] {
		t.Errorf("remaining planners = %v, want only second", remaining)
	}
}

func TestChooseLowestJoinPlanPreservesCandidatesWhenNoJoinApplies(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(1, 10))
	planners := []*TablePlanner{
		newSelectCandidate("unrelated_first", 1, query.Predicate{}, transaction),
		newSelectCandidate("unrelated_second", 2, query.Predicate{}, transaction),
	}

	gotPlan, remaining, err := chooseLowestJoinPlan(planners, current)
	if err != nil {
		t.Fatalf("chooseLowestJoinPlan() error = %v", err)
	}
	if gotPlan != nil {
		t.Errorf("selected join plan = %T, want nil when no join predicate applies", gotPlan)
	}
	if len(remaining) != len(planners) || remaining[0] != planners[0] || remaining[1] != planners[1] {
		t.Errorf("remaining planners = %v, want all candidates in source order", remaining)
	}
}

func TestChooseLowestJoinPlanRejectsUnavailableCandidates(t *testing.T) {
	transaction := &tablePlanTransaction{}
	current := NewTablePlan(transaction, "department", record.NewLayout(newPlannerTestSchema("department_id")), metadata.NewStatInfo(1, 10))
	for _, test := range []struct {
		name     string
		planners []*TablePlanner
		current  Plan
		wantText string
	}{
		{name: "empty candidates", current: current, wantText: "no table planners"},
		{name: "nil current", planners: []*TablePlanner{newSelectCandidate("student", 1, query.Predicate{}, transaction)}, wantText: "current plan is nil"},
		{name: "typed nil current", planners: []*TablePlanner{newSelectCandidate("student", 1, query.Predicate{}, transaction)}, current: (*TablePlan)(nil), wantText: "current plan is nil"},
		{name: "nil planner", planners: []*TablePlanner{nil}, current: current, wantText: "table planner at index 0 is nil"},
		{name: "nil table plan", planners: []*TablePlanner{{tableName: "student"}}, current: current, wantText: `table plan for "student" is nil`},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, remaining, err := chooseLowestJoinPlan(test.planners, test.current)
			if plan != nil || remaining != nil || err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Errorf("chooseLowestJoinPlan() = (%v, %v, %v), want no plan, no remainder, and error containing %q", plan, remaining, err, test.wantText)
			}
		})
	}
}
