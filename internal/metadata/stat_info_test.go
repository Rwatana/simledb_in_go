package metadata

import "testing"

func TestNewStatInfoRetainsObservedCounts(t *testing.T) {
	stats := NewStatInfo(7, 23)
	if stats == nil {
		t.Fatal("NewStatInfo() returned nil")
	}
	if stats.blocks != 7 || stats.records != 23 {
		t.Fatalf("stored counts = (%d, %d), want (7, 23)", stats.blocks, stats.records)
	}

	empty := NewStatInfo(0, 0)
	if empty.blocks != 0 || empty.records != 0 {
		t.Fatalf("empty counts = (%d, %d), want (0, 0)", empty.blocks, empty.records)
	}
}

func TestStatInfoBlocksAccessedReturnsObservedBlocks(t *testing.T) {
	if got := NewStatInfo(7, 23).BlocksAccessed(); got != 7 {
		t.Fatalf("BlocksAccessed() = %d, want 7", got)
	}
	if got := NewStatInfo(0, 23).BlocksAccessed(); got != 0 {
		t.Fatalf("empty BlocksAccessed() = %d, want 0", got)
	}
}

func TestStatInfoRecordsOutputReturnsObservedRecords(t *testing.T) {
	if got := NewStatInfo(7, 23).RecordsOutput(); got != 23 {
		t.Fatalf("RecordsOutput() = %d, want 23", got)
	}
	if got := NewStatInfo(7, 0).RecordsOutput(); got != 0 {
		t.Fatalf("empty RecordsOutput() = %d, want 0", got)
	}
}

func TestStatInfoDistinctValuesUsesBookEstimate(t *testing.T) {
	tests := []struct {
		records int
		want    int
	}{
		{records: 0, want: 1},
		{records: 2, want: 1},
		{records: 3, want: 2},
		{records: 23, want: 8},
	}
	for _, test := range tests {
		stats := NewStatInfo(7, test.records)
		if got := stats.DistinctValues("sid"); got != test.want {
			t.Fatalf("DistinctValues(sid) with %d records = %d, want %d", test.records, got, test.want)
		}
		if got := stats.DistinctValues("other"); got != test.want {
			t.Fatalf("DistinctValues(other) with %d records = %d, want %d", test.records, got, test.want)
		}
	}
}
