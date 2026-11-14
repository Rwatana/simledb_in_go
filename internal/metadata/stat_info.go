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

// BlocksAccessed returns the estimated number of table blocks accessed.
func (s *StatInfo) BlocksAccessed() int {
	return s.blocks
}

// RecordsOutput returns the estimated number of table records produced.
func (s *StatInfo) RecordsOutput() int {
	return s.records
}
