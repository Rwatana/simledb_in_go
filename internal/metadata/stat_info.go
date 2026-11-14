package metadata

// StatInfo stores observed table cardinality statistics for cost estimation.
type StatInfo struct {
	blocks  int
	records int
}

// NewStatInfo initializes table statistics with observed block and record counts.
func NewStatInfo(blocks, records int) *StatInfo {
	return &StatInfo{blocks: blocks, records: records}
}
