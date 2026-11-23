package plan

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

var _ Plan = contractProbe{}

type contractProbe struct{}

func (contractProbe) Open() (query.Scan, error) { return nil, nil }
func (contractProbe) BlocksAccessed() int       { return 2 }
func (contractProbe) RecordsOutput() int        { return 3 }
func (contractProbe) DistinctValues(string) int { return 1 }
func (contractProbe) Schema() *record.Schema    { return nil }

func TestPlanContractExposesScanEstimatesAndSchema(t *testing.T) {
	var plan Plan = contractProbe{}
	if scan, err := plan.Open(); scan != nil || err != nil {
		t.Fatalf("Open() = (%v, %v), want (nil, nil)", scan, err)
	}
	if plan.BlocksAccessed() != 2 {
		t.Errorf("BlocksAccessed() = %d, want 2", plan.BlocksAccessed())
	}
	if plan.RecordsOutput() != 3 {
		t.Errorf("RecordsOutput() = %d, want 3", plan.RecordsOutput())
	}
	if plan.DistinctValues("student_id") != 1 {
		t.Errorf("DistinctValues() = %d, want 1", plan.DistinctValues("student_id"))
	}
	if plan.Schema() != nil {
		t.Fatal("Schema() = non-nil, want nil probe schema")
	}
}

func TestTablePlanSchemaReturnsResolvedTableSchema(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	schema.AddVarcharField("student_name", 24)
	layout := record.NewLayout(&schema)

	tablePlan := NewTablePlan(nil, "student", layout, nil)
	if got := tablePlan.Schema(); got != &schema {
		t.Fatalf("Schema() = %p, want the schema stored in the layout %p", got, &schema)
	}
}

func TestTablePlanSchemaHandlesNilPlanAndLayout(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.Schema(); got != nil {
		t.Fatalf("nil TablePlan.Schema() = %p, want nil", got)
	}

	if got := NewTablePlan(nil, "student", nil, nil).Schema(); got != nil {
		t.Fatalf("TablePlan with nil layout Schema() = %p, want nil", got)
	}
}

func TestTablePlanOpenCreatesTableScan(t *testing.T) {
	transaction := &tablePlanTransaction{size: 1}
	var schema record.Schema
	schema.AddIntegerField("student_id")
	tablePlan := NewTablePlan(transaction, "student", record.NewLayout(&schema), nil)

	scan, err := tablePlan.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*query.RecordScan); !ok {
		t.Fatalf("Open() scan has type %T, want *query.RecordScan", scan)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("first Next() = (%t, %v), want (true, nil) from the first occupied slot", next, err)
	}
	if got, err := scan.GetInt("student_id"); err != nil || got != 42 {
		t.Fatalf("first row student_id = (%d, %v), want (42, nil)", got, err)
	}
	if got, want := transaction.pins, []storage.BlockID{storage.NewBlockID("student.tbl", 0)}; !equalPlanBlocks(got, want) {
		t.Fatalf("pinned blocks = %v, want %v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got, want := transaction.unpins, []storage.BlockID{storage.NewBlockID("student.tbl", 0)}; !equalPlanBlocks(got, want) {
		t.Fatalf("unpinned blocks = %v, want %v", got, want)
	}
}

func TestTablePlanOpenWrapsScanInitializationError(t *testing.T) {
	openErr := errors.New("file size unavailable")
	transaction := &tablePlanTransaction{sizeErr: openErr}
	var schema record.Schema
	schema.AddIntegerField("student_id")

	scan, err := NewTablePlan(transaction, "student", record.NewLayout(&schema), nil).Open()
	if scan != nil || !errors.Is(err, openErr) {
		t.Fatalf("Open() = (%v, %v), want (nil, wrapped file-size error)", scan, err)
	}
	if !strings.Contains(err.Error(), "student") {
		t.Fatalf("Open() error = %q, want table context", err)
	}
}

func TestTablePlanOpenRejectsNilPlanAndDependencies(t *testing.T) {
	var nilPlan *TablePlan
	if scan, err := nilPlan.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Fatalf("nil plan Open() = (%v, %v), want nil scan and nil-plan error", scan, err)
	}

	var schema record.Schema
	layout := record.NewLayout(&schema)
	if scan, err := NewTablePlan(nil, "student", layout, nil).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Fatalf("plan with nil transaction Open() = (%v, %v), want transaction error", scan, err)
	}
	if scan, err := NewTablePlan(&tablePlanTransaction{size: 1}, "student", nil, nil).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("plan with nil layout Open() = (%v, %v), want layout error", scan, err)
	}
}

func TestTablePlanBlocksAccessedReturnsStatistic(t *testing.T) {
	stats := metadata.NewStatInfo(12, 30)
	var schema record.Schema
	tablePlan := NewTablePlan(nil, "student", record.NewLayout(&schema), stats)
	if got := tablePlan.BlocksAccessed(); got != 12 {
		t.Fatalf("BlocksAccessed() = %d, want 12", got)
	}
}

func TestTablePlanBlocksAccessedHandlesNilPlanAndStatistics(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.BlocksAccessed(); got != 0 {
		t.Fatalf("nil TablePlan.BlocksAccessed() = %d, want 0", got)
	}
	if got := NewTablePlan(nil, "student", nil, nil).BlocksAccessed(); got != 0 {
		t.Fatalf("TablePlan with nil statistics BlocksAccessed() = %d, want 0", got)
	}
}

func TestTablePlanRecordsOutputReturnsStatistic(t *testing.T) {
	stats := metadata.NewStatInfo(12, 30)
	var schema record.Schema
	tablePlan := NewTablePlan(nil, "student", record.NewLayout(&schema), stats)
	if got := tablePlan.RecordsOutput(); got != 30 {
		t.Fatalf("RecordsOutput() = %d, want 30", got)
	}
}

func TestTablePlanRecordsOutputHandlesNilPlanAndStatistics(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.RecordsOutput(); got != 0 {
		t.Fatalf("nil TablePlan.RecordsOutput() = %d, want 0", got)
	}
	if got := NewTablePlan(nil, "student", nil, nil).RecordsOutput(); got != 0 {
		t.Fatalf("TablePlan with nil statistics RecordsOutput() = %d, want 0", got)
	}
}

func TestTablePlanDistinctValuesReturnsStatistic(t *testing.T) {
	stats := metadata.NewStatInfo(12, 30)
	var schema record.Schema
	tablePlan := NewTablePlan(nil, "student", record.NewLayout(&schema), stats)
	if got := tablePlan.DistinctValues("student_id"); got != 11 {
		t.Fatalf("DistinctValues(student_id) = %d, want 11", got)
	}
}

func TestTablePlanDistinctValuesHandlesNilPlanAndStatistics(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.DistinctValues("student_id"); got != 0 {
		t.Fatalf("nil TablePlan.DistinctValues(student_id) = %d, want 0", got)
	}
	if got := NewTablePlan(nil, "student", nil, nil).DistinctValues("student_id"); got != 0 {
		t.Fatalf("TablePlan without statistics DistinctValues(student_id) = %d, want 0", got)
	}
}

func TestSelectPlanOpenFiltersRowsFromChildPlan(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	child := NewTablePlan(&tablePlanTransaction{size: 1}, "student", record.NewLayout(&schema), nil)
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_id"),
		query.NewConstantExpression(query.NewIntConstant(42)),
	))

	scan, err := NewSelectPlan(child, predicate).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*query.SelectScan); !ok {
		t.Fatalf("Open() scan has type %T, want *query.SelectScan", scan)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("Next() = (%t, %v), want one matching row", next, err)
	}
	if got, err := scan.GetInt("student_id"); err != nil || got != 42 {
		t.Fatalf("GetInt(student_id) = (%d, %v), want (42, nil)", got, err)
	}
	if next, err := scan.Next(); err != nil || next {
		t.Fatalf("Next() after matching row = (%t, %v), want end of scan", next, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSelectPlanOpenFiltersOutNonmatchingRows(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	child := NewTablePlan(&tablePlanTransaction{size: 1}, "student", record.NewLayout(&schema), nil)
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_id"),
		query.NewConstantExpression(query.NewIntConstant(99)),
	))

	scan, err := NewSelectPlan(child, predicate).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if next, err := scan.Next(); err != nil || next {
		t.Fatalf("Next() = (%t, %v), want no matching rows", next, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSelectPlanOpenWrapsChildScanErrors(t *testing.T) {
	openErr := errors.New("file size unavailable")
	var schema record.Schema
	child := NewTablePlan(&tablePlanTransaction{sizeErr: openErr}, "student", record.NewLayout(&schema), nil)

	scan, err := NewSelectPlan(child, query.Predicate{}).Open()
	if scan != nil || !errors.Is(err, openErr) || !strings.Contains(err.Error(), "select plan") {
		t.Fatalf("Open() = (%v, %v), want nil scan and contextual wrapped error", scan, err)
	}
}

func TestSelectPlanOpenRejectsNilPlansAndChildScans(t *testing.T) {
	var nilPlan *SelectPlan
	if scan, err := nilPlan.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Fatalf("nil SelectPlan.Open() = (%v, %v), want nil scan and nil-plan error", scan, err)
	}
	if scan, err := NewSelectPlan(nil, query.Predicate{}).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "child plan") {
		t.Fatalf("SelectPlan without child Open() = (%v, %v), want nil scan and child-plan error", scan, err)
	}
	if scan, err := NewSelectPlan(contractProbe{}, query.Predicate{}).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "select scan") {
		t.Fatalf("SelectPlan with nil child scan Open() = (%v, %v), want nil scan and select-scan error", scan, err)
	}
}

func TestSelectPlanSchemaReturnsChildSchema(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	layout := record.NewLayout(&schema)
	selectPlan := NewSelectPlan(NewTablePlan(nil, "student", layout, nil), query.Predicate{})
	if got := selectPlan.Schema(); got != &schema {
		t.Fatalf("Schema() = %p, want child schema %p", got, &schema)
	}
}

func TestSelectPlanSchemaHandlesNilPlanAndChild(t *testing.T) {
	var nilPlan *SelectPlan
	if got := nilPlan.Schema(); got != nil {
		t.Fatalf("nil SelectPlan.Schema() = %p, want nil", got)
	}
	if got := NewSelectPlan(nil, query.Predicate{}).Schema(); got != nil {
		t.Fatalf("SelectPlan without child Schema() = %p, want nil", got)
	}
}

func TestSelectPlanBlocksAccessedReturnsChildEstimate(t *testing.T) {
	selectPlan := NewSelectPlan(contractProbe{}, query.Predicate{})
	if got := selectPlan.BlocksAccessed(); got != 2 {
		t.Fatalf("BlocksAccessed() = %d, want child estimate 2", got)
	}
}

func TestSelectPlanBlocksAccessedHandlesNilPlanAndChild(t *testing.T) {
	var nilPlan *SelectPlan
	if got := nilPlan.BlocksAccessed(); got != 0 {
		t.Fatalf("nil SelectPlan.BlocksAccessed() = %d, want 0", got)
	}
	if got := NewSelectPlan(nil, query.Predicate{}).BlocksAccessed(); got != 0 {
		t.Fatalf("SelectPlan without child BlocksAccessed() = %d, want 0", got)
	}
}

func TestSelectPlanRecordsOutputAppliesPredicateReductionFactor(t *testing.T) {
	var schema record.Schema
	child := NewTablePlan(nil, "student", record.NewLayout(&schema), metadata.NewStatInfo(3, 30))
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_id"),
		query.NewConstantExpression(query.NewIntConstant(42)),
	))

	if got := NewSelectPlan(child, predicate).RecordsOutput(); got != 2 {
		t.Fatalf("RecordsOutput() = %d, want 2 from 30 rows divided by an estimated factor of 11", got)
	}
}

func TestSelectPlanRecordsOutputPreservesRowsForEmptyPredicate(t *testing.T) {
	if got := NewSelectPlan(contractProbe{}, query.Predicate{}).RecordsOutput(); got != 3 {
		t.Fatalf("RecordsOutput() = %d, want the child estimate 3 for an empty predicate", got)
	}
}

func TestSelectPlanRecordsOutputHandlesNilPlansAndZeroFactor(t *testing.T) {
	var nilPlan *SelectPlan
	if got := nilPlan.RecordsOutput(); got != 0 {
		t.Fatalf("nil SelectPlan.RecordsOutput() = %d, want 0", got)
	}
	if got := NewSelectPlan(nil, query.Predicate{}).RecordsOutput(); got != 0 {
		t.Fatalf("SelectPlan without child RecordsOutput() = %d, want 0", got)
	}

	var schema record.Schema
	child := NewTablePlan(nil, "student", record.NewLayout(&schema), nil)
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("student_id"),
		query.NewConstantExpression(query.NewIntConstant(42)),
	))
	if got := NewSelectPlan(child, predicate).RecordsOutput(); got != 0 {
		t.Fatalf("RecordsOutput() with a zero reduction factor = %d, want 0", got)
	}
}

func TestSelectPlanDistinctValuesUsesPredicateEqualities(t *testing.T) {
	child := selectPlanEstimateProbe{"age": 8, "birth_year": 3, "score": 11}
	tests := []struct {
		name      string
		predicate query.Predicate
		fieldName string
		want      int
	}{
		{
			name: "field equals constant on left",
			predicate: query.NewPredicateWithTerm(query.NewTerm(
				query.NewFieldExpression("age"),
				query.NewConstantExpression(query.NewIntConstant(20)),
			)),
			fieldName: "age",
			want:      1,
		},
		{
			name: "field equals constant on right including zero-value constant",
			predicate: query.NewPredicateWithTerm(query.NewTerm(
				query.NewConstantExpression(query.Constant{}),
				query.NewFieldExpression("age"),
			)),
			fieldName: "age",
			want:      1,
		},
		{
			name: "field equality uses smaller estimate",
			predicate: query.NewPredicateWithTerm(query.NewTerm(
				query.NewFieldExpression("age"), query.NewFieldExpression("birth_year"),
			)),
			fieldName: "age",
			want:      3,
		},
		{
			name: "field equality uses smaller estimate when requested field is right",
			predicate: query.NewPredicateWithTerm(query.NewTerm(
				query.NewFieldExpression("age"), query.NewFieldExpression("birth_year"),
			)),
			fieldName: "birth_year",
			want:      3,
		},
		{
			name: "unrelated predicate delegates to child",
			predicate: query.NewPredicateWithTerm(query.NewTerm(
				query.NewFieldExpression("score"),
				query.NewConstantExpression(query.NewIntConstant(4)),
			)),
			fieldName: "age",
			want:      8,
		},
		{
			name:      "empty predicate delegates to child",
			predicate: query.Predicate{},
			fieldName: "score",
			want:      11,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NewSelectPlan(child, test.predicate).DistinctValues(test.fieldName); got != test.want {
				t.Fatalf("DistinctValues(%q) = %d, want %d", test.fieldName, got, test.want)
			}
		})
	}
}

func TestSelectPlanDistinctValuesHandlesNilPlansAndChildren(t *testing.T) {
	var nilPlan *SelectPlan
	if got := nilPlan.DistinctValues("age"); got != 0 {
		t.Fatalf("nil SelectPlan.DistinctValues(age) = %d, want 0", got)
	}
	if got := NewSelectPlan(nil, query.Predicate{}).DistinctValues("age"); got != 0 {
		t.Fatalf("SelectPlan without child DistinctValues(age) = %d, want 0", got)
	}
}

func TestProductPlanSchemaCombinesChildFieldsInOrder(t *testing.T) {
	var leftSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	leftSchema.AddVarcharField("student_name", 24)
	var rightSchema record.Schema
	rightSchema.AddIntegerField("course_id")
	rightSchema.AddVarcharField("course_name", 48)

	left := NewTablePlan(nil, "student", record.NewLayout(&leftSchema), nil)
	right := NewTablePlan(nil, "course", record.NewLayout(&rightSchema), nil)
	product := NewProductPlan(left, right)
	schema := product.Schema()
	if schema == nil {
		t.Fatal("Schema() = nil, want the combined schema")
	}
	if got, want := schema.Fields(), []string{"student_id", "student_name", "course_id", "course_name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, want)
	}
	if definition, ok := schema.Field("student_name"); !ok || definition != (record.FieldDefinition{Type: record.VarcharField, Length: 24}) {
		t.Fatalf("student_name definition = (%v, %v), want varchar length 24", definition, ok)
	}
	if definition, ok := schema.Field("course_name"); !ok || definition != (record.FieldDefinition{Type: record.VarcharField, Length: 48}) {
		t.Fatalf("course_name definition = (%v, %v), want varchar length 48", definition, ok)
	}
	if product.Schema() != schema {
		t.Fatal("Schema() returned a different schema pointer on subsequent calls")
	}
}

func TestProductPlanSchemaHandlesNilPlanAndChildren(t *testing.T) {
	var nilPlan *ProductPlan
	if got := nilPlan.Schema(); got != nil {
		t.Fatalf("nil ProductPlan.Schema() = %p, want nil", got)
	}

	var schema record.Schema
	schema.AddIntegerField("student_id")
	child := NewTablePlan(nil, "student", record.NewLayout(&schema), nil)
	for _, test := range []struct {
		name  string
		left  Plan
		right Plan
		want  []string
	}{
		{name: "both children missing"},
		{name: "only left child", left: child, want: []string{"student_id"}},
		{name: "only right child", right: child, want: []string{"student_id"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			product := NewProductPlan(test.left, test.right)
			if got := product.Schema(); got == nil || !reflect.DeepEqual(got.Fields(), test.want) {
				t.Fatalf("Schema() = %v, want fields %v", got, test.want)
			}
		})
	}
}

type selectPlanEstimateProbe map[string]int

func (selectPlanEstimateProbe) Open() (query.Scan, error) { return nil, nil }
func (selectPlanEstimateProbe) BlocksAccessed() int       { return 0 }
func (selectPlanEstimateProbe) RecordsOutput() int        { return 0 }
func (probe selectPlanEstimateProbe) DistinctValues(fieldName string) int {
	return probe[fieldName]
}
func (selectPlanEstimateProbe) Schema() *record.Schema { return nil }

type tablePlanTransaction struct {
	size    int
	sizeErr error
	pins    []storage.BlockID
	unpins  []storage.BlockID
}

func (t *tablePlanTransaction) Pin(block storage.BlockID) error {
	t.pins = append(t.pins, block)
	return nil
}

func (*tablePlanTransaction) BlockSize() (int, error) { return 512, nil }
func (*tablePlanTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	if offset == 0 {
		return 1, nil
	}
	return 42, nil
}
func (*tablePlanTransaction) GetString(storage.BlockID, int) (string, error)       { return "", nil }
func (*tablePlanTransaction) SetInt(storage.BlockID, int32, int32, bool) error     { return nil }
func (*tablePlanTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func (t *tablePlanTransaction) Size(string) (int, error) { return t.size, t.sizeErr }

func (*tablePlanTransaction) Append(fileName string) (storage.BlockID, error) {
	return storage.NewBlockID(fileName, 0), nil
}

func (t *tablePlanTransaction) Unpin(block storage.BlockID) error {
	t.unpins = append(t.unpins, block)
	return nil
}

func equalPlanBlocks(got, want []storage.BlockID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
