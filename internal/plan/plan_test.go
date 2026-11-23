package plan

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/metadata"
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
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

func TestTablePlanSchemaReturnsResolvedTableSchema(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("student_id")
	schema.AddVarcharField("student_name", 24)
	layout := record.NewLayout(&schema)

	tablePlan := NewTablePlan(nil, "student", layout, nil)
	if got := tablePlan.Schema(); got != &schema {
		t.Fatalf("Schema() = %p, want the schema stored in the layout %p", got, &schema)
	}
}

func TestTablePlanSchemaHandlesNilPlanAndLayout(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.Schema(); got != nil {
		t.Fatalf("nil TablePlan.Schema() = %p, want nil", got)
	}

	if got := NewTablePlan(nil, "student", nil, nil).Schema(); got != nil {
		t.Fatalf("TablePlan with nil layout Schema() = %p, want nil", got)
	}
}

func TestTablePlanOpenCreatesTableScan(t *testing.T) {
	transaction := &tablePlanTransaction{size: 1}
	var schema record.Schema
	schema.AddIntegerField("student_id")
	tablePlan := NewTablePlan(transaction, "student", record.NewLayout(&schema), nil)

	scan, err := tablePlan.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := scan.(*query.RecordScan); !ok {
		t.Fatalf("Open() scan has type %T, want *query.RecordScan", scan)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("first Next() = (%t, %v), want (true, nil) from the first occupied slot", next, err)
	}
	if got, err := scan.GetInt("student_id"); err != nil || got != 42 {
		t.Fatalf("first row student_id = (%d, %v), want (42, nil)", got, err)
	}
	if got, want := transaction.pins, []storage.BlockID{storage.NewBlockID("student.tbl", 0)}; !equalPlanBlocks(got, want) {
		t.Fatalf("pinned blocks = %v, want %v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got, want := transaction.unpins, []storage.BlockID{storage.NewBlockID("student.tbl", 0)}; !equalPlanBlocks(got, want) {
		t.Fatalf("unpinned blocks = %v, want %v", got, want)
	}
}

func TestTablePlanOpenWrapsScanInitializationError(t *testing.T) {
	openErr := errors.New("file size unavailable")
	transaction := &tablePlanTransaction{sizeErr: openErr}
	var schema record.Schema
	schema.AddIntegerField("student_id")

	scan, err := NewTablePlan(transaction, "student", record.NewLayout(&schema), nil).Open()
	if scan != nil || !errors.Is(err, openErr) {
		t.Fatalf("Open() = (%v, %v), want (nil, wrapped file-size error)", scan, err)
	}
	if !strings.Contains(err.Error(), "student") {
		t.Fatalf("Open() error = %q, want table context", err)
	}
}

func TestTablePlanOpenRejectsNilPlanAndDependencies(t *testing.T) {
	var nilPlan *TablePlan
	if scan, err := nilPlan.Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "nil plan") {
		t.Fatalf("nil plan Open() = (%v, %v), want nil scan and nil-plan error", scan, err)
	}

	var schema record.Schema
	layout := record.NewLayout(&schema)
	if scan, err := NewTablePlan(nil, "student", layout, nil).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Fatalf("plan with nil transaction Open() = (%v, %v), want transaction error", scan, err)
	}
	if scan, err := NewTablePlan(&tablePlanTransaction{size: 1}, "student", nil, nil).Open(); scan != nil || err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("plan with nil layout Open() = (%v, %v), want layout error", scan, err)
	}
}

func TestTablePlanBlocksAccessedReturnsStatistic(t *testing.T) {
	stats := metadata.NewStatInfo(12, 30)
	var schema record.Schema
	tablePlan := NewTablePlan(nil, "student", record.NewLayout(&schema), stats)
	if got := tablePlan.BlocksAccessed(); got != 12 {
		t.Fatalf("BlocksAccessed() = %d, want 12", got)
	}
}

func TestTablePlanBlocksAccessedHandlesNilPlanAndStatistics(t *testing.T) {
	var nilPlan *TablePlan
	if got := nilPlan.BlocksAccessed(); got != 0 {
		t.Fatalf("nil TablePlan.BlocksAccessed() = %d, want 0", got)
	}
	if got := NewTablePlan(nil, "student", nil, nil).BlocksAccessed(); got != 0 {
		t.Fatalf("TablePlan with nil statistics BlocksAccessed() = %d, want 0", got)
	}
}

type tablePlanTransaction struct {
	size    int
	sizeErr error
	pins    []storage.BlockID
	unpins  []storage.BlockID
}

func (t *tablePlanTransaction) Pin(block storage.BlockID) error {
	t.pins = append(t.pins, block)
	return nil
}

func (*tablePlanTransaction) BlockSize() (int, error) { return 512, nil }
func (*tablePlanTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	if offset == 0 {
		return 1, nil
	}
	return 42, nil
}
func (*tablePlanTransaction) GetString(storage.BlockID, int) (string, error)       { return "", nil }
func (*tablePlanTransaction) SetInt(storage.BlockID, int32, int32, bool) error     { return nil }
func (*tablePlanTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func (t *tablePlanTransaction) Size(string) (int, error) { return t.size, t.sizeErr }

func (*tablePlanTransaction) Append(fileName string) (storage.BlockID, error) {
	return storage.NewBlockID(fileName, 0), nil
}

func (t *tablePlanTransaction) Unpin(block storage.BlockID) error {
	t.unpins = append(t.unpins, block)
	return nil
}

func equalPlanBlocks(got, want []storage.BlockID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
