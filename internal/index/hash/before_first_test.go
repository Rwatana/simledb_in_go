package hash

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestHashIndexBeforeFirstOpensHashedBucket(t *testing.T) {
	tests := []struct {
		name string
		key  query.Constant
		want int
	}{
		{name: "integer key", key: query.NewIntConstant(123), want: 23},
		{name: "string key", key: query.NewStringConstant("abc"), want: 54},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transaction := &hashIndexTransactionProbe{size: 1}
			indexName := "student_id"
			index := NewHashIndex(transaction, indexName, record.NewLayout(&record.Schema{}))
			if err := index.BeforeFirst(test.key); err != nil {
				t.Fatalf("BeforeFirst() error = %v", err)
			}
			wantFile := indexName + strconv.Itoa(test.want) + ".tbl"
			if index.searchKey != test.key || index.scan == nil {
				t.Errorf("positioned index = (key %v, scan %v), want key retained and scan opened", index.searchKey, index.scan)
			}
			if len(transaction.sizeFiles) != 1 || transaction.sizeFiles[0] != wantFile {
				t.Errorf("bucket table size calls = %v, want [%s]", transaction.sizeFiles, wantFile)
			}
			if len(transaction.pins) != 1 || transaction.pins[0] != storage.NewBlockID(wantFile, 0) {
				t.Errorf("bucket pins = %v, want block zero of %s", transaction.pins, wantFile)
			}
		})
	}
}

func TestHashIndexBeforeFirstClosesOldScanBeforeOpeningNewBucket(t *testing.T) {
	transaction := &hashIndexTransactionProbe{size: 1}
	index := NewHashIndex(transaction, "student_id", record.NewLayout(&record.Schema{}))
	if err := index.BeforeFirst(query.NewIntConstant(123)); err != nil {
		t.Fatalf("first BeforeFirst() error = %v", err)
	}
	oldScan := index.scan
	if err := index.BeforeFirst(query.NewIntConstant(224)); err != nil {
		t.Fatalf("second BeforeFirst() error = %v", err)
	}
	wantEvents := []string{
		"size:student_id23.tbl",
		"pin:[file student_id23.tbl, block 0]",
		"unpin:[file student_id23.tbl, block 0]",
		"size:student_id24.tbl",
		"pin:[file student_id24.tbl, block 0]",
	}
	if !reflect.DeepEqual(transaction.events, wantEvents) {
		t.Errorf("transaction events = %v, want %v", transaction.events, wantEvents)
	}
	if index.scan == nil || index.scan == oldScan || !index.searchKey.Equal(query.NewIntConstant(224)) {
		t.Errorf("repositioned state = (key %v, scan %v), want second key and a new scan", index.searchKey, index.scan)
	}
}

func TestHashIndexBeforeFirstRejectsInvalidInputsWithoutIO(t *testing.T) {
	layout := record.NewLayout(&record.Schema{})
	transaction := &hashIndexTransactionProbe{size: 1}
	tests := []struct {
		name  string
		index *HashIndex
		key   query.Constant
	}{
		{name: "nil receiver", index: nil, key: query.NewIntConstant(1)},
		{name: "nil transaction", index: NewHashIndex(nil, "student_id", layout), key: query.NewIntConstant(1)},
		{name: "nil layout", index: NewHashIndex(transaction, "student_id", nil), key: query.NewIntConstant(1)},
		{name: "unset key", index: NewHashIndex(transaction, "student_id", layout), key: query.Constant{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.index.BeforeFirst(test.key); err == nil {
				t.Fatal("BeforeFirst() error = nil, want validation error")
			}
		})
	}
	if transaction.calls != 0 {
		t.Errorf("invalid requests made %d transaction calls, want none", transaction.calls)
	}
}

func TestHashIndexBeforeFirstPreservesScanWhenClosingFails(t *testing.T) {
	unpinErr := errors.New("unpin failed")
	transaction := &hashIndexTransactionProbe{size: 1, unpinErr: unpinErr}
	layout := record.NewLayout(&record.Schema{})
	scan, err := record.NewTableScan(transaction, "student_old", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index := NewHashIndex(transaction, "student_id", layout)
	index.searchKey = query.NewIntConstant(7)
	index.scan = scan

	err = index.BeforeFirst(query.NewIntConstant(9))
	if !errors.Is(err, unpinErr) || !strings.Contains(err.Error(), "student_id") {
		t.Fatalf("BeforeFirst() close error = %v, want contextual unpin cause", err)
	}
	if index.scan != scan || !index.searchKey.Equal(query.NewIntConstant(7)) {
		t.Errorf("state after failed close = (key %v, scan %v), want previous state retained", index.searchKey, index.scan)
	}
	if len(transaction.sizeFiles) != 1 {
		t.Errorf("Size calls after close failure = %v, want only the original table open", transaction.sizeFiles)
	}
}

func TestHashIndexBeforeFirstLeavesUnsetStateWhenBucketOpenFails(t *testing.T) {
	sizeErr := errors.New("size failed")
	pinErr := errors.New("pin failed")
	tests := []struct {
		name        string
		transaction *hashIndexTransactionProbe
		cause       error
	}{
		{name: "size", transaction: &hashIndexTransactionProbe{sizeErr: sizeErr}, cause: sizeErr},
		{name: "pin", transaction: &hashIndexTransactionProbe{size: 1, pinErr: pinErr}, cause: pinErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := NewHashIndex(test.transaction, "student_id", record.NewLayout(&record.Schema{}))
			err := index.BeforeFirst(query.NewIntConstant(123))
			if !errors.Is(err, test.cause) || !strings.Contains(err.Error(), "student_id23") {
				t.Fatalf("BeforeFirst() error = %v, want bucket context and original cause", err)
			}
			if index.scan != nil {
				t.Errorf("scan after bucket open failure = %v, want nil", index.scan)
			}
			if _, initialized := index.searchKey.HashCode(); initialized {
				t.Errorf("search key after bucket open failure = %v, want unset", index.searchKey)
			}
		})
	}
}
