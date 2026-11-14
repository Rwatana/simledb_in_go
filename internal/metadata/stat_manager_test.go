package metadata

import (
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

func TestStatManagerRefreshRebuildsEveryRegisteredTable(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("id")
	if err := tables.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable(student) error = %v", err)
	}
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan(student) error = %v", err)
	}
	for id := 0; id < 2; id++ {
		if err := scan.Insert(); err != nil {
			t.Fatalf("Insert(%d) error = %v", id, err)
		}
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close(student) error = %v", err)
	}

	manager := NewStatManager(tables)
	manager.calls = 99
	manager.stats["stale"] = NewStatInfo(99, 99)
	if err := manager.Refresh(transaction); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if manager.calls != 0 {
		t.Fatalf("calls = %d, want 0", manager.calls)
	}
	if len(manager.stats) != 3 || manager.stats["stale"] != nil {
		t.Fatalf("cache = %v, want only tblcat, fldcat, student", manager.stats)
	}
	wantRecords := map[string]int{"tblcat": 3, "fldcat": 8, "student": 2}
	for tableName, records := range wantRecords {
		stats := manager.stats[tableName]
		if stats == nil || stats.RecordsOutput() != records {
			t.Fatalf("%s statistics = %v, want %d records", tableName, stats, records)
		}
	}
}

func TestStatManagerStatisticsCachesFirstLookup(t *testing.T) {
	transaction := newEmptyCatalogTransaction(256)
	var schema record.Schema
	schema.AddIntegerField("id")
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	manager := NewStatManager(nil)
	first, err := manager.Statistics("student", layout, transaction)
	if err != nil {
		t.Fatalf("Statistics(first) error = %v", err)
	}
	if first.RecordsOutput() != 1 {
		t.Fatalf("first records = %d, want 1", first.RecordsOutput())
	}

	scan, err = record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan(second) error = %v", err)
	}
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert(second) error = %v", err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close(second) error = %v", err)
	}
	second, err := manager.Statistics("student", layout, transaction)
	if err != nil {
		t.Fatalf("Statistics(second) error = %v", err)
	}
	if second != first || second.RecordsOutput() != 1 {
		t.Fatalf("second statistics = %p (%d records), want cached %p (1 record)", second, second.RecordsOutput(), first)
	}
	if manager.calls != 2 || manager.stats["student"] != first {
		t.Fatalf("manager state = (%d calls, %p cached), want (2 calls, %p cached)", manager.calls, manager.stats["student"], first)
	}
}
