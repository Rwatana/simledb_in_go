package metadata

import (
	"errors"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"load table catalog layout"
	"open catalog scan"
	"read size"
	"refresh table statistics"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewStatManagerStartsWithEmptyCache(t *testing.T) {
	tables := NewTableManager()
	manager := NewStatManager(tables)
	if manager == nil {
		t.Fatal("NewStatManager() returned nil")
	}
	if manager.tables != tables {
		t.Fatalf("table manager = %p, want %p", manager.tables, tables)
	}
	if manager.calls != 0 {
		t.Fatalf("calls = %d, want 0", manager.calls)
	}
	if manager.stats == nil || len(manager.stats) != 0 {
		t.Fatalf("statistics cache = %v, want empty initialized map", manager.stats)
	}
}

func TestStatManagerCalculateTableStatsCountsRecordsAndBlocks(t *testing.T) {
	transaction := newEmptyCatalogTransaction(64)
	var schema record.Schema
	schema.AddIntegerField("id")
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	for id := 0; id < 9; id++ {
		if err := scan.Insert(); err != nil {
			t.Fatalf("Insert(%d) error = %v", id, err)
		}
		if err := scan.SetInt("id", int32(id)); err != nil {
			t.Fatalf("SetInt(%d) error = %v", id, err)
		}
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	transaction.unpins = nil

	manager := NewStatManager(nil)
	stats, err := manager.calculateTableStats("student", layout, transaction)
	if err != nil {
		t.Fatalf("calculateTableStats() error = %v", err)
	}
	if stats.BlocksAccessed() != 2 || stats.RecordsOutput() != 9 {
		t.Fatalf("statistics = (%d blocks, %d records), want (2, 9)", stats.BlocksAccessed(), stats.RecordsOutput())
	}
	if manager.calls != 0 || len(manager.stats) != 0 {
		t.Fatalf("manager state = (%d calls, %v), want unchanged", manager.calls, manager.stats)
	}
}

func TestStatManagerCalculateTableStatsPreservesScanFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(64)
	var schema record.Schema
	schema.AddIntegerField("id")
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	transaction.unpins = nil

	wantErr := errors.New("record flag unavailable")
	transaction.getIntErr = wantErr
	transaction.getIntOffset = 0
	transaction.getIntErrFile = "student.tbl"

	manager := NewStatManager(nil)
	stats, err := manager.calculateTableStats("student", layout, transaction)
	if stats != nil {
		t.Fatalf("calculateTableStats() stats = %v, want nil after scan failure", stats)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("calculateTableStats() error = %v, want wrapped scan failure", err)
	}
	for _, context := range []string{
		`calculate statistics for table "student"`,
		"scan records",
		"find occupied record",
	} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("calculateTableStats() error = %q, want context %q", err, context)
		}
	}
	wantBlock := storage.NewBlockID("student.tbl", 0)
	if len(transaction.unpins) != 1 || transaction.unpins[0] != wantBlock {
		t.Errorf("unpins = %v, want [%v] after scan failure", transaction.unpins, wantBlock)
	}
	if manager.calls != 0 || len(manager.stats) != 0 {
		t.Errorf("manager state = (%d calls, %v), want unchanged", manager.calls, manager.stats)
	}
}

func TestScanTableStatisticsPreservesRecordIdentifierFailure(t *testing.T) {
	wantErr := errors.New("record identifier unavailable")
	blocks, records, err := scanTableStatistics(&failingRIDStatisticsScan{err: wantErr})
	if blocks != 0 || records != 1 {
		t.Errorf("scanTableStatistics() counts = (%d blocks, %d records), want (0, 1)", blocks, records)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("scanTableStatistics() error = %v, want wrapped RID failure", err)
	}
	if !strings.Contains(err.Error(), "read RID for record 1") {
		t.Errorf("scanTableStatistics() error = %q, want record-index context", err)
	}
}
