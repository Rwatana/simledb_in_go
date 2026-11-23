package hash

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBucketNumberUsesJavaCompatibleHashesAndNonnegativeRemainders(t *testing.T) {
	tests := []struct {
		name  string
		value query.Constant
		want  int
	}{
		{name: "positive integer", value: query.NewIntConstant(123), want: 23},
		{name: "negative integer", value: query.NewIntConstant(-1), want: 99},
		{name: "negative multiple range", value: query.NewIntConstant(-101), want: 99},
		{name: "ASCII Java string hash", value: query.NewStringConstant("abc"), want: 54},
		{name: "BMP string hash", value: query.NewStringConstant("é"), want: 33},
		{name: "supplementary Unicode UTF-16 hash", value: query.NewStringConstant("😀"), want: 99},
		{name: "empty string hash", value: query.NewStringConstant(""), want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := BucketNumber(test.value)
			if err != nil {
				t.Fatalf("BucketNumber() error = %v", err)
			}
			if got != test.want {
				t.Errorf("BucketNumber() = %d, want %d", got, test.want)
			}
			if got < 0 || got >= BucketCount {
				t.Errorf("BucketNumber() = %d, want a bucket in [0, %d)", got, BucketCount)
			}
		})
	}
}

func TestBucketNumberRejectsUnsetConstant(t *testing.T) {
	if bucket, err := BucketNumber(query.Constant{}); err == nil || bucket != 0 || !strings.Contains(err.Error(), "unset") {
		t.Fatalf("BucketNumber(unset) = (%d, %v), want contextual unset-value error", bucket, err)
	}
}

type hashIndexTransactionProbe struct {
	record.RecordTransaction
	calls            int
	size             int
	pinErr, unpinErr error
	pins, unpins     []storage.BlockID
}

func (transaction *hashIndexTransactionProbe) Pin(block storage.BlockID) error {
	transaction.calls++
	transaction.pins = append(transaction.pins, block)
	return transaction.pinErr
}

func (transaction *hashIndexTransactionProbe) Size(string) (int, error) {
	transaction.calls++
	return transaction.size, nil
}

func (transaction *hashIndexTransactionProbe) Append(string) (storage.BlockID, error) {
	transaction.calls++
	return storage.BlockID{}, nil
}

func (transaction *hashIndexTransactionProbe) Unpin(block storage.BlockID) error {
	transaction.calls++
	transaction.unpins = append(transaction.unpins, block)
	return transaction.unpinErr
}

func TestNewHashIndexRetainsConfigurationWithoutTransactionIO(t *testing.T) {
	transaction := &hashIndexTransactionProbe{}
	layout := record.NewLayout(&record.Schema{})
	index := NewHashIndex(transaction, "student-id", layout)
	if index == nil {
		t.Fatal("NewHashIndex() returned nil")
	}
	if index.transaction != transaction || index.indexName != "student-id" || index.layout != layout {
		t.Fatalf("index configuration = %+v, want transaction, name, and layout retained", index)
	}
	if _, initialized := index.searchKey.HashCode(); initialized || index.scan != nil {
		t.Fatalf("initial search state = (key %v, scan %v), want unset key and no scan", index.searchKey, index.scan)
	}
	if transaction.calls != 0 {
		t.Errorf("constructor made %d table transaction calls, want none", transaction.calls)
	}
}

func TestHashIndexCloseWithoutScanIsNoOp(t *testing.T) {
	transaction := &hashIndexTransactionProbe{}
	index := NewHashIndex(transaction, "student-id", record.NewLayout(&record.Schema{}))
	if err := index.Close(); err != nil {
		t.Fatalf("Close() without scan error = %v", err)
	}
	if transaction.calls != 0 || index.scan != nil {
		t.Errorf("close without scan made %d transaction calls and left scan %v", transaction.calls, index.scan)
	}
}

func TestHashIndexCloseUnpinsAndClearsScanOnce(t *testing.T) {
	transaction := &hashIndexTransactionProbe{size: 1}
	layout := record.NewLayout(&record.Schema{})
	scan, err := record.NewTableScan(transaction, "student-index", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index := NewHashIndex(transaction, "student-id", layout)
	index.scan = scan

	if err := index.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if index.scan != nil {
		t.Errorf("scan after Close() = %v, want nil", index.scan)
	}
	if len(transaction.unpins) != 1 {
		t.Fatalf("Unpin calls = %v, want one call", transaction.unpins)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
	if len(transaction.unpins) != 1 {
		t.Errorf("Unpin calls after repeated close = %v, want one call", transaction.unpins)
	}
}

func TestHashIndexCloseRetainsScanWhenUnpinFails(t *testing.T) {
	unpinErr := errors.New("unpin failed")
	transaction := &hashIndexTransactionProbe{size: 1, unpinErr: unpinErr}
	layout := record.NewLayout(&record.Schema{})
	scan, err := record.NewTableScan(transaction, "student-index", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index := NewHashIndex(transaction, "student-id", layout)
	index.scan = scan

	if err := index.Close(); !errors.Is(err, unpinErr) || !strings.Contains(err.Error(), "student-id") || index.scan != scan {
		t.Fatalf("Close() on unpin failure = (%v, scan %v), want wrapped error and retryable scan", err, index.scan)
	}
	transaction.unpinErr = nil
	if err := index.Close(); err != nil {
		t.Fatalf("Close() retry error = %v", err)
	}
	if index.scan != nil || len(transaction.unpins) != 2 {
		t.Errorf("after retry scan = %v, unpins = %v; want nil scan and two attempts", index.scan, transaction.unpins)
	}
}
