package hash

import (
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
	calls int
}

func (transaction *hashIndexTransactionProbe) Size(string) (int, error) {
	transaction.calls++
	return 0, nil
}

func (transaction *hashIndexTransactionProbe) Append(string) (storage.BlockID, error) {
	transaction.calls++
	return storage.BlockID{}, nil
}

func (transaction *hashIndexTransactionProbe) Unpin(storage.BlockID) error {
	transaction.calls++
	return nil
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
