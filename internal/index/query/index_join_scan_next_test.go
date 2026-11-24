package indexquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestIndexJoinScanNextVisitsMatchingRIDsAcrossLeftRows(t *testing.T) {
	firstKey := query.NewIntConstant(1)
	secondKey := query.NewIntConstant(2)
	left := &indexJoinLeftScan{rows: []map[string]query.Constant{
		{"student_id": firstKey},
		{"student_id": secondKey},
	}}
	firstRID := record.NewRID(0, 1)
	secondRID := record.NewRID(0, 2)
	thirdRID := record.NewRID(1, 3)
	indexScan := &indexJoinAdvancingIndex{entriesByPosition: [][]record.RID{{firstRID, secondRID}, {thirdRID}}}
	right := newPositioningTableScan(t)
	scan, err := NewIndexJoinScan(left, indexScan, "student_id", right)
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	for i, want := range []record.RID{firstRID, secondRID, thirdRID} {
		found, err := scan.Next()
		if err != nil || !found {
			t.Fatalf("Next() #%d = (%t, %v), want matching row", i+1, found, err)
		}
		got, err := right.GetRID()
		if err != nil || !got.Equal(want) {
			t.Fatalf("right GetRID() #%d = (%v, %v), want %v", i+1, got, err, want)
		}
	}
	if found, err := scan.Next(); err != nil || found {
		t.Fatalf("Next() after all matches = (%t, %v), want (false, nil)", found, err)
	}
	if scan.hasLeftRow {
		t.Fatal("exhausted scan still has a current left row")
	}
	if len(indexScan.beforeKeys) != 2 || !indexScan.beforeKeys[0].Equal(firstKey) || !indexScan.beforeKeys[1].Equal(secondKey) {
		t.Fatalf("index positions = %v, want keys %v and %v", indexScan.beforeKeys, firstKey, secondKey)
	}
}

func TestIndexJoinScanNextReturnsFalseForEmptyLeftInput(t *testing.T) {
	indexScan := &indexJoinAdvancingIndex{}
	scan, err := NewIndexJoinScan(&indexJoinLeftScan{}, indexScan, "student_id", newPositioningTableScan(t))
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	found, err := scan.Next()
	if err != nil || found {
		t.Fatalf("Next() on empty left input = (%t, %v), want (false, nil)", found, err)
	}
	if indexScan.nextCalls != 0 {
		t.Fatalf("index Next() calls = %d, want 0", indexScan.nextCalls)
	}
}

func TestIndexJoinScanNextRejectsMissingState(t *testing.T) {
	right := newPositioningTableScan(t)
	left := &indexJoinLeftScan{}
	indexScan := &indexJoinAdvancingIndex{}
	tests := []struct {
		name string
		scan *IndexJoinScan
		want string
	}{
		{name: "nil receiver", want: "nil index join scan"},
		{name: "left scan", scan: &IndexJoinScan{index: indexScan, rhs: right, hasLeftRow: true}, want: "left scan is nil"},
		{name: "index", scan: &IndexJoinScan{lhs: left, rhs: right, hasLeftRow: true}, want: "index is nil"},
		{name: "right scan", scan: &IndexJoinScan{lhs: left, index: indexScan, hasLeftRow: true}, want: "right table scan is nil"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found, err := test.scan.Next()
			if found || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Next() = (%t, %v), want contextual error containing %q", found, err, test.want)
			}
		})
	}
}

func TestIndexJoinScanNextPreservesIndexAndRIDFailures(t *testing.T) {
	nextErr := errors.New("index iteration failed")
	ridErr := errors.New("index RID read failed")
	tests := []struct {
		name      string
		index     *indexJoinAdvancingIndex
		wantCause error
		wantText  string
	}{
		{name: "index iteration", index: &indexJoinAdvancingIndex{nextErr: nextErr}, wantCause: nextErr, wantText: "index"},
		{name: "RID read", index: &indexJoinAdvancingIndex{entriesByPosition: [][]record.RID{{record.NewRID(0, 1)}}, ridErr: ridErr}, wantCause: ridErr, wantText: "RID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan := newIndexJoinScanForNextTest(t, []query.Constant{query.NewIntConstant(1)}, test.index, newPositioningTableScan(t))
			found, err := scan.Next()
			if found || !errors.Is(err, test.wantCause) || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("Next() = (%t, %v), want wrapped error %v", found, err, test.wantCause)
			}
		})
	}
}

func TestIndexJoinScanNextPreservesLeftAndIndexRepositionFailures(t *testing.T) {
	nextErr := errors.New("left advance failed")
	valueErr := errors.New("left join value failed")
	positionErr := errors.New("index reposition failed")
	tests := []struct {
		name      string
		left      *indexJoinLeftScan
		index     *indexJoinAdvancingIndex
		wantCause error
		wantText  string
	}{
		{
			name:      "left advance",
			left:      &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(1)}}, nextErr: nextErr, nextErrCall: 2},
			index:     &indexJoinAdvancingIndex{},
			wantCause: nextErr,
			wantText:  "left scan",
		},
		{
			name:      "join value read",
			left:      &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(1)}, {"student_id": query.NewIntConstant(2)}}, getValErr: valueErr, getValErrCall: 2},
			index:     &indexJoinAdvancingIndex{},
			wantCause: valueErr,
			wantText:  "join field",
		},
		{
			name:      "index reposition",
			left:      &indexJoinLeftScan{rows: []map[string]query.Constant{{"student_id": query.NewIntConstant(1)}, {"student_id": query.NewIntConstant(2)}}},
			index:     &indexJoinAdvancingIndex{beforeErr: positionErr, beforeErrCall: 2},
			wantCause: positionErr,
			wantText:  "index",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scan, err := NewIndexJoinScan(test.left, test.index, "student_id", newPositioningTableScan(t))
			if err != nil {
				t.Fatalf("NewIndexJoinScan() error = %v", err)
			}
			found, err := scan.Next()
			if found || !errors.Is(err, test.wantCause) || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("Next() = (%t, %v), want wrapped error %v", found, err, test.wantCause)
			}
		})
	}
}

func TestIndexJoinScanNextPreservesRightRIDMoveFailure(t *testing.T) {
	moveErr := errors.New("right table pin failed")
	transaction := &indexJoinMoveTransaction{positioningTransaction: &positioningTransaction{}, failPinCall: 2, pinErr: moveErr}
	right := newIndexJoinRightScan(t, transaction)
	indexScan := &indexJoinAdvancingIndex{entriesByPosition: [][]record.RID{{record.NewRID(0, 2)}}}
	scan := newIndexJoinScanForNextTest(t, []query.Constant{query.NewIntConstant(1)}, indexScan, right)
	found, err := scan.Next()
	if found || !errors.Is(err, moveErr) || !strings.Contains(err.Error(), "right table") {
		t.Fatalf("Next() = (%t, %v), want wrapped right-side movement error %v", found, err, moveErr)
	}
}

func newIndexJoinScanForNextTest(t *testing.T, keys []query.Constant, indexScan *indexJoinAdvancingIndex, right *query.RecordScan) *IndexJoinScan {
	t.Helper()
	rows := make([]map[string]query.Constant, len(keys))
	for i, key := range keys {
		rows[i] = map[string]query.Constant{"student_id": key}
	}
	scan, err := NewIndexJoinScan(&indexJoinLeftScan{rows: rows}, indexScan, "student_id", right)
	if err != nil {
		t.Fatalf("NewIndexJoinScan() error = %v", err)
	}
	return scan
}

func newIndexJoinRightScan(t *testing.T, transaction record.TableScanTransaction) *query.RecordScan {
	t.Helper()
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	scan, err := query.NewRecordScan(transaction, "students", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	return scan
}

type indexJoinAdvancingIndex struct {
	entriesByPosition [][]record.RID
	current           []record.RID
	position          int
	currentRID        int
	beforeKeys        []query.Constant
	beforeErr         error
	beforeErrCall     int
	nextErr           error
	nextErrCall       int
	ridErr            error
	ridErrCall        int
	beforeCalls       int
	nextCalls         int
	ridCalls          int
}

func (index *indexJoinAdvancingIndex) BeforeFirst(key query.Constant) error {
	index.beforeCalls++
	index.beforeKeys = append(index.beforeKeys, key)
	if index.beforeErr != nil && (index.beforeErrCall == 0 || index.beforeErrCall == index.beforeCalls) {
		return index.beforeErr
	}
	position := index.beforeCalls - 1
	if position < len(index.entriesByPosition) {
		index.current = index.entriesByPosition[position]
	} else {
		index.current = nil
	}
	index.position = 0
	index.currentRID = -1
	return nil
}

func (index *indexJoinAdvancingIndex) Next() (bool, error) {
	index.nextCalls++
	if index.nextErr != nil && (index.nextErrCall == 0 || index.nextErrCall == index.nextCalls) {
		return false, index.nextErr
	}
	if index.position >= len(index.current) {
		return false, nil
	}
	index.currentRID = index.position
	index.position++
	return true, nil
}

func (index *indexJoinAdvancingIndex) GetDataRID() (record.RID, error) {
	index.ridCalls++
	if index.ridErr != nil && (index.ridErrCall == 0 || index.ridErrCall == index.ridCalls) {
		return record.RID{}, index.ridErr
	}
	if index.currentRID < 0 || index.currentRID >= len(index.current) {
		return record.RID{}, errors.New("index is not on an entry")
	}
	return index.current[index.currentRID], nil
}

func (*indexJoinAdvancingIndex) Insert(query.Constant, record.RID) error { return nil }

func (*indexJoinAdvancingIndex) Delete(query.Constant, record.RID) error { return nil }

func (*indexJoinAdvancingIndex) Close() error { return nil }

type indexJoinMoveTransaction struct {
	*positioningTransaction
	pinCalls    int
	failPinCall int
	pinErr      error
}

func (transaction *indexJoinMoveTransaction) Pin(block storage.BlockID) error {
	transaction.pinCalls++
	if transaction.pinErr != nil && transaction.pinCalls == transaction.failPinCall {
		return transaction.pinErr
	}
	return transaction.positioningTransaction.Pin(block)
}
