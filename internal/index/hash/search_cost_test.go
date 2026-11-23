package hash

import "testing"

func TestSearchCostEstimatesOneOfOneHundredBuckets(t *testing.T) {
	tests := []struct {
		name            string
		numBlocks       int
		recordsPerBlock int
		want            int
	}{
		{name: "empty index", numBlocks: 0, recordsPerBlock: 10, want: 0},
		{name: "below one bucket", numBlocks: 99, recordsPerBlock: 10, want: 0},
		{name: "exactly one bucket", numBlocks: 100, recordsPerBlock: 10, want: 1},
		{name: "fractional bucket rounds down", numBlocks: 199, recordsPerBlock: 10, want: 1},
		{name: "multiple buckets", numBlocks: 500, recordsPerBlock: 10, want: 5},
		{name: "records per block is ignored", numBlocks: 500, recordsPerBlock: 0, want: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SearchCost(test.numBlocks, test.recordsPerBlock); got != test.want {
				t.Errorf("SearchCost(%d, %d) = %d, want %d", test.numBlocks, test.recordsPerBlock, got, test.want)
			}
		})
	}
}
