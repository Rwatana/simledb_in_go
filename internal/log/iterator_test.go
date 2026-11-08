package log

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestIteratorReadsCurrentBlockNewestToOldest(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 64)
	records := [][]byte{
		[]byte("oldest"),
		{},
		{0xde, 0xad, 0x00, 0xbe, 0xef},
	}
	for _, record := range records {
		if _, err := manager.Append(record); err != nil {
			t.Fatalf("Append(%v) error = %v", record, err)
		}
	}

	iterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("Iterator() error = %v", err)
	}
	if _, err := manager.Append([]byte("after-snapshot")); err != nil {
		t.Fatalf("Append(after snapshot) error = %v", err)
	}

	want := [][]byte{records[2], records[1], records[0]}
	for index, expected := range want {
		got, ok, err := iterator.Next()
		if err != nil {
			t.Fatalf("Next() record %d error = %v", index, err)
		}
		if !ok {
			t.Fatalf("Next() record %d ok = false, want true", index)
		}
		if !bytes.Equal(got, expected) {
			t.Errorf("Next() record %d = %v, want %v", index, got, expected)
		}
	}
	if got, ok, err := iterator.Next(); err != nil || ok || got != nil {
		t.Errorf("Next() after exhaustion = (%v, %t, %v), want (nil, false, nil)", got, ok, err)
	}

	diskTail, err := files.Read(storage.NewBlockID("simpledb.log", 0))
	if err != nil {
		t.Fatalf("Read(disk tail) error = %v", err)
	}
	diskBoundary, err := diskTail.GetInt(0)
	if err != nil {
		t.Fatalf("diskTail.GetInt(0) error = %v", err)
	}
	if diskBoundary != 64 {
		t.Errorf("disk boundary after Iterator() = %d, want unchanged boundary 64", diskBoundary)
	}
}

func TestIteratorSnapshotsReturnedRecordsAndEmptyTail(t *testing.T) {
	t.Parallel()

	emptyManager, _ := newLogManager(t, 32)
	emptyIterator, err := emptyManager.Iterator()
	if err != nil {
		t.Fatalf("empty Iterator() error = %v", err)
	}
	if record, ok, err := emptyIterator.Next(); err != nil || ok || record != nil {
		t.Errorf("empty Next() = (%v, %t, %v), want (nil, false, nil)", record, ok, err)
	}

	manager, _ := newLogManager(t, 32)
	want := []byte{1, 2, 3}
	if _, err := manager.Append(want); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	firstIterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("first Iterator() error = %v", err)
	}
	secondIterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("second Iterator() error = %v", err)
	}

	first, ok, err := firstIterator.Next()
	if err != nil || !ok {
		t.Fatalf("first Next() = (%v, %t, %v), want a record", first, ok, err)
	}
	first[0] = 99
	second, ok, err := secondIterator.Next()
	if err != nil || !ok {
		t.Fatalf("second Next() = (%v, %t, %v), want a record", second, ok, err)
	}
	if !bytes.Equal(second, want) {
		t.Errorf("second iterator record = %v, want independent copy %v", second, want)
	}
}

func TestIteratorRejectsInvalidCurrentBlockBoundary(t *testing.T) {
	manager, files := newLogManager(t, 64)
	if err := manager.page.SetInt(0, 3); err != nil {
		t.Fatalf("SetInt(boundary) error = %v", err)
	}

	iterator, err := manager.Iterator()
	if iterator != nil {
		t.Errorf("Iterator() = %v, want nil when the current boundary is invalid", iterator)
	}
	if err == nil {
		t.Fatal("Iterator() error = nil, want invalid boundary error")
	}
	for _, context := range []string{manager.block.String(), "boundary 3", "[4, 64]"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Iterator() error = %q, want context %q", err, context)
		}
	}

	disk, err := files.Read(manager.block)
	if err != nil {
		t.Fatalf("Read(current block) error = %v", err)
	}
	diskBoundary, err := disk.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt(disk boundary) error = %v", err)
	}
	if diskBoundary != 64 {
		t.Errorf("disk boundary = %d, want unchanged initial boundary 64", diskBoundary)
	}
}

func TestBlockBoundaryRejectsPageWithoutCompleteInteger(t *testing.T) {
	page, err := storage.NewPage(boundarySize - 1)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	block := storage.NewBlockID("simpledb.log", 4)
	boundary, err := blockBoundary(page, block)
	if boundary != 0 {
		t.Errorf("blockBoundary() = %d, want 0 on a short page", boundary)
	}
	if err == nil {
		t.Fatal("blockBoundary() error = nil, want a short-page read error")
	}
	for _, context := range []string{block.String(), "read boundary", "get integer at offset 0"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("blockBoundary() error = %q, want context %q", err, context)
		}
	}
}

func TestIteratorRejectsPositionPastPageCapacity(t *testing.T) {
	page, err := storage.NewPage(8)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	iterator := &Iterator{
		page:     page,
		position: page.Capacity() + 1,
		block:    storage.NewBlockID("simpledb.log", 0),
	}
	record, ok, err := iterator.Next()
	if record != nil || ok {
		t.Errorf("Next() = (%v, %t, %v), want no record", record, ok, err)
	}
	if err == nil {
		t.Fatal("Next() error = nil, want invalid position error")
	}
	for _, context := range []string{"position 9", "page capacity 8"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Next() error = %q, want context %q", err, context)
		}
	}
}

func TestIteratorRejectsTruncatedRecordLengthPrefix(t *testing.T) {
	page, err := storage.NewPage(8)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	iterator := &Iterator{
		page:     page,
		position: page.Capacity() - 3,
		block:    storage.NewBlockID("simpledb.log", 0),
	}
	record, ok, err := iterator.Next()
	if record != nil || ok {
		t.Errorf("Next() = (%v, %t, %v), want no record", record, ok, err)
	}
	if err == nil {
		t.Fatal("Next() error = nil, want a truncated length-prefix error")
	}
	for _, context := range []string{"iterate log block at offset 5", "get integer at offset 5", "page capacity 8"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("Next() error = %q, want context %q", err, context)
		}
	}
}
