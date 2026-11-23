// Package plan defines query execution plans.
package plan

import (
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// Plan describes the scan, output estimates, and schema of a relational query tree.
type Plan interface {
	// Open constructs a scan for this plan, positioned before its first record.
	Open() (query.Scan, error)

	// BlocksAccessed estimates the block accesses needed to read the scan to completion.
	BlocksAccessed() int

	// RecordsOutput estimates the number of records produced by the scan.
	RecordsOutput() int

	// DistinctValues estimates the number of distinct output values for fieldName.
	DistinctValues(fieldName string) int

	// Schema returns the schema of the plan's output table.
	Schema() *record.Schema
}
