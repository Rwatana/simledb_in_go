package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/index"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func newIndexJoinIntegerRecordScan(t *testing.T, transaction record.TableScanTransaction, fields ...string) *query.RecordScan {
	t.Helper()
	schema := &record.Schema{}
	for _, field := range fields {
		schema.AddIntegerField(field)
	}
	scan, err := query.NewRecordScan(transaction, "students", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	return scan
}

type indexJoinIntegerTransaction struct {
	*positioningTransaction
	value        int32
	getIntErr    error
	getIntCalls  int
	getIntBlock  storage.BlockID
	getIntOffset int
}

func (transaction *indexJoinIntegerTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	transaction.getIntCalls++
	transaction.getIntBlock = block
	transaction.getIntOffset = offset
	if transaction.getIntErr != nil {
		return 0, transaction.getIntErr
	}
	return transaction.value, nil
}

func TestNewIndexJoinScanPositionsIndexFromFirstLeftRow(t *testing.T) {
	key := query.NewIntConstant(42)
	left := &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": key}}}
	indexScan := &positioningIndex{}
	right := newPositioningTableScan(t)

	scan, err := NewIndexJoinScan(left, indexScan, "student_id", right)
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	if scan == nil || scan.lhs != left || scan.index != indexScan || scan.joinField != "student_id" || scan.rhs != right {
		t.Fatalf("constructed scan = %#v, want retained inputs", scan)
	}
	if !scan.hasLeftRow {
		t.Fatal("constructed scan has no left row, want it positioned on the first row")
	}
	if left.beforeFirstCalls != 1 || left.nextCalls != 1 || len(left.getValFields) != 1 || left.getValFields[0] != "student_id" {
		t.Fatalf("left positioning: before-first=%d next=%d fields=%v", left.beforeFirstCalls, left.nextCalls, left.getValFields)
	}
	if len(indexScan.keys) != 1 || !indexScan.keys[0].Equal(key) {
		t.Fatalf("index positions = %v, want first left key %v exactly once", indexScan.keys, key)
	}
}

func TestNewIndexJoinScanLeavesEmptyLeftSideExhausted(t *testing.T) {
	left := &indexJoinLeftScan{}
	indexScan := &positioningIndex{}
	scan, err := NewIndexJoinScan(left, indexScan, "student_id", newPositioningTableScan(t))
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	if scan.hasLeftRow {
		t.Fatal("empty left input is marked as having a current row")
	}
	if len(indexScan.keys) != 0 {
		t.Fatalf("empty left input positioned index with keys %v", indexScan.keys)
	}
}

func TestIndexJoinScanBeforeFirstRepositionsFromFirstLeftRow(t *testing.T) {
	key := query.NewStringConstant("A-1")
	left := &indexJoinLeftScan{rows: []map[string]query.Constant{{"course_id": key}}}
	indexScan := &positioningIndex{}
	scan, err := NewIndexJoinScan(left, indexScan, "course_id", newPositioningTableScan(t))
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if left.beforeFirstCalls != 2 || left.nextCalls != 2 || len(indexScan.keys) != 2 || !indexScan.keys[0].Equal(key) || !indexScan.keys[1].Equal(key) {
		t.Fatalf("reposition counts: before-first=%d next=%d keys=%v", left.beforeFirstCalls, left.nextCalls, indexScan.keys)
	}
}

func TestNewIndexJoinScanRejectsMissingDependencies(t *testing.T) {
	if err := (*IndexJoinScan)(nil).BeforeFirst(); err == nil || !strings.Contains(err.Error(), "nil index join scan") {
		t.Fatalf("nil scan BeforeFirst() error = %v, want nil-receiver error", err)
	}
	key := query.NewIntConstant(42)
	validLeft := &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": key}}}
	validIndex := &positioningIndex{}
	validRight := newPositioningTableScan(t)
	tests := []struct {
		name  string
		left  query.Scan
		index index.Index
		right *query.RecordScan
	}{
		{name: "nil left scan", index: validIndex, right: validRight},
		{name: "nil index", left: validLeft, right: validRight},
		{name: "nil right scan", left: validLeft, index: validIndex},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := NewIndexJoinScan(test.left, test.index, "student_id", test.right)
			if scan != nil || err == nil || !strings.Contains(err.Error(), "index join scan") {
				t.Fatalf("NewIndexJoinScan() = (%v, %v), want contextual validation error", scan, err)
			}
		})
	}
	if validLeft.beforeFirstCalls != 0 || len(validIndex.keys) != 0 {
		t.Fatalf("valid dependencies were touched while rejecting inputs: lhs resets=%d index keys=%v", validLeft.beforeFirstCalls, validIndex.keys)
	}
}

func TestNewIndexJoinScanPreservesPositioningFailures(t *testing.T) {
	beforeFirstErr := errors.New("left reset failed")
	nextErr := errors.New("left advance failed")
	valueErr := errors.New("left key read failed")
	indexErr := errors.New("index position failed")
	tests := []struct {
		name      string
		left      *indexJoinLeftScan
		index     *indexJoinPositioningIndex
		wantCause error
		wantText  string
	}{
		{name: "left reset", left: &indexJoinLeftScan{beforeFirstErr: beforeFirstErr}, index: &indexJoinPositioningIndex{}, wantCause: beforeFirstErr, wantText: "left scan"},
		{name: "left advance", left: &indexJoinLeftScan{nextErr: nextErr}, index: &indexJoinPositioningIndex{}, wantCause: nextErr, wantText: "left scan"},
		{name: "join value read", left: &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(42)}}, getValErr: valueErr}, index: &indexJoinPositioningIndex{}, wantCause: valueErr, wantText: "join field"},
		{name: "index positioning", left: &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(42)}}}, index: &indexJoinPositioningIndex{positionErr: indexErr}, wantCause: indexErr, wantText: "index"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := NewIndexJoinScan(test.left, test.index, "student_id", newPositioningTableScan(t))
			if scan != nil || !errors.Is(err, test.wantCause) || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("NewIndexJoinScan() = (%v, %v), want contextual error wrapping %v", scan, err, test.wantCause)
			}
			if test.left.closeCalls != 0 {
				t.Errorf("constructor closed caller-owned left scan %d times", test.left.closeCalls)
			}
			if test.index.closeCalls != 0 {
				t.Errorf("constructor closed caller-owned index %d times", test.index.closeCalls)
			}
			if len(test.index.keys) > 1 {
				t.Errorf("index positioning calls = %v, want at most one", test.index.keys)
			}
		})
	}
}

func TestIndexJoinScanGetIntReadsRightTableField(t *testing.T) {
	transaction := &indexJoinIntegerTransaction{positioningTransaction: &positioningTransaction{}, value: 73}
	right := newIndexJoinIntegerRecordScan(t, transaction, "id")
	if err := right.MoveToRID(record.NewRID(0, 2)); err != nil {
		t.Fatalf("right MoveToRID() error = %v", err)
	}
	left := &indexJoinLeftScan{fieldNames: map[string]bool{"id": true}, intValues: map[string]int32{"id": 11}}
	scan := &IndexJoinScan{lhs: left, rhs: right}

	got, err := scan.GetInt("id")
	if err != nil || got != 73 {
		t.Fatalf("GetInt(\"id\") = (%d, %v), want (73, nil)", got, err)
	}
	if transaction.getIntCalls != 1 || transaction.getIntBlock.Number() != 0 || transaction.getIntOffset < 0 {
		t.Fatalf("right integer reads = %d at %v/%d, want one read for current record", transaction.getIntCalls, transaction.getIntBlock, transaction.getIntOffset)
	}
	if len(left.getIntFields) != 0 {
		t.Fatalf("left GetInt calls = %v, want none when right declares the field", left.getIntFields)
	}
}

func TestIndexJoinScanGetIntReadsLeftWhenRightDoesNotDeclareField(t *testing.T) {
	transaction := &indexJoinIntegerTransaction{positioningTransaction: &positioningTransaction{}, value: 73}
	right := newIndexJoinIntegerRecordScan(t, transaction, "id")
	left := &indexJoinLeftScan{fieldNames: map[string]bool{"student_id": true}, intValues: map[string]int32{"student_id": 41}}
	scan := &IndexJoinScan{lhs: left, rhs: right}

	got, err := scan.GetInt("student_id")
	if err != nil || got != 41 {
		t.Fatalf("GetInt(\"student_id\") = (%d, %v), want (41, nil)", got, err)
	}
	if len(left.getIntFields) != 1 || left.getIntFields[0] != "student_id" {
		t.Fatalf("left GetInt calls = %v, want [student_id]", left.getIntFields)
	}
	if transaction.getIntCalls != 0 {
		t.Fatalf("right integer reads = %d, want none for a left-only field", transaction.getIntCalls)
	}
}

func TestIndexJoinScanGetIntValidatesReceiverAndDependencies(t *testing.T) {
	right := newPositioningTableScan(t)
	left := &indexJoinLeftScan{}
	tests := []struct {
		name string
		scan *IndexJoinScan
		want string
	}{
		{name: "nil receiver", want: "nil index join scan"},
		{name: "left scan", scan: &IndexJoinScan{rhs: right}, want: "left scan is nil"},
		{name: "right scan", scan: &IndexJoinScan{lhs: left}, want: "right table scan is nil"},
		{name: "right table scan", scan: &IndexJoinScan{lhs: left, rhs: &query.RecordScan{}}, want: "right table scan is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.scan.GetInt("id")
			if got != 0 || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("GetInt() = (%d, %v), want error containing %q", got, err, test.want)
			}
		})
	}
}

func TestIndexJoinScanGetIntWrapsSelectedScanErrors(t *testing.T) {
	rightErr := errors.New("right integer read failed")
	leftErr := errors.New("left integer read failed")
	rightTransaction := &indexJoinIntegerTransaction{positioningTransaction: &positioningTransaction{}, getIntErr: rightErr}
	rightScan := newIndexJoinIntegerRecordScan(t, rightTransaction, "id")
	if err := rightScan.MoveToRID(record.NewRID(0, 2)); err != nil {
		t.Fatalf("right MoveToRID() error = %v", err)
	}
	leftScan := &indexJoinLeftScan{fieldNames: map[string]bool{"id": true}}
	rightIndexJoinScan := &IndexJoinScan{lhs: leftScan, rhs: rightScan}
	if _, err := rightIndexJoinScan.GetInt("id"); !errors.Is(err, rightErr) || !strings.Contains(err.Error(), "right table") {
		t.Fatalf("right GetInt() error = %v, want wrapped cause %v", err, rightErr)
	}

	leftScan = &indexJoinLeftScan{fieldNames: map[string]bool{"student_id": true}, getIntErr: leftErr}
	leftIndexJoinScan := &IndexJoinScan{lhs: leftScan, rhs: newPositioningTableScan(t)}
	if _, err := leftIndexJoinScan.GetInt("student_id"); !errors.Is(err, leftErr) || !strings.Contains(err.Error(), "left scan") {
		t.Fatalf("left GetInt() error = %v, want wrapped cause %v", err, leftErr)
	}
}

type indexJoinPositioningIndex struct {
	keys        []query.Constant
	positionErr error
	closeCalls  int
}

func (index *indexJoinPositioningIndex) BeforeFirst(key query.Constant) error {
	index.keys = append(index.keys, key)
	return index.positionErr
}

func (*indexJoinPositioningIndex) Next() (bool, error) { return false, nil }

func (*indexJoinPositioningIndex) GetDataRID() (record.RID, error) { return record.RID{}, nil }

func (*indexJoinPositioningIndex) Insert(query.Constant, record.RID) error { return nil }

func (*indexJoinPositioningIndex) Delete(query.Constant, record.RID) error { return nil }

func (index *indexJoinPositioningIndex) Close() error {
	index.closeCalls++
	return nil
}

type indexJoinLeftScan struct {
	rows             []map[string]query.Constant
	fieldNames       map[string]bool
	intValues        map[string]int32
	position         int
	beforeFirstErr   error
	nextErr          error
	nextErrCall      int
	getValErr        error
	getValErrCall    int
	beforeFirstCalls int
	nextCalls        int
	getValFields     []string
	getIntFields     []string
	getIntErr        error
	closeCalls       int
}

func (scan *indexJoinLeftScan) BeforeFirst() error {
	scan.beforeFirstCalls++
	if scan.beforeFirstErr != nil {
		return scan.beforeFirstErr
	}
	scan.position = -1
	return nil
}

func (scan *indexJoinLeftScan) Next() (bool, error) {
	scan.nextCalls++
	if scan.nextErr != nil && (scan.nextErrCall == 0 || scan.nextErrCall == scan.nextCalls) {
		return false, scan.nextErr
	}
	scan.position++
	return scan.position < len(scan.rows), nil
}

func (scan *indexJoinLeftScan) GetVal(fieldName string) (query.Constant, error) {
	scan.getValFields = append(scan.getValFields, fieldName)
	if scan.getValErr != nil && (scan.getValErrCall == 0 || scan.getValErrCall == len(scan.getValFields)) {
		return query.Constant{}, scan.getValErr
	}
	if scan.position < 0 || scan.position >= len(scan.rows) {
		return query.Constant{}, errors.New("left scan is not positioned on a row")
	}
	value, ok := scan.rows[scan.position][fieldName]
	if !ok {
		return query.Constant{}, errors.New("join field is missing")
	}
	return value, nil
}

func (scan *indexJoinLeftScan) GetInt(fieldName string) (int32, error) {
	scan.getIntFields = append(scan.getIntFields, fieldName)
	if scan.getIntErr != nil {
		return 0, scan.getIntErr
	}
	return scan.intValues[fieldName], nil
}
func (*indexJoinLeftScan) GetString(string) (string, error) { return "", nil }
func (scan *indexJoinLeftScan) HasField(fieldName string) bool {
	if scan.fieldNames == nil {
		return true
	}
	return scan.fieldNames[fieldName]
}
func (scan *indexJoinLeftScan) Close() error { scan.closeCalls++; return nil }
