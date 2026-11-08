package log

import (
	"bytes"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestFlushPersistsTheTailThroughRequestedLSN(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 64)
	firstLSN, err := manager.Append([]byte("first"))
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	secondLSN, err := manager.Append([]byte("second"))
	if err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}

	diskBefore, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read() before flush error = %v", err)
	}
	beforeBoundary, err := diskBefore.GetInt(0)
	if err != nil {
		t.Fatalf("diskBefore.GetInt() error = %v", err)
	}
	if beforeBoundary != 64 {
		t.Fatalf("disk boundary before flush = %d, want 64", beforeBoundary)
	}

	if err := manager.Flush(firstLSN); err != nil {
		t.Fatalf("Flush(%d) error = %v", firstLSN, err)
	}
	if manager.lastSavedLSN != secondLSN {
		t.Errorf("lastSavedLSN = %d, want latest LSN %d", manager.lastSavedLSN, secondLSN)
	}

	diskAfter, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read() after flush error = %v", err)
	}
	if got, want := diskAfter.Snapshot(), manager.page.Snapshot(); !bytes.Equal(got, want) {
		t.Errorf("disk Page after flush differs from memory tail")
	}
}

func TestFlushRejectsFutureLSNAndSkipsSavedLSN(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 32)
	lsn, err := manager.Append([]byte("record"))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := manager.Flush(lsn); err != nil {
		t.Fatalf("Flush(%d) error = %v", lsn, err)
	}

	if err := manager.Flush(lsn); err != nil {
		t.Errorf("Flush(%d) saved LSN error = %v", lsn, err)
	}
	if manager.lastSavedLSN != lsn {
		t.Errorf("lastSavedLSN after no-op = %d, want %d", manager.lastSavedLSN, lsn)
	}
	if err := manager.Flush(lsn + 1); err == nil {
		t.Errorf("Flush(%d) future LSN error = nil, want an invalid LSN error", lsn+1)
	}
	if manager.lastSavedLSN != lsn {
		t.Errorf("lastSavedLSN after rejected future flush = %d, want %d", manager.lastSavedLSN, lsn)
	}
}

func TestFlushZeroOnEmptyLogIsNoOp(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 32)
	before := manager.page.Snapshot()
	if err := manager.Flush(0); err != nil {
		t.Fatalf("Flush(0) error = %v", err)
	}
	if manager.lastSavedLSN != 0 {
		t.Errorf("lastSavedLSN = %d, want 0", manager.lastSavedLSN)
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, before) {
		t.Errorf("Flush(0) changed empty tail Page")
	}
}
