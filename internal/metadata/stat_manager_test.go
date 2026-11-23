package metadata

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
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

func TestStatManagerRefreshPreservesTableCatalogLayoutFailure(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}

	manager := NewStatManager(tables)
	stale := NewStatInfo(8, 9)
	manager.stats["stale"] = stale
	manager.calls = 37
	wantErr := errors.New("table catalog size unavailable")
	transaction.sizeErr = wantErr
	transaction.sizeErrFile = "tblcat.tbl"

	err := manager.Refresh(transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Refresh() error = %v, want wrapped catalog size failure", err)
	}
	for _, context := range []string{
		"refresh table statistics",
		"load table catalog layout",
		`load layout for table "tblcat" from table catalog`,
		"open catalog scan",
		"read size",
	} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Refresh() error = %q, want context %q", err, context)
		}
	}
	if manager.calls != 37 {
		t.Errorf("lookup calls = %d, want unchanged value 37", manager.calls)
	}
	if len(manager.stats) != 1 || manager.stats["stale"] != stale {
		t.Errorf("statistics cache = %v, want prior stale snapshot retained", manager.stats)
	}
}

func TestStatManagerRefreshPreservesRegisteredTableScanFailure(t *testing.T) {
	transaction := newCatalogTransaction(t, 2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}

	manager := NewStatManager(tables)
	stale := NewStatInfo(8, 9)
	manager.stats["stale"] = stale
	manager.calls = 37
	transaction.unpins = nil

	wantErr := errors.New("registered table flag unavailable")
	transaction.getIntErr = wantErr
	transaction.getIntOffset = 76
	transaction.getIntErrFile = "tblcat.tbl"
	transaction.getIntErrAfter = 2

	err := manager.Refresh(transaction)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Refresh() error = %v, want wrapped registered-table scan failure", err)
	}
	for _, context := range []string{"refresh table statistics", "scan registered tables", "find occupied record"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Refresh() error = %q, want context %q", err, context)
		}
	}
	if len(transaction.unpins) == 0 || transaction.unpins[len(transaction.unpins)-1] != storage.NewBlockID("tblcat.tbl", 0) {
		t.Errorf("last unpin = %v, want failed registered-table scan block %v", transaction.unpins, storage.NewBlockID("tblcat.tbl", 0))
	}
	if manager.calls != 37 {
		t.Errorf("lookup calls = %d, want unchanged value 37", manager.calls)
	}
	if len(manager.stats) != 1 || manager.stats["stale"] != stale {
		t.Errorf("statistics cache = %v, want the prior stale snapshot retained", manager.stats)
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

func TestStatManagerStatisticsRefreshesOnHundredFirstLookup(t *testing.T) {
	transaction := newEmptyCatalogTransaction(2048)
	tables := NewTableManager()
	if err := tables.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	var schema record.Schema
	schema.AddIntegerField("id")
	if err := tables.CreateTable("student", &schema, transaction); err != nil {
		t.Fatalf("CreateTable() error = %v", err)
	}
	layout := record.NewLayout(&schema)
	scan, err := record.NewTableScan(transaction, "student", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	for id := 0; id < 2; id++ {
		if err := scan.Insert(); err != nil {
			t.Fatalf("Insert(%d) error = %v", id, err)
		}
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	manager := NewStatManager(tables)
	stale := NewStatInfo(99, 99)
	manager.stats["student"] = stale
	manager.calls = 100
	stats, err := manager.Statistics("student", layout, transaction)
	if err != nil {
		t.Fatalf("Statistics() error = %v", err)
	}
	if stats == stale || stats.RecordsOutput() != 2 {
		t.Fatalf("statistics = %p (%d records), want refreshed snapshot with 2 records", stats, stats.RecordsOutput())
	}
	if manager.calls != 0 {
		t.Fatalf("calls = %d, want 0 after refresh", manager.calls)
	}
	if len(manager.stats) != 3 || manager.stats["student"] != stats {
		t.Fatalf("cache = %v, want refreshed tblcat, fldcat, and student snapshots", manager.stats)
	}
}

func TestStatManagerStatisticsPreservesRefreshFailureOnHundredFirstLookup(t *testing.T) {
	manager := NewStatManager(nil)
	manager.calls = 100

	stats, err := manager.Statistics("student", nil, nil)
	if stats != nil {
		t.Fatalf("Statistics() stats = %v, want nil when refresh fails", stats)
	}
	if err == nil {
		t.Fatal("Statistics() error = nil, want refresh failure")
	}
	for _, context := range []string{"look up statistics for table \"student\"", "refresh cache", "table manager is nil"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Statistics() error = %q, want context %q", err, context)
		}
	}
	if manager.calls != 101 {
		t.Errorf("lookup calls = %d, want 101 after failed refresh", manager.calls)
	}
	if len(manager.stats) != 0 {
		t.Errorf("statistics cache = %v, want no result cached after failed refresh", manager.stats)
	}
}
