package btree

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestDirEntryRetainsKeyAndBlockNumber(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   query.Constant
		block int
	}{
		{name: "integer block zero", key: query.NewIntConstant(17), block: 0},
		{name: "varchar negative block", key: query.NewStringConstant("alpha"), block: -3},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := NewDirEntry(test.key, test.block)
			if got := entry.DataVal(); !got.Equal(test.key) {
				t.Fatalf("DataVal() = %v, want %v", got, test.key)
			}
			if got := entry.BlockNumber(); got != test.block {
				t.Fatalf("BlockNumber() = %d, want %d", got, test.block)
			}
		})
	}
}
