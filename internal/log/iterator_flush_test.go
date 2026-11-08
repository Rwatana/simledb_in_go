package log

import (
	"bytes"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestIteratorFlushesTheCurrentTailBeforeSnapshot(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 64)
	if _, err := manager.Append([]byte("first")); err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	latestLSN, err := manager.Append([]byte("latest"))
	if err != nil {
		t.Fatalf("Append(latest) error = %v", err)
	}

	diskBefore, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(before Iterator) error = %v", err)
	}
	beforeBoundary, err := diskBefore.GetInt(0)
	if err != nil {
		t.Fatalf("diskBefore.GetInt(0) error = %v", err)
	}
	if beforeBoundary != 64 {
		t.Fatalf("disk boundary before Iterator() = %d, want 64", beforeBoundary)
	}

	iterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("Iterator() error = %v", err)
	}
	if iterator == nil {
		t.Fatal("Iterator() = nil, want an iterator")
	}
	if manager.lastSavedLSN != latestLSN {
		t.Errorf("lastSavedLSN = %d, want latest LSN %d", manager.lastSavedLSN, latestLSN)
	}

	diskAfter, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(after Iterator) error = %v", err)
	}
	if got, want := diskAfter.Snapshot(), manager.page.Snapshot(); !bytes.Equal(got, want) {
		t.Error("disk current block after Iterator() differs from the iterator source Page")
	}
}
