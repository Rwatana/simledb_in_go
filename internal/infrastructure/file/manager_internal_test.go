package file

import (
	"math"
	"strings"
	"testing"
)

func TestBlockCountToIntEnforcesPlatformLimit(t *testing.T) {
	t.Parallel()

	const maxInt = int64(math.MaxInt32)

	got, err := blockCountToInt("student.tbl", maxInt, maxInt)
	if err != nil || got != math.MaxInt32 {
		t.Fatalf("blockCountToInt() = (%d, %v), want (%d, nil)", got, err, math.MaxInt32)
	}

	got, err = blockCountToInt("student.tbl", maxInt+1, maxInt)
	if got != 0 {
		t.Errorf("blockCountToInt() overflow result = %d, want 0", got)
	}
	if err == nil || !strings.Contains(err.Error(), "student.tbl") || !strings.Contains(err.Error(), "exceeding platform int capacity") {
		t.Errorf("blockCountToInt() overflow error = %v, want file-specific platform-int error", err)
	}
}
