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

func TestProductPlanOpenStartsLeftMajorProductScan(t *testing.T) {
	left := &productPlanScanStub{
		rows:   []map[string]query.Constant{{"left_id": query.NewIntConstant(7)}},
		fields: map[string]bool{"left_id": true},
	}
	right := &productPlanScanStub{
		rows:   []map[string]query.Constant{{"right_id": query.NewStringConstant("Ada")}},
		fields: map[string]bool{"right_id": true},
	}
	leftPlan := &productPlanProbe{scan: left}
	rightPlan := &productPlanProbe{scan: right}

	scan, err := NewProductPlan(leftPlan, rightPlan).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*query.ProductScan); !ok {
		t.Fatalf("Open() scan has type %T, want *query.ProductScan", scan)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("first Next() = (%t, %v), want (true, nil)", next, err)
	}
	if got, err := scan.GetInt("left_id"); err != nil || got != 7 {
		t.Errorf("GetInt(left_id) = (%d, %v), want (7, nil)", got, err)
	}
	if got, err := scan.GetString("right_id"); err != nil || got != "Ada" {
		t.Errorf("GetString(right_id) = (%q, %v), want (Ada, nil)", got, err)
	}
	if leftPlan.opens != 1 || rightPlan.opens != 1 {
		t.Errorf("child Open() calls = (%d, %d), want (1, 1)", leftPlan.opens, rightPlan.opens)
	}
	if left.beforeFirstCalls != 1 || right.beforeFirstCalls != 1 {
		t.Errorf("child BeforeFirst() calls = (%d, %d), want (1, 1)", left.beforeFirstCalls, right.beforeFirstCalls)
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestProductPlanOpenRejectsNilPlanAndChildPlans(t *testing.T) {
	var nilPlan *ProductPlan
	if scan, err := nilPlan.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Errorf("nil ProductPlan.Open() = (%v, %v), want nil scan and nil-plan error", scan, err)
	}
	if scan, err := NewProductPlan(nil, &productPlanProbe{}).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "left child plan") {
		t.Errorf("ProductPlan without left child Open() = (%v, %v), want left-plan error", scan, err)
	}

	leftPlan := &productPlanProbe{scan: &productPlanScanStub{}}
	if scan, err := NewProductPlan(leftPlan, nil).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "right child plan") {
		t.Errorf("ProductPlan without right child Open() = (%v, %v), want right-plan error", scan, err)
	}
	if leftPlan.opens != 0 {
		t.Errorf("left child Open() calls = %d, want 0 when a child plan is absent", leftPlan.opens)
	}
}

func TestProductPlanOpenWrapsLeftPlanErrorWithoutOpeningRight(t *testing.T) {
	openErr := errors.New("left child failed")
	leftPlan := &productPlanProbe{openErr: openErr}
	rightPlan := &productPlanProbe{scan: &productPlanScanStub{}}

	scan, err := NewProductPlan(leftPlan, rightPlan).Open()
	if scan != nil || !errors.Is(err, openErr) || !strings.Contains(err.Error(), "left child") {
		t.Fatalf("Open() = (%v, %v), want wrapped left child error", scan, err)
	}
	if rightPlan.opens != 0 {
		t.Errorf("right child Open() calls = %d, want 0 after left failure", rightPlan.opens)
	}
}

func TestProductPlanOpenClosesLeftAfterRightPlanError(t *testing.T) {
	openErr := errors.New("right child failed")
	closeErr := errors.New("left scan close failed")
	leftScan := &productPlanScanStub{closeErr: closeErr}
	leftPlan := &productPlanProbe{scan: leftScan}
	rightPlan := &productPlanProbe{openErr: openErr}

	scan, err := NewProductPlan(leftPlan, rightPlan).Open()
	if scan != nil || !errors.Is(err, openErr) || !errors.Is(err, closeErr) ||
		!strings.Contains(err.Error(), "right child") || !strings.Contains(err.Error(), "close left") {
		t.Fatalf("Open() = (%v, %v), want contextual right-open and left-close errors", scan, err)
	}
	if leftScan.closeCalls != 1 {
		t.Errorf("left child Close() calls = %d, want 1 after right-open failure", leftScan.closeCalls)
	}
}

func TestProductPlanOpenRejectsMissingScansAndClosesAvailableInputs(t *testing.T) {
	t.Run("left scan is nil", func(t *testing.T) {
		rightScan := &productPlanScanStub{}
		rightPlan := &productPlanProbe{scan: rightScan}
		scan, err := NewProductPlan(&productPlanProbe{}, rightPlan).Open()
		if scan != nil || err == nil || !strings.Contains(err.Error(), "left child scan is nil") {
			t.Fatalf("Open() = (%v, %v), want nil scan and left-scan error", scan, err)
		}
		if rightPlan.opens != 0 || rightScan.closeCalls != 0 {
			t.Errorf("right child Open()/Close() calls = (%d, %d), want (0, 0) after missing left scan", rightPlan.opens, rightScan.closeCalls)
		}
	})

	t.Run("right scan is nil", func(t *testing.T) {
		leftScan := &productPlanScanStub{}
		scan, err := NewProductPlan(&productPlanProbe{scan: leftScan}, &productPlanProbe{}).Open()
		if scan != nil || err == nil || !strings.Contains(err.Error(), "right scan is nil") {
			t.Fatalf("Open() = (%v, %v), want nil scan and right-scan error", scan, err)
		}
		if leftScan.closeCalls != 1 {
			t.Errorf("left child Close() calls = %d, want 1 after construction failure", leftScan.closeCalls)
		}
	})
}

func TestProductPlanOpenClosesBothScansAfterInitializationFailure(t *testing.T) {
	leftErr := errors.New("left reset failed")
	rightErr := errors.New("right reset failed")
	leftCloseErr := errors.New("left close failed")
	rightCloseErr := errors.New("right close failed")
	leftScan := &productPlanScanStub{beforeFirstErr: leftErr, closeErr: leftCloseErr}
	rightScan := &productPlanScanStub{beforeFirstErr: rightErr, closeErr: rightCloseErr}

	scan, err := NewProductPlan(&productPlanProbe{scan: leftScan}, &productPlanProbe{scan: rightScan}).Open()
	for _, want := range []error{leftErr, rightErr, leftCloseErr, rightCloseErr} {
		if !errors.Is(err, want) {
			t.Errorf("Open() error = %v, want cause %v", err, want)
		}
	}
	if scan != nil || !strings.Contains(err.Error(), "initialize product scan") {
		t.Fatalf("Open() = (%v, %v), want nil scan and initialization context", scan, err)
	}
	if leftScan.closeCalls != 1 || rightScan.closeCalls != 1 {
		t.Errorf("child Close() calls = (%d, %d), want (1, 1)", leftScan.closeCalls, rightScan.closeCalls)
	}
}

func TestProductPlanBlocksAccessedUsesNestedLoopEstimate(t *testing.T) {
	left := &productPlanProbe{blocks: 3, records: 4}
	right := &productPlanProbe{blocks: 5, records: 6}
	if got, want := NewProductPlan(left, right).BlocksAccessed(), 23; got != want {
		t.Fatalf("BlocksAccessed() = %d, want %d", got, want)
	}
}

func TestProductPlanBlocksAccessedHandlesNilPlanAndChildren(t *testing.T) {
	var nilPlan *ProductPlan
	if got := nilPlan.BlocksAccessed(); got != 0 {
		t.Errorf("nil ProductPlan.BlocksAccessed() = %d, want 0", got)
	}
	if got := NewProductPlan(nil, &productPlanProbe{blocks: 5}).BlocksAccessed(); got != 0 {
		t.Errorf("ProductPlan without left child BlocksAccessed() = %d, want 0", got)
	}
	if got := NewProductPlan(&productPlanProbe{blocks: 3, records: 4}, nil).BlocksAccessed(); got != 0 {
		t.Errorf("ProductPlan without right child BlocksAccessed() = %d, want 0", got)
	}
}

func TestProductPlanBlocksAccessedUsesJavaInt32Arithmetic(t *testing.T) {
	maxInt32 := int(^uint32(0) >> 1)
	tests := []struct {
		name  string
		left  *productPlanProbe
		right *productPlanProbe
		want  int
	}{
		{
			name:  "multiplication wraps before addition",
			left:  &productPlanProbe{blocks: 10, records: maxInt32},
			right: &productPlanProbe{blocks: 2},
			want:  8,
		},
		{
			name:  "addition wraps",
			left:  &productPlanProbe{blocks: maxInt32, records: 1},
			right: &productPlanProbe{blocks: 1},
			want:  int(int32(maxInt32 + 1)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NewProductPlan(test.left, test.right).BlocksAccessed(); got != test.want {
				t.Errorf("BlocksAccessed() = %d, want Java int result %d", got, test.want)
			}
		})
	}
}

func TestProductPlanRecordsOutputMultipliesChildEstimates(t *testing.T) {
	left := &productPlanProbe{records: 4}
	right := &productPlanProbe{records: 6}
	if got, want := NewProductPlan(left, right).RecordsOutput(), 24; got != want {
		t.Fatalf("RecordsOutput() = %d, want %d", got, want)
	}
}

func TestProductPlanRecordsOutputHandlesNilPlanAndChildren(t *testing.T) {
	var nilPlan *ProductPlan
	if got := nilPlan.RecordsOutput(); got != 0 {
		t.Errorf("nil ProductPlan.RecordsOutput() = %d, want 0", got)
	}
	if got := NewProductPlan(nil, &productPlanProbe{records: 6}).RecordsOutput(); got != 0 {
		t.Errorf("ProductPlan without left child RecordsOutput() = %d, want 0", got)
	}
	if got := NewProductPlan(&productPlanProbe{records: 4}, nil).RecordsOutput(); got != 0 {
		t.Errorf("ProductPlan without right child RecordsOutput() = %d, want 0", got)
	}
}

func TestProductPlanRecordsOutputUsesJavaInt32Arithmetic(t *testing.T) {
	maxInt32 := int(^uint32(0) >> 1)
	product := NewProductPlan(
		&productPlanProbe{records: maxInt32},
		&productPlanProbe{records: 2},
	)
	if got, want := product.RecordsOutput(), -2; got != want {
		t.Fatalf("RecordsOutput() = %d, want Java int result %d", got, want)
	}
}

func TestProductPlanDistinctValuesDelegatesByChildSchema(t *testing.T) {
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	rightSchema.AddVarcharField("student_name", 24)
	left := &productPlanProbe{schema: &leftSchema, distinct: map[string]int{"student_id": 7}}
	right := &productPlanProbe{schema: &rightSchema, distinct: map[string]int{"student_name": 9}}
	product := NewProductPlan(left, right)
	for field, want := range map[string]int{"student_id": 7, "student_name": 9} {
		if got := product.DistinctValues(field); got != want {
			t.Errorf("DistinctValues(%q) = %d, want %d", field, got, want)
		}
	}
	if got := product.DistinctValues("missing"); got != 0 {
		t.Errorf("DistinctValues(missing) = %d, want 0", got)
	}
}

func TestProductPlanDistinctValuesPrefersLeftForDuplicateField(t *testing.T) {
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("shared")
	rightSchema.AddIntegerField("shared")
	left := &productPlanProbe{schema: &leftSchema, distinct: map[string]int{"shared": 3}}
	right := &productPlanProbe{schema: &rightSchema, distinct: map[string]int{"shared": 8}}
	if got := NewProductPlan(left, right).DistinctValues("shared"); got != 3 {
		t.Fatalf("DistinctValues(shared) = %d, want left estimate 3", got)
	}
}

func TestProductPlanDistinctValuesHandlesNilPlanChildrenAndSchemas(t *testing.T) {
	var nilPlan *ProductPlan
	if got := nilPlan.DistinctValues("id"); got != 0 {
		t.Errorf("nil ProductPlan.DistinctValues(id) = %d, want 0", got)
	}
	var schema record.Schema
	child := &productPlanProbe{schema: &schema, distinct: map[string]int{"id": 3}}
	if got := NewProductPlan(nil, child).DistinctValues("id"); got != 0 {
		t.Errorf("ProductPlan without left child DistinctValues(id) = %d, want 0", got)
	}
	if got := NewProductPlan(child, nil).DistinctValues("id"); got != 0 {
		t.Errorf("ProductPlan without right child DistinctValues(id) = %d, want 0", got)
	}
	if got := NewProductPlan(&productPlanProbe{}, child).DistinctValues("id"); got != 0 {
		t.Errorf("ProductPlan with missing left schema DistinctValues(id) = %d, want 0", got)
	}
	if got := NewProductPlan(child, &productPlanProbe{}).DistinctValues("id"); got != 0 {
		t.Errorf("ProductPlan with missing right schema DistinctValues(id) = %d, want 0", got)
	}
}

func TestProjectPlanSchemaCopiesRequestedFieldsInOrder(t *testing.T) {
	var childSchema record.Schema
	childSchema.AddIntegerField("student_id")
	childSchema.AddVarcharField("student_name", 24)
	childSchema.AddIntegerField("grade")
	child := &productPlanProbe{schema: &childSchema}
	project, err := NewProjectPlan(child, []string{"student_name", "student_id"})
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	schema := project.Schema()
	if schema == nil {
		t.Fatal("Schema() = nil, want projected schema")
	}
	if got, want := schema.Fields(), []string{"student_name", "student_id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Schema().Fields() = %v, want %v", got, want)
	}
	if got, want := schema.Field("student_name"); !want || got != (record.FieldDefinition{Type: record.VarcharField, Length: 24}) {
		t.Errorf("student_name definition = (%v, %t), want varchar length 24", got, want)
	}
	if got, want := schema.Field("student_id"); !want || got != (record.FieldDefinition{Type: record.IntegerField}) {
		t.Errorf("student_id definition = (%v, %t), want integer", got, want)
	}
	if project.Schema() != schema {
		t.Fatal("Schema() returned a different pointer on a later call")
	}
	childSchema.AddIntegerField("later_field")
	if schema.HasField("later_field") {
		t.Error("projected schema changed after the child schema was extended")
	}
}

func TestProjectPlanSchemaHandlesEmptyFieldsAndNilReceiver(t *testing.T) {
	var childSchema record.Schema
	childSchema.AddIntegerField("student_id")
	project, err := NewProjectPlan(&productPlanProbe{schema: &childSchema}, nil)
	if err != nil {
		t.Fatalf("NewProjectPlan() with no fields error = %v", err)
	}
	if got := project.Schema().Fields(); len(got) != 0 {
		t.Errorf("empty projection fields = %v, want none", got)
	}
	var nilPlan *ProjectPlan
	if got := nilPlan.Schema(); got != nil {
		t.Errorf("nil ProjectPlan.Schema() = %p, want nil", got)
	}
}

func TestNewProjectPlanRejectsNilDependenciesAndUnknownFields(t *testing.T) {
	if project, err := NewProjectPlan(nil, []string{"id"}); project != nil || err == nil || !strings.Contains(err.Error(), "child plan") {
		t.Errorf("NewProjectPlan(nil) = (%v, %v), want child-plan error", project, err)
	}
	if project, err := NewProjectPlan(&productPlanProbe{}, []string{"id"}); project != nil || err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("NewProjectPlan(nil schema) = (%v, %v), want schema error", project, err)
	}
	var childSchema record.Schema
	childSchema.AddIntegerField("id")
	if project, err := NewProjectPlan(&productPlanProbe{schema: &childSchema}, []string{"missing"}); project != nil || err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("NewProjectPlan(missing field) = (%v, %v), want field-specific error", project, err)
	}
}

func TestProjectPlanOpenReturnsProjectedScanAtChildPosition(t *testing.T) {
	var childSchema record.Schema
	childSchema.AddIntegerField("student_id")
	childSchema.AddVarcharField("student_name", 24)
	source := &productPlanScanStub{
		rows: []map[string]query.Constant{{
			"student_id":   query.NewIntConstant(42),
			"student_name": query.NewStringConstant("Ada"),
		}},
		fields:   map[string]bool{"student_id": true, "student_name": true},
		position: -1,
	}
	child := &productPlanProbe{schema: &childSchema, scan: source}
	project, err := NewProjectPlan(child, []string{"student_id"})
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	scan, err := project.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*query.ProjectScan); !ok {
		t.Fatalf("Open() scan has type %T, want *query.ProjectScan", scan)
	}
	if !scan.HasField("student_id") || scan.HasField("student_name") {
		t.Error("Open() scan fields do not match the projection")
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("first Next() = (%t, %v), want (true, nil) without resetting the child", next, err)
	}
	if got, err := scan.GetInt("student_id"); err != nil || got != 42 {
		t.Errorf("GetInt(student_id) = (%d, %v), want (42, nil)", got, err)
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if source.closeCalls != 1 {
		t.Errorf("child scan Close() calls = %d, want 1", source.closeCalls)
	}
}

func TestProjectPlanOpenRejectsNilPlanAndChild(t *testing.T) {
	var nilPlan *ProjectPlan
	if scan, err := nilPlan.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Errorf("nil ProjectPlan.Open() = (%v, %v), want nil scan and nil-plan error", scan, err)
	}
	if scan, err := (&ProjectPlan{}).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "child plan") {
		t.Errorf("ProjectPlan without child Open() = (%v, %v), want child-plan error", scan, err)
	}
}

func TestProjectPlanOpenWrapsChildErrorsAndRejectsNilScan(t *testing.T) {
	var childSchema record.Schema
	childSchema.AddIntegerField("id")
	openErr := errors.New("child open failed")
	project, err := NewProjectPlan(&productPlanProbe{schema: &childSchema, openErr: openErr}, []string{"id"})
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	if scan, err := project.Open(); scan != nil || !errors.Is(err, openErr) || !strings.Contains(err.Error(), "project plan child") {
		t.Errorf("Open() = (%v, %v), want wrapped child-open error", scan, err)
	}

	project, err = NewProjectPlan(&productPlanProbe{schema: &childSchema}, []string{"id"})
	if err != nil {
		t.Fatalf("NewProjectPlan() for nil child scan error = %v", err)
	}
	if scan, err := project.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "project scan") {
		t.Errorf("Open() with nil child scan = (%v, %v), want project-scan error", scan, err)
	}
}

type productPlanProbe struct {
	scan     query.Scan
	openErr  error
	opens    int
	blocks   int
	records  int
	schema   *record.Schema
	distinct map[string]int
}

func (plan *productPlanProbe) Open() (query.Scan, error) {
	plan.opens++
	return plan.scan, plan.openErr
}
func (plan *productPlanProbe) BlocksAccessed() int                 { return plan.blocks }
func (plan *productPlanProbe) RecordsOutput() int                  { return plan.records }
func (plan *productPlanProbe) DistinctValues(fieldName string) int { return plan.distinct[fieldName] }
func (plan *productPlanProbe) Schema() *record.Schema              { return plan.schema }

type productPlanScanStub struct {
	rows             []map[string]query.Constant
	fields           map[string]bool
	position         int
	beforeFirstErr   error
	closeErr         error
	beforeFirstCalls int
	closeCalls       int
}

func (scan *productPlanScanStub) BeforeFirst() error {
	scan.beforeFirstCalls++
	scan.position = -1
	return scan.beforeFirstErr
}
func (scan *productPlanScanStub) Next() (bool, error) {
	scan.position++
	return scan.position < len(scan.rows), nil
}
func (scan *productPlanScanStub) GetInt(fieldName string) (int32, error) {
	value, ok := scan.rows[scan.position][fieldName]
	if !ok {
		return 0, errors.New("integer field is missing")
	}
	integer, ok := value.AsInt()
	if !ok {
		return 0, errors.New("field is not an integer")
	}
	return integer, nil
}
func (scan *productPlanScanStub) GetString(fieldName string) (string, error) {
	value, ok := scan.rows[scan.position][fieldName]
	if !ok {
		return "", errors.New("string field is missing")
	}
	text, ok := value.AsString()
	if !ok {
		return "", errors.New("field is not a string")
	}
	return text, nil
}
func (scan *productPlanScanStub) GetVal(fieldName string) (query.Constant, error) {
	value, ok := scan.rows[scan.position][fieldName]
	if !ok {
		return query.Constant{}, errors.New("field is missing")
	}
	return value, nil
}
func (scan *productPlanScanStub) HasField(fieldName string) bool { return scan.fields[fieldName] }
func (scan *productPlanScanStub) Close() error {
	scan.closeCalls++
	return scan.closeErr
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
