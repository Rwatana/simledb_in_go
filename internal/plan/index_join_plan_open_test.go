package plan

import (
	"errors"
	"strings"
	"testing"

	indexquery "github.com/Rwatana/simledb_in_go/internal/index/query"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestIndexJoinPlanOpenReturnsInitializedIndexJoinScan(t *testing.T) {
	join, left, right, indexTransaction, dataTransaction := newIndexJoinPlanOpenFixture(t)

	scan, err := join.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*indexquery.IndexJoinScan); !ok {
		t.Fatalf("Open() scan type = %T, want *indexquery.IndexJoinScan", scan)
	}
	if left.opens != 1 || right.opens != 1 {
		t.Fatalf("child opens = (%d, %d), want (1, 1)", left.opens, right.opens)
	}
	if len(indexTransaction.pins) != 1 {
		t.Fatalf("index bucket pins = %v, want one bucket", indexTransaction.pins)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if len(indexTransaction.unpins) != 1 || len(dataTransaction.unpins) != 1 {
		t.Fatalf("unpins: index=%v data=%v, want one each", indexTransaction.unpins, dataTransaction.unpins)
	}
}

func TestIndexJoinPlanOpenRejectsMissingDependencies(t *testing.T) {
	join, left, right, _, _ := newIndexJoinPlanOpenFixture(t)
	tests := []struct {
		name string
		plan *IndexJoinPlan
		want string
	}{
		{name: "nil receiver", plan: nil, want: "nil index join plan"},
		{name: "nil left", plan: NewIndexJoinPlan(nil, right, join.indexInfo, "student_id"), want: "left plan is nil"},
		{name: "nil right", plan: NewIndexJoinPlan(left, nil, join.indexInfo, "student_id"), want: "right plan is nil"},
		{name: "nil index metadata", plan: NewIndexJoinPlan(left, right, nil, "student_id"), want: "index metadata is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := test.plan.Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Open() = (%v, %v), want nil scan and error containing %q", scan, err, test.want)
			}
		})
	}
}

func TestIndexJoinPlanOpenPreservesLeftOpenAndCleanupFailures(t *testing.T) {
	openErr := errors.New("left open failed")
	closeErr := errors.New("left cleanup failed")
	partialScan := &productPlanScanStub{closeErr: closeErr}
	left := &productPlanProbe{scan: partialScan, openErr: openErr}
	join, _, right, _, _ := newIndexJoinPlanOpenFixture(t)
	join.left = left
	join.right = right

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, openErr) || !errors.Is(err, closeErr) {
		t.Fatalf("Open() = (%v, %v), want both left-open and cleanup causes", scan, err)
	}
	if partialScan.closeCalls != 1 || right.opens != 0 {
		t.Fatalf("left closes=%d, right opens=%d, want (1, 0)", partialScan.closeCalls, right.opens)
	}
}

func TestIndexJoinPlanOpenClosesPartialLeftScanWhenOpenFails(t *testing.T) {
	openErr := errors.New("left open failed after creating scan")
	partialScan := &productPlanScanStub{}
	left := &productPlanProbe{scan: partialScan, openErr: openErr}
	join, _, right, _, _ := newIndexJoinPlanOpenFixture(t)
	join.left = left
	join.right = right

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, openErr) {
		t.Fatalf("Open() = (%v, %v), want wrapped left-open error", scan, err)
	}
	if partialScan.closeCalls != 1 || right.opens != 0 {
		t.Fatalf("left closes=%d, right opens=%d, want (1, 0)", partialScan.closeCalls, right.opens)
	}
}

func TestIndexJoinPlanOpenHandlesLeftFailureWithoutPartialScan(t *testing.T) {
	openErr := errors.New("left open failed without a scan")
	left := &productPlanProbe{openErr: openErr}
	join, _, right, _, _ := newIndexJoinPlanOpenFixture(t)
	join.left = left
	join.right = right

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, openErr) {
		t.Fatalf("Open() = (%v, %v), want wrapped left-open error", scan, err)
	}
	if right.opens != 0 {
		t.Fatalf("right Open() calls = %d, want zero", right.opens)
	}
}

func TestIndexJoinPlanOpenRejectsNilLeftScan(t *testing.T) {
	join, left, _, _, _ := newIndexJoinPlanOpenFixture(t)
	var typedNil *productPlanScanStub
	for name, leftScan := range map[string]query.Scan{"nil": nil, "typed nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			left.scan = leftScan
			scan, err := join.Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), "left scan is nil") {
				t.Fatalf("Open() = (%v, %v), want nil scan and missing-left-scan error", scan, err)
			}
			if join.right.(*productPlanProbe).opens != 0 {
				t.Fatal("right plan opened after the left plan returned no scan")
			}
		})
	}
}

func TestIndexJoinPlanOpenClosesBothScansWhenRightOpenFails(t *testing.T) {
	openErr := errors.New("right open failed")
	leftCloseErr := errors.New("left close failed")
	rightCloseErr := errors.New("partial right close failed")
	leftScan := &productPlanScanStub{closeErr: leftCloseErr}
	rightScan := &productPlanScanStub{closeErr: rightCloseErr}
	left := &productPlanProbe{scan: leftScan}
	right := &productPlanProbe{scan: rightScan, openErr: openErr}
	join, _, _, _, _ := newIndexJoinPlanOpenFixture(t)
	join.left = left
	join.right = right

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, openErr) || !errors.Is(err, leftCloseErr) || !errors.Is(err, rightCloseErr) {
		t.Fatalf("Open() = (%v, %v), want open and both cleanup causes", scan, err)
	}
	if leftScan.closeCalls != 1 || rightScan.closeCalls != 1 {
		t.Fatalf("close calls: left=%d right=%d, want one each", leftScan.closeCalls, rightScan.closeCalls)
	}
}

func TestIndexJoinPlanOpenSkipsUninitializedRightScanDuringCleanup(t *testing.T) {
	openErr := errors.New("right open returned before initialization")
	leftScan := &productPlanScanStub{}
	left := &productPlanProbe{scan: leftScan}
	right := &productPlanProbe{scan: &query.RecordScan{}, openErr: openErr}
	join, _, _, _, _ := newIndexJoinPlanOpenFixture(t)
	join.left = left
	join.right = right

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, openErr) {
		t.Fatalf("Open() = (%v, %v), want wrapped right-open error", scan, err)
	}
	if leftScan.closeCalls != 1 {
		t.Fatalf("left scan close calls = %d, want one", leftScan.closeCalls)
	}
}

func TestIndexJoinPlanOpenRejectsNilAndNonRecordRightScans(t *testing.T) {
	join, left, _, _, _ := newIndexJoinPlanOpenFixture(t)
	var typedNil *query.RecordScan
	tests := []struct {
		name  string
		scan  query.Scan
		want  string
		close *productPlanScanStub
	}{
		{name: "nil scan", scan: nil, want: "right scan is nil"},
		{name: "typed nil record scan", scan: typedNil, want: "right table scan is nil"},
		{name: "record scan without table", scan: &query.RecordScan{}, want: "right table scan is nil"},
		{name: "non-record scan", scan: &productPlanScanStub{}, want: "is not a query record scan"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			leftScan := &productPlanScanStub{}
			left.scan = leftScan
			join.left = left
			join.right = &productPlanProbe{scan: test.scan}

			scan, err := join.Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Open() = (%v, %v), want nil scan and error containing %q", scan, err, test.want)
			}
			if leftScan.closeCalls != 1 {
				t.Fatalf("left scan close calls = %d, want one", leftScan.closeCalls)
			}
		})
	}
}

func TestIndexJoinPlanOpenClosesScansAfterInitializationFailure(t *testing.T) {
	positionErr := errors.New("left before-first failed")
	leftCloseErr := errors.New("left close after positioning failed")
	leftScan := &productPlanScanStub{beforeFirstErr: positionErr, closeErr: leftCloseErr}
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	rightSchema.AddIntegerField("course_id")
	dataTransaction := &tablePlanTransaction{size: 1}
	rightScan, err := query.NewRecordScan(dataTransaction, "course", record.NewLayout(&rightSchema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	indexTransaction := &tablePlanTransaction{size: 1}
	indexInfo := newIndexSelectPlanTestInfo(t, indexTransaction, &rightSchema, "course_idx", "course_id")
	join := NewIndexJoinPlan(&productPlanProbe{scan: leftScan, schema: &leftSchema}, &productPlanProbe{scan: rightScan, schema: &rightSchema}, indexInfo, "student_id")

	scan, err := join.Open()
	if scan != nil || !errors.Is(err, positionErr) || !errors.Is(err, leftCloseErr) {
		t.Fatalf("Open() = (%v, %v), want position and left-cleanup causes", scan, err)
	}
	if leftScan.closeCalls != 1 || len(dataTransaction.unpins) != 1 {
		t.Fatalf("cleanup: left closes=%d right unpins=%v, want one each", leftScan.closeCalls, dataTransaction.unpins)
	}
}

func TestIndexJoinPlanOpenPreservesLeftAdvanceAndJoinValueFailures(t *testing.T) {
	advanceErr := errors.New("left advance failed")
	valueErr := errors.New("join value read failed")
	cases := []struct {
		name string
		scan *indexJoinPlanOpenLeftScan
		err  error
	}{
		{name: "left advance", scan: &indexJoinPlanOpenLeftScan{productPlanScanStub: &productPlanScanStub{}, nextErr: advanceErr}, err: advanceErr},
		{name: "join value", scan: &indexJoinPlanOpenLeftScan{productPlanScanStub: &productPlanScanStub{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(4)}}}, getValErr: valueErr}, err: valueErr},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			join, left, _, _, dataTransaction := newIndexJoinPlanOpenFixture(t)
			left.scan = test.scan

			scan, err := join.Open()
			if scan != nil || !errors.Is(err, test.err) || !strings.Contains(err.Error(), "open index join plan scan") {
				t.Fatalf("Open() = (%v, %v), want wrapped initialization cause", scan, err)
			}
			if test.scan.closeCalls != 1 || len(dataTransaction.unpins) != 1 {
				t.Fatalf("cleanup: left closes=%d right unpins=%v, want one each", test.scan.closeCalls, dataTransaction.unpins)
			}
		})
	}
}

func TestIndexJoinPlanOpenPreservesIndexPositionFailure(t *testing.T) {
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	rightSchema.AddIntegerField("course_id")
	leftScan := &productPlanScanStub{rows: []map[string]query.Constant{{"student_id": {}}}}
	dataTransaction := &tablePlanTransaction{size: 1}
	rightScan, err := query.NewRecordScan(dataTransaction, "course", record.NewLayout(&rightSchema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	indexTransaction := &tablePlanTransaction{size: 1}
	indexInfo := newIndexSelectPlanTestInfo(t, indexTransaction, &rightSchema, "course_idx", "course_id")
	join := NewIndexJoinPlan(&productPlanProbe{scan: leftScan, schema: &leftSchema}, &productPlanProbe{scan: rightScan, schema: &rightSchema}, indexInfo, "student_id")

	scan, err := join.Open()
	if scan != nil || err == nil || !strings.Contains(err.Error(), "initialize index join scan") {
		t.Fatalf("Open() = (%v, %v), want index-positioning error", scan, err)
	}
	if len(dataTransaction.unpins) != 1 {
		t.Fatalf("right table unpins = %v, want one after index positioning failure", dataTransaction.unpins)
	}
}

func newIndexJoinPlanOpenFixture(t *testing.T) (*IndexJoinPlan, *productPlanProbe, *productPlanProbe, *tablePlanTransaction, *tablePlanTransaction) {
	t.Helper()
	var leftSchema, rightSchema record.Schema
	leftSchema.AddIntegerField("student_id")
	rightSchema.AddIntegerField("course_id")
	leftScan := &productPlanScanStub{
		rows:   []map[string]query.Constant{{"student_id": query.NewIntConstant(7)}},
		fields: map[string]bool{"student_id": true},
	}
	dataTransaction := &tablePlanTransaction{size: 1}
	rightScan, err := query.NewRecordScan(dataTransaction, "course", record.NewLayout(&rightSchema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	indexTransaction := &tablePlanTransaction{size: 1}
	indexInfo := newIndexSelectPlanTestInfo(t, indexTransaction, &rightSchema, "course_idx", "course_id")
	left := &productPlanProbe{scan: leftScan, schema: &leftSchema}
	right := &productPlanProbe{scan: rightScan, schema: &rightSchema}
	return NewIndexJoinPlan(left, right, indexInfo, "student_id"), left, right, indexTransaction, dataTransaction
}

type indexJoinPlanOpenLeftScan struct {
	*productPlanScanStub
	nextErr   error
	getValErr error
}

func (scan *indexJoinPlanOpenLeftScan) Next() (bool, error) {
	if scan.nextErr != nil {
		return false, scan.nextErr
	}
	return scan.productPlanScanStub.Next()
}

func (scan *indexJoinPlanOpenLeftScan) GetVal(fieldName string) (query.Constant, error) {
	if scan.getValErr != nil {
		return query.Constant{}, scan.getValErr
	}
	return scan.productPlanScanStub.GetVal(fieldName)
}
