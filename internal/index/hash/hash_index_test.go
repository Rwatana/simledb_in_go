package hash

import (
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
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
