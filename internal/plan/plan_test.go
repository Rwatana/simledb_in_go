package plan

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

var _ Plan = contractProbe{}

type contractProbe struct{}

func (contractProbe) Open() (query.Scan, error) { return nil, nil }
func (contractProbe) BlocksAccessed() int       { return 2 }
func (contractProbe) RecordsOutput() int        { return 3 }
func (contractProbe) DistinctValues(string) int { return 1 }
func (contractProbe) Schema() *record.Schema    { return nil }

func TestPlanContractExposesScanEstimatesAndSchema(t *testing.T) {
	var plan Plan = contractProbe{}
	if scan, err := plan.Open(); scan != nil || err != nil {
		t.Fatalf("Open() = (%v, %v), want (nil, nil)", scan, err)
	}
	if plan.BlocksAccessed() != 2 {
		t.Errorf("BlocksAccessed() = %d, want 2", plan.BlocksAccessed())
	}
	if plan.RecordsOutput() != 3 {
		t.Errorf("RecordsOutput() = %d, want 3", plan.RecordsOutput())
	}
	if plan.DistinctValues("student_id") != 1 {
		t.Errorf("DistinctValues() = %d, want 1", plan.DistinctValues("student_id"))
	}
	if plan.Schema() != nil {
		t.Fatal("Schema() = non-nil, want nil probe schema")
	}
}
