package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestIteratorReadsCurrentBlockNewestToOldest(t *testing.T) {
	t.Parallel()

	manager, _ := newLogManager(t, 64)
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
