package log

import (
	"bytes"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestAppendRollsOverAFullTailBlock(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 20)
	first := []byte("first-record")
	firstLSN, err := manager.Append(first)
	if err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}

	second := []byte("next")
	secondLSN, err := manager.Append(second)
	if err != nil {
		t.Fatalf("Append(second) rollover error = %v", err)
	}
	if firstLSN != 1 || secondLSN != 2 {
		t.Errorf("Append() LSNs = (%d, %d), want (1, 2)", firstLSN, secondLSN)
	}
	if manager.block != storage.NewBlockID("simpledb.log", 1) {
		t.Errorf("current block = %v, want simpledb.log block 1", manager.block)
	}
	if manager.lastSavedLSN != firstLSN {
		t.Errorf("lastSavedLSN = %d, want rolled-over LSN %d", manager.lastSavedLSN, firstLSN)
	}

	length, err := files.Length("simpledb.log")
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if length != 2 {
		t.Fatalf("Length() = %d, want 2 blocks", length)
	}

	oldTail, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(old tail) error = %v", err)
	}
	oldBoundary, err := oldTail.GetInt(0)
	if err != nil {
		t.Fatalf("oldTail.GetInt(0) error = %v", err)
	}
	if oldBoundary != 4 {
		t.Fatalf("old tail boundary = %d, want 4", oldBoundary)
	}
	oldRecord, err := oldTail.GetBytes(int(oldBoundary))
	if err != nil {
		t.Fatalf("oldTail.GetBytes() error = %v", err)
	}
	if !bytes.Equal(oldRecord, first) {
		t.Errorf("old tail record = %q, want %q", oldRecord, first)
	}

	newBoundary, err := manager.page.GetInt(0)
	if err != nil {
		t.Fatalf("memory new tail GetInt(0) error = %v", err)
	}
	if newBoundary != 12 {
		t.Fatalf("memory new tail boundary = %d, want 12", newBoundary)
	}
	newRecord, err := manager.page.GetBytes(int(newBoundary))
	if err != nil {
		t.Fatalf("memory new tail GetBytes() error = %v", err)
	}
	if !bytes.Equal(newRecord, second) {
		t.Errorf("memory new tail record = %q, want %q", newRecord, second)
	}

	diskNewTail, err := files.Read(storage.NewBlockID("simpledb.log", 1))
	if err != nil {
		t.Fatalf("Read(new tail) error = %v", err)
	}
	diskBoundary, err := diskNewTail.GetInt(0)
	if err != nil {
		t.Fatalf("disk new tail GetInt(0) error = %v", err)
	}
	if diskBoundary != 20 {
		t.Errorf("disk new tail boundary = %d, want initialized boundary 20 before explicit flush", diskBoundary)
	}
}

func TestAppendRejectsARecordThatCannotFitAnEmptyBlockBeforeRollover(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 20)
	if _, err := manager.Append([]byte("pending")); err != nil {
		t.Fatalf("Append(pending) error = %v", err)
	}
	beforePage := manager.page.Snapshot()
	beforeBlock := manager.block
	beforeLSN := manager.latestLSN

	if _, err := manager.Append(make([]byte, 13)); err == nil {
		t.Fatal("Append(oversized) error = nil, want an empty-block capacity error")
	}

	length, err := files.Length("simpledb.log")
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if length != 1 {
		t.Errorf("Length() after rejected append = %d, want 1", length)
	}
	if manager.block != beforeBlock {
		t.Errorf("current block after rejected append = %v, want %v", manager.block, beforeBlock)
	}
	if manager.latestLSN != beforeLSN {
		t.Errorf("latestLSN after rejected append = %d, want %d", manager.latestLSN, beforeLSN)
	}
	if got := manager.page.Snapshot(); !bytes.Equal(got, beforePage) {
		t.Errorf("tail Page changed after rejected append: got %v, want %v", got, beforePage)
	}

	diskTail, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(tail) error = %v", err)
	}
	diskBoundary, err := diskTail.GetInt(0)
	if err != nil {
		t.Fatalf("diskTail.GetInt(0) error = %v", err)
	}
	if diskBoundary != 20 {
		t.Errorf("disk boundary after rejected append = %d, want unchanged boundary 20", diskBoundary)
	}
}
