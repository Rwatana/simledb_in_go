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
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

var _ Plan = contractProbe{}
var _ QueryPlanner = queryPlannerContractProbe{}
var _ UpdatePlanner = updatePlannerContractProbe{}

type queryPlannerContractProbe struct{}

func (queryPlannerContractProbe) CreatePlan(parse.QueryData, record.TableScanTransaction) (Plan, error) {
	return nil, nil
}

type updatePlannerContractProbe struct{}

func (updatePlannerContractProbe) ExecuteInsert(parse.InsertData, record.TableScanTransaction) (int, error) {
	return 0, nil
}
func (updatePlannerContractProbe) ExecuteDelete(parse.DeleteData, record.TableScanTransaction) (int, error) {
	return 0, nil
}
func (updatePlannerContractProbe) ExecuteModify(parse.ModifyData, record.TableScanTransaction) (int, error) {
	return 0, nil
}
func (updatePlannerContractProbe) ExecuteCreateTable(parse.CreateTableData, record.TableScanTransaction) (int, error) {
	return 0, nil
}
func (updatePlannerContractProbe) ExecuteCreateView(parse.CreateViewData, record.TableScanTransaction) (int, error) {
	return 0, nil
}
func (updatePlannerContractProbe) ExecuteCreateIndex(parse.CreateIndexData, record.TableScanTransaction) (int, error) {
	return 0, nil
}

func TestUpdatePlannerContractCoversEachUpdateCommand(t *testing.T) {
	planner := updatePlannerContractProbe{}
	transaction := &tablePlanTransaction{}
	checks := []struct {
		name    string
		execute func() (int, error)
	}{
		{"insert", func() (int, error) { return planner.ExecuteInsert(parse.InsertData{}, transaction) }},
		{"delete", func() (int, error) { return planner.ExecuteDelete(parse.DeleteData{}, transaction) }},
		{"modify", func() (int, error) { return planner.ExecuteModify(parse.ModifyData{}, transaction) }},
		{"create table", func() (int, error) { return planner.ExecuteCreateTable(parse.CreateTableData{}, transaction) }},
		{"create view", func() (int, error) { return planner.ExecuteCreateView(parse.CreateViewData{}, transaction) }},
		{"create index", func() (int, error) { return planner.ExecuteCreateIndex(parse.CreateIndexData{}, transaction) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if affected, err := check.execute(); affected != 0 || err != nil {
				t.Errorf("execution probe = (%d, %v), want (0, nil)", affected, err)
			}
		})
	}
}

func TestBasicUpdatePlannerExecuteDeleteCountsMatchesAndClosesScan(t *testing.T) {
	for _, test := range []struct {
		name      string
		predicate query.Predicate
		wantCount int
		wantFlags []int32
	}{
		{
			name:      "delete matching row",
			predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(10)))),
			wantCount: 1,
			wantFlags: []int32{0, 1},
		},
		{
			name:      "preserve unmatched rows",
			predicate: query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(99)))),
			wantCount: 0,
			wantFlags: []int32{1, 1},
		},
		{
			name:      "delete every row for an empty predicate",
			predicate: query.Predicate{},
			wantCount: 2,
			wantFlags: []int32{0, 0},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := newPlannerTestSchema("id")
			layout := record.NewLayout(schema)
			transaction := newBasicUpdatePlannerTransaction(layout, []int32{1, 1}, []int32{10, 20})
			planner := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout})

			count, err := planner.ExecuteDelete(parse.NewDeleteData("student", test.predicate), transaction)
			if err != nil {
				t.Fatalf("ExecuteDelete() error = %v", err)
			}
			if count != test.wantCount {
				t.Errorf("ExecuteDelete() affected = %d, want %d", count, test.wantCount)
			}
			if !reflect.DeepEqual(transaction.flags, test.wantFlags) {
				t.Errorf("record flags = %v, want %v", transaction.flags, test.wantFlags)
			}
			if len(transaction.unpins) != 1 {
				t.Errorf("unpin calls = %v, want one closed scan block", transaction.unpins)
			}
		})
	}
}

func TestBasicUpdatePlannerExecuteDeleteValidatesDependenciesAndMetadata(t *testing.T) {
	data := parse.NewDeleteData("student", query.Predicate{})
	transaction := &tablePlanTransaction{size: 1}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteDelete(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteDelete() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteDelete(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteDelete() = (%d, %v), want an error", count, err)
	}
	metadataErr := errors.New("layout unavailable")
	metadata := &basicUpdatePlannerMetadataProbe{layoutErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteDelete(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteDelete() with metadata error = (%d, %v), want wrapped layout error", count, err)
	}
	if !reflect.DeepEqual(metadata.layoutRequests, []string{"student"}) {
		t.Errorf("layout requests = %v, want [student]", metadata.layoutRequests)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteDelete(data, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteDelete() with nil layout = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: record.NewLayout(newPlannerTestSchema("id"))}).ExecuteDelete(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteDelete() with nil transaction = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: record.NewLayout(newPlannerTestSchema("id"))}).ExecuteDelete(parse.DeleteData{}, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteDelete() with empty table name = (%d, %v), want an error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteDeleteJoinsOperationAndCloseErrors(t *testing.T) {
	readErr := errors.New("record flag read failed")
	closeErr := errors.New("scan close failed")
	layout := record.NewLayout(newPlannerTestSchema("id"))
	transaction := newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	transaction.readErrOffset = 0
	transaction.readErr = readErr
	transaction.unpinErr = closeErr
	count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteDelete(parse.NewDeleteData("student", query.Predicate{}), transaction)
	if count != 0 || !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteDelete() with iteration/close errors = (%d, %v), want both causes", count, err)
	}
	if len(transaction.unpins) != 1 {
		t.Errorf("unpin calls = %v, want one close attempt after iteration failure", transaction.unpins)
	}

	deleteErr := errors.New("record delete failed")
	transaction = newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	transaction.writeErrOffset = 0
	transaction.writeErr = deleteErr
	transaction.unpinErr = closeErr
	count, err = NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteDelete(parse.NewDeleteData("student", query.Predicate{}), transaction)
	if count != 0 || !errors.Is(err, deleteErr) || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteDelete() with delete/close errors = (%d, %v), want both causes", count, err)
	}
}

func TestBasicUpdatePlannerExecuteDeleteReportsScanOpenAndCloseErrors(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	openErr := errors.New("table size unavailable")
	transaction := &basicUpdatePlannerTransaction{tablePlanTransaction: tablePlanTransaction{sizeErr: openErr}, slotSize: layout.SlotSize()}
	count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteDelete(parse.NewDeleteData("student", query.Predicate{}), transaction)
	if count != 0 || !errors.Is(err, openErr) {
		t.Errorf("ExecuteDelete() with scan-open error = (%d, %v), want wrapped size error", count, err)
	}

	transaction = newBasicUpdatePlannerTransaction(layout, []int32{0}, []int32{10})
	closeErr := errors.New("scan close failed")
	transaction.unpinErr = closeErr
	count, err = NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteDelete(parse.NewDeleteData("student", query.Predicate{}), transaction)
	if count != 0 || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteDelete() with close error = (%d, %v), want wrapped close error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteModifyUpdatesOnlyMatchingRows(t *testing.T) {
	for _, test := range []struct {
		name       string
		predicate  query.Predicate
		expression query.Expression
		wantCount  int
		wantIDs    []int32
	}{
		{
			name:       "modify matching row",
			predicate:  query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(10)))),
			expression: query.NewConstantExpression(query.NewIntConstant(99)),
			wantCount:  1,
			wantIDs:    []int32{99, 20},
		},
		{
			name:       "evaluate a field expression against the matching row",
			predicate:  query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(10)))),
			expression: query.NewFieldExpression("id"),
			wantCount:  1,
			wantIDs:    []int32{10, 20},
		},
		{
			name:       "leave unmatched rows unchanged",
			predicate:  query.NewPredicateWithTerm(query.NewTerm(query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(99)))),
			expression: query.NewConstantExpression(query.NewIntConstant(99)),
			wantCount:  0,
			wantIDs:    []int32{10, 20},
		},
		{
			name:       "modify every row for an empty predicate",
			predicate:  query.Predicate{},
			expression: query.NewConstantExpression(query.NewIntConstant(99)),
			wantCount:  2,
			wantIDs:    []int32{99, 99},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout := record.NewLayout(newPlannerTestSchema("id"))
			transaction := newBasicUpdatePlannerTransaction(layout, []int32{1, 1}, []int32{10, 20})
			planner := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout})
			data := parse.NewModifyData("student", "id", test.expression, test.predicate)

			count, err := planner.ExecuteModify(data, transaction)
			if err != nil {
				t.Fatalf("ExecuteModify() error = %v", err)
			}
			if count != test.wantCount {
				t.Errorf("ExecuteModify() affected = %d, want %d", count, test.wantCount)
			}
			if !reflect.DeepEqual(transaction.values, test.wantIDs) {
				t.Errorf("record values = %v, want %v", transaction.values, test.wantIDs)
			}
			if len(transaction.unpins) != 1 {
				t.Errorf("unpin calls = %v, want one closed scan block", transaction.unpins)
			}
		})
	}
}

func TestBasicUpdatePlannerExecuteModifyValidatesDependenciesAndMetadata(t *testing.T) {
	data := parse.NewModifyData("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{})
	transaction := &tablePlanTransaction{size: 1}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteModify(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteModify() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteModify(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteModify() = (%d, %v), want an error", count, err)
	}
	metadataErr := errors.New("layout unavailable")
	metadata := &basicUpdatePlannerMetadataProbe{layoutErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteModify(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteModify() with metadata error = (%d, %v), want wrapped layout error", count, err)
	}
	if !reflect.DeepEqual(metadata.layoutRequests, []string{"student"}) {
		t.Errorf("layout requests = %v, want [student]", metadata.layoutRequests)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteModify(data, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteModify() with nil layout = (%d, %v), want an error", count, err)
	}
	layout := record.NewLayout(newPlannerTestSchema("id"))
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteModify() with nil transaction = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(parse.ModifyData{}, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteModify() with empty table name = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(parse.NewModifyData("student", "", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{}), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteModify() with empty target field = (%d, %v), want an error", count, err)
	}
	transactionWithRows := newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	unknownTarget := parse.NewModifyData("student", "missing", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{})
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(unknownTarget, transactionWithRows); count != 0 || err == nil {
		t.Errorf("ExecuteModify() with unknown target field = (%d, %v), want an error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteModifyJoinsEvaluationWriteIterationAndCloseErrors(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	closeErr := errors.New("scan close failed")
	readErr := errors.New("record flag read failed")
	transaction := newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	transaction.readErrOffset = 0
	transaction.readErr = readErr
	transaction.unpinErr = closeErr
	data := parse.NewModifyData("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{})
	count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, transaction)
	if count != 0 || !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteModify() with iteration/close errors = (%d, %v), want both causes", count, err)
	}

	transaction = newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	transaction.unpinErr = closeErr
	data = parse.NewModifyData("student", "id", query.NewFieldExpression("missing"), query.Predicate{})
	count, err = NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, transaction)
	if count != 0 || err == nil || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteModify() with expression/close errors = (%d, %v), want evaluation and close errors", count, err)
	}

	valueOffset, ok := layout.Offset("id")
	if !ok {
		t.Fatal("layout does not contain id field")
	}
	writeErr := errors.New("record update failed")
	transaction = newBasicUpdatePlannerTransaction(layout, []int32{1}, []int32{10})
	transaction.writeErrOffset = valueOffset
	transaction.writeErr = writeErr
	transaction.unpinErr = closeErr
	data = parse.NewModifyData("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{})
	count, err = NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, transaction)
	if count != 0 || !errors.Is(err, writeErr) || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteModify() with write/close errors = (%d, %v), want both causes", count, err)
	}
}

func TestBasicUpdatePlannerExecuteModifyReportsScanOpenAndCloseErrors(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	openErr := errors.New("table size unavailable")
	transaction := &basicUpdatePlannerTransaction{tablePlanTransaction: tablePlanTransaction{sizeErr: openErr}, slotSize: layout.SlotSize()}
	data := parse.NewModifyData("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), query.Predicate{})
	count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, transaction)
	if count != 0 || !errors.Is(err, openErr) {
		t.Errorf("ExecuteModify() with scan-open error = (%d, %v), want wrapped size error", count, err)
	}

	transaction = newBasicUpdatePlannerTransaction(layout, []int32{0}, []int32{10})
	closeErr := errors.New("scan close failed")
	transaction.unpinErr = closeErr
	count, err = NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteModify(data, transaction)
	if count != 0 || !errors.Is(err, closeErr) {
		t.Errorf("ExecuteModify() with close error = (%d, %v), want wrapped close error", count, err)
	}
}

func TestModifyRowsFromScanHandlesReadOnlyAndNilScans(t *testing.T) {
	closeErr := errors.New("read-only scan close failed")
	scan := &productPlanScanStub{closeErr: closeErr}
	if count, err := modifyRowsFromScan("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), scan); count != 0 || err == nil || !errors.Is(err, closeErr) {
		t.Errorf("modifyRowsFromScan() with read-only scan = (%d, %v), want unsupported and close errors", count, err)
	}
	if scan.closeCalls != 1 {
		t.Errorf("read-only scan close calls = %d, want 1", scan.closeCalls)
	}
	if count, err := modifyRowsFromScan("student", "id", query.NewConstantExpression(query.NewIntConstant(99)), nil); count != 0 || err == nil {
		t.Errorf("modifyRowsFromScan() with nil scan = (%d, %v), want an error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteInsertCreatesAndPopulatesOneRecord(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	transaction := newBasicUpdatePlannerTransaction(layout, []int32{1, 0}, []int32{10, 0})
	planner := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout})
	data := parse.NewInsertData("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)})

	count, err := planner.ExecuteInsert(data, transaction)
	if err != nil {
		t.Fatalf("ExecuteInsert() error = %v", err)
	}
	if count != 1 {
		t.Errorf("ExecuteInsert() affected = %d, want 1", count)
	}
	if !reflect.DeepEqual(transaction.flags, []int32{1, 1}) {
		t.Errorf("record flags = %v, want [1 1]", transaction.flags)
	}
	if !reflect.DeepEqual(transaction.values, []int32{10, 42}) {
		t.Errorf("record values = %v, want [10 42]", transaction.values)
	}
	if len(transaction.unpins) != 1 {
		t.Errorf("unpin calls = %v, want one closed scan block", transaction.unpins)
	}
}

func TestBasicUpdatePlannerExecuteInsertValidatesDependenciesAndData(t *testing.T) {
	data := parse.NewInsertData("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)})
	transaction := &tablePlanTransaction{size: 1}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteInsert(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteInsert() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteInsert(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteInsert() = (%d, %v), want an error", count, err)
	}
	layout := record.NewLayout(newPlannerTestSchema("id"))
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with nil transaction = (%d, %v), want an error", count, err)
	}
	metadataErr := errors.New("layout unavailable")
	metadata := &basicUpdatePlannerMetadataProbe{layoutErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteInsert(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteInsert() with metadata error = (%d, %v), want wrapped layout error", count, err)
	}
	if !reflect.DeepEqual(metadata.layoutRequests, []string{"student"}) {
		t.Errorf("layout requests = %v, want [student]", metadata.layoutRequests)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteInsert(data, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with nil layout = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(parse.InsertData{}, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with empty table name = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(parse.NewInsertData("student", nil, nil), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with no fields = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(parse.NewInsertData("student", []string{"id"}, nil), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with mismatched field/value lists = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(parse.NewInsertData("student", []string{""}, []query.Constant{query.NewIntConstant(42)}), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteInsert() with empty field name = (%d, %v), want an error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteInsertReportsScanOpenErrors(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	openErr := errors.New("table size unavailable")
	transaction := &basicUpdatePlannerTransaction{tablePlanTransaction: tablePlanTransaction{sizeErr: openErr}, slotSize: layout.SlotSize()}
	data := parse.NewInsertData("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)})
	count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{layout: layout}).ExecuteInsert(data, transaction)
	if count != 0 || !errors.Is(err, openErr) {
		t.Errorf("ExecuteInsert() with scan-open error = (%d, %v), want wrapped size error", count, err)
	}
}

func TestInsertRowIntoScanClosesAfterInsertAndValueErrors(t *testing.T) {
	layout := record.NewLayout(newPlannerTestSchema("id"))
	insertErr := errors.New("record insert failed")
	transaction := newBasicUpdatePlannerTransaction(layout, []int32{0}, []int32{0})
	transaction.writeErrOffset = 0
	transaction.writeErr = insertErr
	scan, err := query.NewRecordScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	count, err := insertRowIntoScan("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)}, scan)
	if count != 0 || !errors.Is(err, insertErr) {
		t.Errorf("insertRowIntoScan() with insert error = (%d, %v), want wrapped insert error", count, err)
	}
	if len(transaction.unpins) != 1 {
		t.Errorf("unpin calls = %v, want one close after insert error", transaction.unpins)
	}

	valueOffset, ok := layout.Offset("id")
	if !ok {
		t.Fatal("layout does not contain id field")
	}
	transaction = newBasicUpdatePlannerTransaction(layout, []int32{0}, []int32{0})
	transaction.writeErrOffset = valueOffset
	transaction.writeErr = errors.New("record value write failed")
	scan, err = query.NewRecordScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewRecordScan() for write error = %v", err)
	}
	closeErr := errors.New("scan close failed")
	transaction.unpinErr = closeErr
	count, err = insertRowIntoScan("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)}, scan)
	if count != 0 || !errors.Is(err, transaction.writeErr) || !errors.Is(err, closeErr) {
		t.Errorf("insertRowIntoScan() with value/close errors = (%d, %v), want both causes", count, err)
	}
}

func TestInsertRowIntoScanHandlesReadOnlyAndCloseErrors(t *testing.T) {
	closeErr := errors.New("read-only scan close failed")
	scan := &productPlanScanStub{closeErr: closeErr}
	if count, err := insertRowIntoScan("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)}, scan); count != 0 || err == nil || !errors.Is(err, closeErr) {
		t.Errorf("insertRowIntoScan() with read-only scan = (%d, %v), want unsupported and close errors", count, err)
	}
	if scan.closeCalls != 1 {
		t.Errorf("read-only scan close calls = %d, want 1", scan.closeCalls)
	}
	if count, err := insertRowIntoScan("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)}, nil); count != 0 || err == nil {
		t.Errorf("insertRowIntoScan() with nil scan = (%d, %v), want an error", count, err)
	}

	layout := record.NewLayout(newPlannerTestSchema("id"))
	transaction := newBasicUpdatePlannerTransaction(layout, []int32{0}, []int32{0})
	transaction.unpinErr = closeErr
	recordScan, err := query.NewRecordScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewRecordScan() for close error = %v", err)
	}
	if count, err := insertRowIntoScan("student", []string{"id"}, []query.Constant{query.NewIntConstant(42)}, recordScan); count != 1 || !errors.Is(err, closeErr) {
		t.Errorf("insertRowIntoScan() with close error = (%d, %v), want inserted count and close error", count, err)
	}
}

func TestBasicUpdatePlannerExecuteCreateTableForwardsSchemaAndTransaction(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 32)
	data := parse.NewCreateTableData("student", schema)
	transaction := &tablePlanTransaction{}
	metadata := &basicUpdatePlannerMetadataProbe{}

	count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateTable(data, transaction)
	if err != nil {
		t.Fatalf("ExecuteCreateTable() error = %v", err)
	}
	if count != 0 {
		t.Errorf("ExecuteCreateTable() affected = %d, want 0", count)
	}
	if !reflect.DeepEqual(metadata.createTableRequests, []string{"student"}) {
		t.Errorf("CreateTable requests = %v, want [student]", metadata.createTableRequests)
	}
	if len(metadata.createTableTransactions) != 1 || metadata.createTableTransactions[0] != transaction {
		t.Errorf("CreateTable transactions = %v, want the original transaction", metadata.createTableTransactions)
	}
	if len(metadata.createTableSchemas) != 1 || !reflect.DeepEqual(metadata.createTableSchemas[0].Fields(), []string{"id", "name"}) {
		t.Fatalf("CreateTable schemas = %v, want fields [id name]", metadata.createTableSchemas)
	}
	createdSchema := metadata.createTableSchemas[0]
	if definition, ok := createdSchema.Field("id"); !ok || definition.Type != record.IntegerField {
		t.Errorf("created id field = (%+v, %v), want integer", definition, ok)
	}
	if definition, ok := createdSchema.Field("name"); !ok || definition.Type != record.VarcharField || definition.Length != 32 {
		t.Errorf("created name field = (%+v, %v), want varchar(32)", definition, ok)
	}
}

func TestBasicUpdatePlannerExecuteCreateTableValidatesInputsAndMetadataErrors(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("id")
	data := parse.NewCreateTableData("student", schema)
	transaction := &tablePlanTransaction{}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteCreateTable(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteCreateTable() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteCreateTable(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteCreateTable() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteCreateTable(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteCreateTable() with nil transaction = (%d, %v), want an error", count, err)
	}
	metadataErr := errors.New("create table failed")
	metadata := &basicUpdatePlannerMetadataProbe{createTableErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateTable(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteCreateTable() with metadata error = (%d, %v), want wrapped metadata error", count, err)
	}
	if len(metadata.createTableRequests) != 1 {
		t.Errorf("CreateTable requests after metadata failure = %v, want one attempted create", metadata.createTableRequests)
	}
	metadata = &basicUpdatePlannerMetadataProbe{}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateTable(parse.CreateTableData{}, transaction); count != 0 || err == nil {
		t.Errorf("ExecuteCreateTable() with empty table name = (%d, %v), want an error", count, err)
	}
	if len(metadata.createTableRequests) != 0 {
		t.Errorf("CreateTable requests for invalid data = %v, want none", metadata.createTableRequests)
	}
}

func TestBasicUpdatePlannerExecuteCreateViewForwardsDefinitionAndTransaction(t *testing.T) {
	definition := parse.NewQueryData(
		[]string{"id", "name"},
		[]string{"student"},
		query.NewPredicateWithTerm(query.NewTerm(
			query.NewFieldExpression("active"),
			query.NewConstantExpression(query.NewIntConstant(1)),
		)),
	)
	data := parse.NewCreateViewData("active_students", definition)
	transaction := &tablePlanTransaction{}
	metadata := &basicUpdatePlannerMetadataProbe{}

	count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateView(data, transaction)
	if err != nil {
		t.Fatalf("ExecuteCreateView() error = %v", err)
	}
	if count != 0 {
		t.Errorf("ExecuteCreateView() affected = %d, want 0", count)
	}
	if !reflect.DeepEqual(metadata.createViewRequests, []string{"active_students"}) {
		t.Errorf("CreateView requests = %v, want [active_students]", metadata.createViewRequests)
	}
	if !reflect.DeepEqual(metadata.createViewDefinitions, []string{"select id, name from student where active=1"}) {
		t.Errorf("CreateView definitions = %v, want rendered SELECT", metadata.createViewDefinitions)
	}
	if len(metadata.createViewTransactions) != 1 || metadata.createViewTransactions[0] != transaction {
		t.Errorf("CreateView transactions = %v, want original transaction", metadata.createViewTransactions)
	}
}

func TestBasicUpdatePlannerExecuteCreateViewValidatesInputsAndMetadataErrors(t *testing.T) {
	queryData := parse.NewQueryData([]string{"id"}, []string{"student"}, query.Predicate{})
	data := parse.NewCreateViewData("active_students", queryData)
	transaction := &tablePlanTransaction{}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteCreateView(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteCreateView() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteCreateView(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteCreateView() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteCreateView(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteCreateView() with nil transaction = (%d, %v), want an error", count, err)
	}
	metadata := &basicUpdatePlannerMetadataProbe{}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateView(parse.NewCreateViewData("", queryData), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteCreateView() with empty view name = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateView(parse.NewCreateViewData("active_students", parse.QueryData{}), transaction); count != 0 || err == nil {
		t.Errorf("ExecuteCreateView() with unset definition = (%d, %v), want an error", count, err)
	}
	if len(metadata.createViewRequests) != 0 {
		t.Errorf("CreateView requests for invalid inputs = %v, want none", metadata.createViewRequests)
	}

	metadataErr := errors.New("create view failed")
	metadata = &basicUpdatePlannerMetadataProbe{createViewErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateView(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteCreateView() with metadata error = (%d, %v), want wrapped metadata error", count, err)
	}
	if len(metadata.createViewRequests) != 1 {
		t.Errorf("CreateView requests after metadata failure = %v, want one attempted create", metadata.createViewRequests)
	}
}

func TestBasicUpdatePlannerExecuteCreateIndexForwardsNamesAndTransaction(t *testing.T) {
	data := parse.NewCreateIndexData("student_id_idx", "student", "id")
	transaction := &tablePlanTransaction{}
	metadata := &basicUpdatePlannerMetadataProbe{}

	count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateIndex(data, transaction)
	if err != nil {
		t.Fatalf("ExecuteCreateIndex() error = %v", err)
	}
	if count != 0 {
		t.Errorf("ExecuteCreateIndex() affected = %d, want 0", count)
	}
	if !reflect.DeepEqual(metadata.createIndexNames, []string{"student_id_idx"}) {
		t.Errorf("CreateIndex names = %v, want [student_id_idx]", metadata.createIndexNames)
	}
	if !reflect.DeepEqual(metadata.createIndexTables, []string{"student"}) {
		t.Errorf("CreateIndex tables = %v, want [student]", metadata.createIndexTables)
	}
	if !reflect.DeepEqual(metadata.createIndexFields, []string{"id"}) {
		t.Errorf("CreateIndex fields = %v, want [id]", metadata.createIndexFields)
	}
	if len(metadata.createIndexTransactions) != 1 || metadata.createIndexTransactions[0] != transaction {
		t.Errorf("CreateIndex transactions = %v, want original transaction", metadata.createIndexTransactions)
	}
}

func TestBasicUpdatePlannerExecuteCreateIndexValidatesInputsAndMetadataErrors(t *testing.T) {
	data := parse.NewCreateIndexData("student_id_idx", "student", "id")
	transaction := &tablePlanTransaction{}
	var nilPlanner *BasicUpdatePlanner
	if count, err := nilPlanner.ExecuteCreateIndex(data, transaction); count != 0 || err == nil {
		t.Errorf("nil planner ExecuteCreateIndex() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(nil).ExecuteCreateIndex(data, transaction); count != 0 || err == nil {
		t.Errorf("planner without metadata ExecuteCreateIndex() = (%d, %v), want an error", count, err)
	}
	if count, err := NewBasicUpdatePlanner(&basicUpdatePlannerMetadataProbe{}).ExecuteCreateIndex(data, nil); count != 0 || err == nil {
		t.Errorf("ExecuteCreateIndex() with nil transaction = (%d, %v), want an error", count, err)
	}
	metadata := &basicUpdatePlannerMetadataProbe{}
	for _, invalid := range []struct {
		name string
		data parse.CreateIndexData
	}{
		{name: "empty index name", data: parse.NewCreateIndexData("", "student", "id")},
		{name: "empty table name", data: parse.NewCreateIndexData("student_id_idx", "", "id")},
		{name: "empty field name", data: parse.NewCreateIndexData("student_id_idx", "student", "")},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateIndex(invalid.data, transaction); count != 0 || err == nil {
				t.Errorf("ExecuteCreateIndex() with invalid names = (%d, %v), want an error", count, err)
			}
		})
	}
	if len(metadata.createIndexNames) != 0 {
		t.Errorf("CreateIndex calls for invalid names = %v, want none", metadata.createIndexNames)
	}

	metadataErr := errors.New("create index failed")
	metadata = &basicUpdatePlannerMetadataProbe{createIndexErr: metadataErr}
	if count, err := NewBasicUpdatePlanner(metadata).ExecuteCreateIndex(data, transaction); count != 0 || !errors.Is(err, metadataErr) {
		t.Errorf("ExecuteCreateIndex() with metadata error = (%d, %v), want wrapped metadata error", count, err)
	}
	if len(metadata.createIndexNames) != 1 {
		t.Errorf("CreateIndex calls after metadata failure = %v, want one attempted create", metadata.createIndexNames)
	}
}

func TestDeleteRowsFromScanHandlesReadOnlyAndNilScans(t *testing.T) {
	closeErr := errors.New("read-only scan close failed")
	scan := &productPlanScanStub{closeErr: closeErr}
	if count, err := deleteRowsFromScan("student", scan); count != 0 || err == nil || !errors.Is(err, closeErr) {
		t.Errorf("deleteRowsFromScan() with read-only scan = (%d, %v), want unsupported and close errors", count, err)
	}
	if scan.closeCalls != 1 {
		t.Errorf("read-only scan close calls = %d, want 1", scan.closeCalls)
	}
	if count, err := deleteRowsFromScan("student", nil); count != 0 || err == nil {
		t.Errorf("deleteRowsFromScan() with nil scan = (%d, %v), want an error", count, err)
	}
	primary := errors.New("primary operation failed")
	if err := closeUpdatePlannerScan(nil, primary); !errors.Is(err, primary) {
		t.Errorf("closing nil scan with primary error = %v, want primary error", err)
	}
}

func TestRequireUpdateScanRejectsReadOnlyScan(t *testing.T) {
	closeErr := errors.New("scan close failed")
	scan := &productPlanScanStub{closeErr: closeErr}
	if update, err := requireUpdateScan(scan); update != nil || err == nil {
		t.Errorf("requireUpdateScan() = (%v, %v), want a read-only scan error", update, err)
	}
	if err := closeUpdatePlannerScan(scan, errors.New("primary")); !errors.Is(err, closeErr) {
		t.Errorf("closeUpdatePlannerScan() error = %v, want close error", err)
	}
}

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

func TestProjectPlanBlocksAccessedDelegatesAndHandlesNilPlan(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("id")
	project, err := NewProjectPlan(&productPlanProbe{schema: &schema, blocks: 2}, nil)
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	if got := project.BlocksAccessed(); got != 2 {
		t.Errorf("BlocksAccessed() = %d, want child estimate 2", got)
	}

	var nilPlan *ProjectPlan
	if got := nilPlan.BlocksAccessed(); got != 0 {
		t.Errorf("nil ProjectPlan.BlocksAccessed() = %d, want 0", got)
	}
	if got := (&ProjectPlan{}).BlocksAccessed(); got != 0 {
		t.Errorf("ProjectPlan without child BlocksAccessed() = %d, want 0", got)
	}
}

func TestProjectPlanRecordsOutputDelegatesAndHandlesNilPlan(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("id")
	project, err := NewProjectPlan(&productPlanProbe{schema: &schema, records: 3}, nil)
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	if got := project.RecordsOutput(); got != 3 {
		t.Errorf("RecordsOutput() = %d, want child estimate 3", got)
	}

	var nilPlan *ProjectPlan
	if got := nilPlan.RecordsOutput(); got != 0 {
		t.Errorf("nil ProjectPlan.RecordsOutput() = %d, want 0", got)
	}
	if got := (&ProjectPlan{}).RecordsOutput(); got != 0 {
		t.Errorf("ProjectPlan without child RecordsOutput() = %d, want 0", got)
	}
}

func TestProjectPlanDistinctValuesDelegatesAndHandlesNilPlan(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	child := &productPlanProbe{
		schema:   &schema,
		distinct: map[string]int{"student_id": 7},
	}
	project, err := NewProjectPlan(child, []string{"student_id"})
	if err != nil {
		t.Fatalf("NewProjectPlan() error = %v", err)
	}
	if got := project.DistinctValues("student_id"); got != 7 {
		t.Errorf("DistinctValues(student_id) = %d, want child estimate 7", got)
	}

	var nilPlan *ProjectPlan
	if got := nilPlan.DistinctValues("student_id"); got != 0 {
		t.Errorf("nil ProjectPlan.DistinctValues() = %d, want 0", got)
	}
	if got := (&ProjectPlan{}).DistinctValues("student_id"); got != 0 {
		t.Errorf("ProjectPlan without child DistinctValues() = %d, want 0", got)
	}
}

func TestBasicQueryPlannerComposesPlansForPhysicalTables(t *testing.T) {
	leftSchema := newPlannerTestSchema("left_id")
	rightSchema := newPlannerTestSchema("right_name")
	metadata := &basicPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"left":  record.NewLayout(leftSchema),
			"right": record.NewLayout(rightSchema),
		},
		statistics: map[string]*metadata.StatInfo{
			"left":  metadata.NewStatInfo(2, 5),
			"right": metadata.NewStatInfo(3, 7),
		},
	}
	predicate := query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("left_id"),
		query.NewConstantExpression(query.NewIntConstant(7)),
	))
	planner := NewBasicQueryPlanner(metadata)
	got, err := planner.CreatePlan(parse.NewQueryData(
		[]string{"right_name", "left_id"}, []string{"left", "right"}, predicate,
	), &tablePlanTransaction{})
	if err != nil {
		t.Fatalf("CreatePlan() error = %v", err)
	}
	project, ok := got.(*ProjectPlan)
	if !ok {
		t.Fatalf("CreatePlan() type = %T, want *ProjectPlan", got)
	}
	if fields, want := project.Schema().Fields(), []string{"right_name", "left_id"}; !reflect.DeepEqual(fields, want) {
		t.Errorf("projected fields = %v, want %v", fields, want)
	}
	selectPlan, ok := project.child.(*SelectPlan)
	if !ok {
		t.Fatalf("ProjectPlan child type = %T, want *SelectPlan", project.child)
	}
	if got := selectPlan.predicate.String(); got != predicate.String() {
		t.Errorf("selection predicate = %q, want %q", got, predicate.String())
	}
	product, ok := selectPlan.child.(*ProductPlan)
	if !ok {
		t.Fatalf("SelectPlan child type = %T, want *ProductPlan", selectPlan.child)
	}
	left, leftOK := product.left.(*TablePlan)
	right, rightOK := product.right.(*TablePlan)
	if !leftOK || !rightOK {
		t.Fatalf("ProductPlan children = (%T, %T), want two *TablePlan values", product.left, product.right)
	}
	if left.tableName != "left" || right.tableName != "right" {
		t.Errorf("product table order = (%q, %q), want (left, right)", left.tableName, right.tableName)
	}
	if got, want := metadata.layoutRequests, []string{"left", "right"}; !reflect.DeepEqual(got, want) {
		t.Errorf("layout requests = %v, want %v", got, want)
	}
	if got, want := metadata.statisticsRequests, []string{"left", "right"}; !reflect.DeepEqual(got, want) {
		t.Errorf("statistics requests = %v, want %v", got, want)
	}
}

func TestBasicQueryPlannerRejectsInvalidInputsAndMetadataFailures(t *testing.T) {
	var nilPlanner *BasicQueryPlanner
	validQuery := parse.NewQueryData([]string{"id"}, []string{"student"}, query.Predicate{})
	if plan, err := nilPlanner.CreatePlan(validQuery, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("nil planner CreatePlan() = (%v, %v), want an error", plan, err)
	}
	if plan, err := NewBasicQueryPlanner(nil).CreatePlan(validQuery, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("planner without metadata CreatePlan() = (%v, %v), want an error", plan, err)
	}
	metadata := &basicPlannerMetadataProbe{}
	planner := NewBasicQueryPlanner(metadata)
	if plan, err := planner.CreatePlan(validQuery, nil); plan != nil || err == nil {
		t.Errorf("CreatePlan() with nil transaction = (%v, %v), want an error", plan, err)
	}
	if plan, err := planner.CreatePlan(parse.QueryData{}, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("CreatePlan() without tables = (%v, %v), want an error", plan, err)
	}

	var schema record.Schema
	schema.AddIntegerField("id")
	layout := record.NewLayout(&schema)
	layoutErr := errors.New("layout unavailable")
	metadata.layoutErr = layoutErr
	data := parse.NewQueryData([]string{"id"}, []string{"student"}, query.Predicate{})
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || !errors.Is(err, layoutErr) {
		t.Errorf("CreatePlan() with layout error = (%v, %v), want wrapped layout error", plan, err)
	}
	metadata.layoutErr = nil
	metadata.layouts = map[string]*record.Layout{"student": nil}
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("CreatePlan() with nil layout = (%v, %v), want an error", plan, err)
	}
	metadata.layouts["student"] = layout
	statisticsErr := errors.New("statistics unavailable")
	metadata.statisticsErr = statisticsErr
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || !errors.Is(err, statisticsErr) {
		t.Errorf("CreatePlan() with statistics error = (%v, %v), want wrapped error", plan, err)
	}
	metadata.statisticsErr = nil
	if plan, err := planner.CreatePlan(parse.NewQueryData([]string{"missing"}, []string{"student"}, query.Predicate{}), &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("CreatePlan() with unknown projection = (%v, %v), want an error", plan, err)
	}
}

func TestBasicQueryPlannerExpandsViewsRecursively(t *testing.T) {
	schema := newPlannerTestSchema("student_id")
	courseSchema := newPlannerTestSchema("course_code")
	metadata := &basicPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"student": record.NewLayout(schema),
			"course":  record.NewLayout(courseSchema),
		},
		statistics: map[string]*metadata.StatInfo{
			"student": metadata.NewStatInfo(2, 8),
			"course":  metadata.NewStatInfo(1, 4),
		},
		viewDefinitions: map[string]string{"active_students": "select student_id from student"},
	}
	planner := NewBasicQueryPlanner(metadata)
	got, err := planner.CreatePlan(
		parse.NewQueryData([]string{"student_id", "course_code"}, []string{"active_students", "course"}, query.Predicate{}),
		&tablePlanTransaction{},
	)
	if err != nil {
		t.Fatalf("CreatePlan() for view error = %v", err)
	}
	outerProject, ok := got.(*ProjectPlan)
	if !ok {
		t.Fatalf("CreatePlan() type = %T, want *ProjectPlan", got)
	}
	outerSelect, ok := outerProject.child.(*SelectPlan)
	if !ok {
		t.Fatalf("outer ProjectPlan child type = %T, want *SelectPlan", outerProject.child)
	}
	product, ok := outerSelect.child.(*ProductPlan)
	if !ok {
		t.Fatalf("outer SelectPlan child type = %T, want *ProductPlan", outerSelect.child)
	}
	viewProject, ok := product.left.(*ProjectPlan)
	if !ok {
		t.Fatalf("product left child type = %T, want *ProjectPlan for view", product.left)
	}
	viewSelect, ok := viewProject.child.(*SelectPlan)
	if !ok {
		t.Fatalf("view projection child type = %T, want *SelectPlan", viewProject.child)
	}
	table, ok := viewSelect.child.(*TablePlan)
	if !ok || table.tableName != "student" {
		t.Fatalf("view base plan = %#v, want TablePlan for student", viewSelect.child)
	}
	right, ok := product.right.(*TablePlan)
	if !ok || right.tableName != "course" {
		t.Fatalf("product right child = %#v, want TablePlan for course", product.right)
	}
	if got, want := metadata.viewRequests, []string{"active_students", "student", "course"}; !reflect.DeepEqual(got, want) {
		t.Errorf("view lookups = %v, want %v", got, want)
	}
}

func TestBasicQueryPlannerCombinesMultipleViews(t *testing.T) {
	studentSchema := newPlannerTestSchema("student_id")
	courseSchema := newPlannerTestSchema("course_id")
	metadata := &basicPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"student": record.NewLayout(studentSchema),
			"course":  record.NewLayout(courseSchema),
		},
		statistics: map[string]*metadata.StatInfo{
			"student": metadata.NewStatInfo(1, 1),
			"course":  metadata.NewStatInfo(1, 1),
		},
		viewDefinitions: map[string]string{
			"students": "select student_id from student",
			"courses":  "select course_id from course",
		},
	}
	got, err := NewBasicQueryPlanner(metadata).CreatePlan(
		parse.NewQueryData([]string{"student_id", "course_id"}, []string{"students", "courses"}, query.Predicate{}),
		&tablePlanTransaction{},
	)
	if err != nil {
		t.Fatalf("CreatePlan() for two views error = %v", err)
	}
	outerSelect := got.(*ProjectPlan).child.(*SelectPlan)
	product, ok := outerSelect.child.(*ProductPlan)
	if !ok {
		t.Fatalf("selection child type = %T, want *ProductPlan", outerSelect.child)
	}
	if _, ok := product.left.(*ProjectPlan); !ok {
		t.Errorf("product left type = %T, want *ProjectPlan", product.left)
	}
	if _, ok := product.right.(*ProjectPlan); !ok {
		t.Errorf("product right type = %T, want *ProjectPlan", product.right)
	}
}

func TestBasicQueryPlannerReportsViewErrorsAndCycles(t *testing.T) {
	metadata := &basicPlannerMetadataProbe{viewErr: errors.New("view metadata unavailable")}
	planner := NewBasicQueryPlanner(metadata)
	data := parse.NewQueryData([]string{"id"}, []string{"active_students"}, query.Predicate{})
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || !errors.Is(err, metadata.viewErr) || !strings.Contains(err.Error(), "active_students") {
		t.Errorf("CreatePlan() with view lookup error = (%v, %v), want contextual wrapped error", plan, err)
	}

	metadata.viewErr = nil
	metadata.viewDefinitions = map[string]string{"broken_view": "select from student"}
	data = parse.NewQueryData([]string{"id"}, []string{"broken_view"}, query.Predicate{})
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil || !strings.Contains(err.Error(), "broken_view") {
		t.Errorf("CreatePlan() with invalid view query = (%v, %v), want contextual parse error", plan, err)
	}

	metadata.viewDefinitions = map[string]string{
		"first_view":  "select id from second_view",
		"second_view": "select id from first_view",
	}
	data = parse.NewQueryData([]string{"id"}, []string{"first_view"}, query.Predicate{})
	if plan, err := planner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil || !strings.Contains(err.Error(), "recursive view") {
		t.Errorf("CreatePlan() with cyclic views = (%v, %v), want recursive-view error", plan, err)
	}
}

func TestBetterQueryPlannerChoosesLowerCostJoinOrder(t *testing.T) {
	for _, test := range []struct {
		name       string
		left       *metadata.StatInfo
		right      *metadata.StatInfo
		wantTables []string
	}{
		{
			name:       "swap when reversed order is cheaper",
			left:       metadata.NewStatInfo(1, 100),
			right:      metadata.NewStatInfo(100, 1),
			wantTables: []string{"right", "left"},
		},
		{
			name:       "preserve order when original is cheaper",
			left:       metadata.NewStatInfo(100, 1),
			right:      metadata.NewStatInfo(1, 10),
			wantTables: []string{"left", "right"},
		},
		{
			name:       "preserve order on equal cost",
			left:       metadata.NewStatInfo(1, 1),
			right:      metadata.NewStatInfo(1, 1),
			wantTables: []string{"left", "right"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			metadata := &basicPlannerMetadataProbe{
				layouts: map[string]*record.Layout{
					"left":  record.NewLayout(newPlannerTestSchema("left_id")),
					"right": record.NewLayout(newPlannerTestSchema("right_id")),
				},
				statistics: map[string]*metadata.StatInfo{"left": test.left, "right": test.right},
			}
			got, err := NewBetterQueryPlanner(metadata).CreatePlan(
				parse.NewQueryData([]string{"left_id", "right_id"}, []string{"left", "right"}, query.Predicate{}),
				&tablePlanTransaction{},
			)
			if err != nil {
				t.Fatalf("CreatePlan() error = %v", err)
			}
			if tables := basicPlannerTableOrder(t, got); !reflect.DeepEqual(tables, test.wantTables) {
				t.Errorf("table order = %v, want %v", tables, test.wantTables)
			}
		})
	}
}

func TestBetterQueryPlannerBuildsLeftDeepProductsAndRejectsInvalidInputs(t *testing.T) {
	metadata := &basicPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"first":  record.NewLayout(newPlannerTestSchema("first_id")),
			"second": record.NewLayout(newPlannerTestSchema("second_id")),
			"third":  record.NewLayout(newPlannerTestSchema("third_id")),
		},
		statistics: map[string]*metadata.StatInfo{
			"first":  metadata.NewStatInfo(1, 1),
			"second": metadata.NewStatInfo(1, 1),
			"third":  metadata.NewStatInfo(1, 1),
		},
	}
	planner := NewBetterQueryPlanner(metadata)
	data := parse.NewQueryData([]string{"first_id", "second_id", "third_id"}, []string{"first", "second", "third"}, query.Predicate{})
	got, err := planner.CreatePlan(data, &tablePlanTransaction{})
	if err != nil {
		t.Fatalf("CreatePlan() with three tables error = %v", err)
	}
	outerSelect := got.(*ProjectPlan).child.(*SelectPlan)
	product, ok := outerSelect.child.(*ProductPlan)
	if !ok {
		t.Fatalf("selection child type = %T, want *ProductPlan", outerSelect.child)
	}
	if _, ok := product.left.(*ProductPlan); !ok {
		t.Errorf("three-table product left type = %T, want *ProductPlan", product.left)
	}

	var nilPlanner *BetterQueryPlanner
	if plan, err := nilPlanner.CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("nil planner CreatePlan() = (%v, %v), want an error", plan, err)
	}
	if plan, err := (&BetterQueryPlanner{}).CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("planner without basic planner CreatePlan() = (%v, %v), want an error", plan, err)
	}
	if plan, err := NewBetterQueryPlanner(nil).CreatePlan(data, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("planner without metadata CreatePlan() = (%v, %v), want an error", plan, err)
	}
	if plan, err := planner.CreatePlan(data, nil); plan != nil || err == nil {
		t.Errorf("CreatePlan() with nil transaction = (%v, %v), want an error", plan, err)
	}
	if plan, err := planner.CreatePlan(parse.QueryData{}, &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("CreatePlan() without sources = (%v, %v), want an error", plan, err)
	}
	if plan, err := planner.CreatePlan(parse.NewQueryData([]string{"missing"}, []string{"first"}, query.Predicate{}), &tablePlanTransaction{}); plan != nil || err == nil {
		t.Errorf("CreatePlan() with unknown projection = (%v, %v), want an error", plan, err)
	}
}

func TestBetterQueryPlannerReusesRecursiveViewExpansion(t *testing.T) {
	metadata := &basicPlannerMetadataProbe{
		layouts: map[string]*record.Layout{
			"left":  record.NewLayout(newPlannerTestSchema("left_id")),
			"right": record.NewLayout(newPlannerTestSchema("right_id")),
		},
		statistics: map[string]*metadata.StatInfo{
			"left":  metadata.NewStatInfo(1, 100),
			"right": metadata.NewStatInfo(100, 1),
		},
		viewDefinitions: map[string]string{"right_view": "select right_id from right"},
	}
	data := parse.NewQueryData([]string{"left_id", "right_id"}, []string{"left", "right_view"}, query.Predicate{})

	got, err := NewBetterQueryPlanner(metadata).CreatePlan(data, &tablePlanTransaction{})
	if err != nil {
		t.Fatalf("CreatePlan() with view source error = %v", err)
	}
	if tables := basicPlannerTableOrder(t, got); !reflect.DeepEqual(tables, []string{"right", "left"}) {
		t.Errorf("expanded and ordered tables = %v, want [right left]", tables)
	}
}

func basicPlannerTableOrder(t *testing.T, plan Plan) []string {
	t.Helper()
	switch current := plan.(type) {
	case *ProjectPlan:
		return basicPlannerTableOrder(t, current.child)
	case *SelectPlan:
		return basicPlannerTableOrder(t, current.child)
	case *ProductPlan:
		return append(basicPlannerTableOrder(t, current.left), basicPlannerTableOrder(t, current.right)...)
	case *TablePlan:
		return []string{current.tableName}
	default:
		t.Fatalf("unexpected plan node %T", plan)
		return nil
	}
}

func newPlannerTestSchema(fieldName string) *record.Schema {
	schema := &record.Schema{}
	schema.AddIntegerField(fieldName)
	return schema
}

type basicPlannerMetadataProbe struct {
	layouts            map[string]*record.Layout
	statistics         map[string]*metadata.StatInfo
	viewDefinitions    map[string]string
	layoutErr          error
	statisticsErr      error
	viewErr            error
	layoutRequests     []string
	statisticsRequests []string
	viewRequests       []string
}

func (probe *basicPlannerMetadataProbe) ViewDefinition(viewName string, _ record.TableScanTransaction) (string, bool, error) {
	probe.viewRequests = append(probe.viewRequests, viewName)
	if probe.viewErr != nil {
		return "", false, probe.viewErr
	}
	definition, found := probe.viewDefinitions[viewName]
	return definition, found, nil
}

func (probe *basicPlannerMetadataProbe) Layout(tableName string, _ record.TableScanTransaction) (*record.Layout, error) {
	probe.layoutRequests = append(probe.layoutRequests, tableName)
	if probe.layoutErr != nil {
		return nil, probe.layoutErr
	}
	return probe.layouts[tableName], nil
}

func (probe *basicPlannerMetadataProbe) Statistics(tableName string, _ *record.Layout, _ record.TableScanTransaction) (*metadata.StatInfo, error) {
	probe.statisticsRequests = append(probe.statisticsRequests, tableName)
	if probe.statisticsErr != nil {
		return nil, probe.statisticsErr
	}
	return probe.statistics[tableName], nil
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

type basicUpdatePlannerMetadataProbe struct {
	layout                  *record.Layout
	layoutErr               error
	layoutRequests          []string
	createTableErr          error
	createTableRequests     []string
	createTableSchemas      []record.Schema
	createTableTransactions []record.TableScanTransaction
	createViewErr           error
	createViewRequests      []string
	createViewDefinitions   []string
	createViewTransactions  []record.TableScanTransaction
	createIndexErr          error
	createIndexNames        []string
	createIndexTables       []string
	createIndexFields       []string
	createIndexTransactions []record.TableScanTransaction
}

func (probe *basicUpdatePlannerMetadataProbe) Layout(tableName string, _ record.TableScanTransaction) (*record.Layout, error) {
	probe.layoutRequests = append(probe.layoutRequests, tableName)
	return probe.layout, probe.layoutErr
}
func (probe *basicUpdatePlannerMetadataProbe) CreateTable(tableName string, schema *record.Schema, transaction record.TableScanTransaction) error {
	probe.createTableRequests = append(probe.createTableRequests, tableName)
	if schema != nil {
		probe.createTableSchemas = append(probe.createTableSchemas, *schema)
	}
	probe.createTableTransactions = append(probe.createTableTransactions, transaction)
	return probe.createTableErr
}
func (probe *basicUpdatePlannerMetadataProbe) CreateView(viewName, definition string, transaction record.TableScanTransaction) error {
	probe.createViewRequests = append(probe.createViewRequests, viewName)
	probe.createViewDefinitions = append(probe.createViewDefinitions, definition)
	probe.createViewTransactions = append(probe.createViewTransactions, transaction)
	return probe.createViewErr
}
func (probe *basicUpdatePlannerMetadataProbe) CreateIndex(indexName, tableName, fieldName string, transaction record.TableScanTransaction) error {
	probe.createIndexNames = append(probe.createIndexNames, indexName)
	probe.createIndexTables = append(probe.createIndexTables, tableName)
	probe.createIndexFields = append(probe.createIndexFields, fieldName)
	probe.createIndexTransactions = append(probe.createIndexTransactions, transaction)
	return probe.createIndexErr
}

type basicUpdatePlannerTransaction struct {
	tablePlanTransaction
	slotSize       int
	blockSize      int
	flags          []int32
	values         []int32
	readErrOffset  int
	readErr        error
	writeErrOffset int
	writeErr       error
	unpinErr       error
}

func newBasicUpdatePlannerTransaction(layout *record.Layout, flags, values []int32) *basicUpdatePlannerTransaction {
	slotSize := layout.SlotSize()
	return &basicUpdatePlannerTransaction{
		tablePlanTransaction: tablePlanTransaction{size: 1},
		slotSize:             slotSize,
		blockSize:            slotSize * len(flags),
		flags:                append([]int32(nil), flags...),
		values:               append([]int32(nil), values...),
		readErrOffset:        -1,
		writeErrOffset:       -1,
	}
}

func (transaction *basicUpdatePlannerTransaction) BlockSize() (int, error) {
	return transaction.blockSize, nil
}

func (transaction *basicUpdatePlannerTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	if offset == transaction.readErrOffset && transaction.readErr != nil {
		return 0, transaction.readErr
	}
	slot := offset / transaction.slotSize
	if slot < 0 || slot >= len(transaction.flags) {
		return 0, errors.New("record offset outside test block")
	}
	if offset%transaction.slotSize == 0 {
		return transaction.flags[slot], nil
	}
	return transaction.values[slot], nil
}

func (*basicUpdatePlannerTransaction) GetString(storage.BlockID, int) (string, error) { return "", nil }

func (transaction *basicUpdatePlannerTransaction) SetInt(_ storage.BlockID, offset int32, value int32, _ bool) error {
	if int(offset) == transaction.writeErrOffset && transaction.writeErr != nil {
		return transaction.writeErr
	}
	slot := int(offset) / transaction.slotSize
	if slot < 0 || slot >= len(transaction.flags) {
		return errors.New("record offset outside test block")
	}
	if int(offset)%transaction.slotSize == 0 {
		transaction.flags[slot] = value
	} else {
		transaction.values[slot] = value
	}
	return nil
}

func (*basicUpdatePlannerTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func (transaction *basicUpdatePlannerTransaction) Unpin(block storage.BlockID) error {
	transaction.unpins = append(transaction.unpins, block)
	return transaction.unpinErr
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
