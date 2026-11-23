package plan

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/index/hash"
	indexquery "github.com/Rwatana/simledb_in_go/internal/index/query"
	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestIndexSelectPlanOpenReturnsPositionedIndexSelectScan(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	dataTransaction := &tablePlanTransaction{size: 1}
	dataScan, err := query.NewRecordScan(dataTransaction, "student", record.NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	indexTransaction := &tablePlanTransaction{size: 1}
	indexInfo := newIndexSelectPlanTestInfo(t, indexTransaction, &schema, "student_idx", "student_id")
	child := &productPlanProbe{scan: dataScan}
	searchKey := query.NewIntConstant(6)

	scan, err := NewIndexSelectPlan(child, indexInfo, searchKey).Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*indexquery.IndexSelectScan); !ok {
		t.Fatalf("Open() scan type = %T, want *indexquery.IndexSelectScan", scan)
	}
	if child.opens != 1 {
		t.Fatalf("child Open() calls = %d, want 1", child.opens)
	}
	if got := len(indexTransaction.pins); got != 1 {
		t.Fatalf("index bucket pins = %d, want 1", got)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if len(indexTransaction.unpins) != 1 || len(dataTransaction.unpins) != 1 {
		t.Fatalf("resource unpins: index=%v data=%v, want one each", indexTransaction.unpins, dataTransaction.unpins)
	}
}

func TestIndexSelectPlanOpenRejectsMissingPlanDependenciesAndKey(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	transaction := &tablePlanTransaction{size: 1}
	indexInfo := newIndexSelectPlanTestInfo(t, transaction, &schema, "student_idx", "student_id")
	child := &productPlanProbe{}
	tests := []struct {
		name string
		plan *IndexSelectPlan
		want string
	}{
		{name: "nil receiver", plan: nil, want: "nil index select plan"},
		{name: "missing child", plan: NewIndexSelectPlan(nil, indexInfo, query.NewIntConstant(1)), want: "child plan is nil"},
		{name: "missing metadata", plan: NewIndexSelectPlan(child, nil, query.NewIntConstant(1)), want: "index metadata is nil"},
		{name: "unset key", plan: NewIndexSelectPlan(child, indexInfo, query.Constant{}), want: "search key is unset"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := test.plan.Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Open() = (%v, %v), want nil scan and error containing %q", scan, err, test.want)
			}
		})
	}
	if child.opens != 0 {
		t.Fatalf("child opened before invalid metadata/key rejection: %d calls", child.opens)
	}
}

func TestIndexSelectPlanOpenPreservesChildOpenFailure(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	indexInfo := newIndexSelectPlanTestInfo(t, &tablePlanTransaction{size: 1}, &schema, "student_idx", "student_id")
	openErr := errors.New("child open failed")
	child := &productPlanProbe{openErr: openErr}
	scan, err := NewIndexSelectPlan(child, indexInfo, query.NewIntConstant(1)).Open()
	if scan != nil || !errors.Is(err, openErr) || !strings.Contains(err.Error(), "child plan") {
		t.Fatalf("Open() = (%v, %v), want child-open context and wrapped cause", scan, err)
	}
}

func TestIndexSelectPlanOpenClosesScanReturnedWithChildFailure(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	indexInfo := newIndexSelectPlanTestInfo(t, &tablePlanTransaction{size: 1}, &schema, "student_idx", "student_id")
	openErr := errors.New("child returned an error")
	closeErr := errors.New("child scan close failed")
	partialScan := &productPlanScanStub{closeErr: closeErr}
	child := &productPlanProbe{scan: partialScan, openErr: openErr}
	scan, err := NewIndexSelectPlan(child, indexInfo, query.NewIntConstant(1)).Open()
	if scan != nil || !errors.Is(err, openErr) || !errors.Is(err, closeErr) {
		t.Fatalf("Open() = (%v, %v), want both open and cleanup causes", scan, err)
	}
	if partialScan.closeCalls != 1 {
		t.Fatalf("partial child scan close calls = %d, want 1", partialScan.closeCalls)
	}
}

func TestIndexSelectPlanOpenRejectsNilOrNonRecordChildScans(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	indexInfo := newIndexSelectPlanTestInfo(t, &tablePlanTransaction{size: 1}, &schema, "student_idx", "student_id")
	nonRecordScan := &productPlanScanStub{}
	var typedNil *query.RecordScan
	tests := []struct {
		name  string
		scan  query.Scan
		want  string
		close *productPlanScanStub
	}{
		{name: "nil", scan: nil, want: "child scan is nil"},
		{name: "non-record", scan: nonRecordScan, want: "is not a query record scan", close: nonRecordScan},
		{name: "typed nil", scan: typedNil, want: "table scan is nil"},
		{name: "empty record adapter", scan: &query.RecordScan{}, want: "table scan is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			child := &productPlanProbe{scan: test.scan}
			scan, err := NewIndexSelectPlan(child, indexInfo, query.NewIntConstant(1)).Open()
			if scan != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Open() = (%v, %v), want error containing %q", scan, err, test.want)
			}
			if test.close != nil && test.close.closeCalls != 1 {
				t.Fatalf("invalid child scan close calls = %d, want 1", test.close.closeCalls)
			}
		})
	}
}

func TestIndexSelectPlanOpenJoinsPositionAndCleanupFailures(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	dataTransaction := &indexSelectPlanOpenTransaction{
		tablePlanTransaction: &tablePlanTransaction{size: 1},
		unpinErr:             errors.New("data scan cleanup failed"),
	}
	dataScan, err := query.NewRecordScan(dataTransaction, "student", record.NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	searchKey := query.NewIntConstant(6)
	bucket, err := hash.BucketNumber(searchKey)
	if err != nil {
		t.Fatalf("BucketNumber() error = %v", err)
	}
	positionErr := errors.New("bucket open failed")
	indexTransaction := &indexSelectPlanOpenTransaction{
		tablePlanTransaction: &tablePlanTransaction{size: 1},
		sizeErrFile:          "student_idx" + itoaForPlanTest(bucket) + ".tbl",
		sizeErr:              positionErr,
	}
	indexInfo := newIndexSelectPlanTestInfo(t, indexTransaction, &schema, "student_idx", "student_id")
	child := &productPlanProbe{scan: dataScan}
	scan, err := NewIndexSelectPlan(child, indexInfo, searchKey).Open()
	if scan != nil || !errors.Is(err, positionErr) || !errors.Is(err, dataTransaction.unpinErr) {
		t.Fatalf("Open() = (%v, %v), want position and table cleanup causes", scan, err)
	}
	if len(dataTransaction.unpins) != 1 {
		t.Fatalf("data scan cleanup unpins = %v, want one unpin", dataTransaction.unpins)
	}
}

type indexSelectPlanOpenTransaction struct {
	*tablePlanTransaction
	sizeErrFile string
	sizeErr     error
	unpinErr    error
}

func (transaction *indexSelectPlanOpenTransaction) Size(fileName string) (int, error) {
	if transaction.sizeErr != nil && fileName == transaction.sizeErrFile {
		return 0, transaction.sizeErr
	}
	return transaction.tablePlanTransaction.Size(fileName)
}

func (transaction *indexSelectPlanOpenTransaction) Unpin(block storage.BlockID) error {
	if err := transaction.tablePlanTransaction.Unpin(block); err != nil {
		return err
	}
	return transaction.unpinErr
}

func newIndexSelectPlanTestInfo(t *testing.T, transaction record.TableScanTransaction, schema *record.Schema, indexName, fieldName string) *metadata.IndexInfo {
	t.Helper()
	info, err := metadata.NewIndexInfo(indexName, fieldName, schema, transaction, metadata.NewStatInfo(1, 1))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	return info
}

func itoaForPlanTest(value int) string {
	return strconv.Itoa(value)
}

var _ record.TableScanTransaction = (*indexSelectPlanOpenTransaction)(nil)
