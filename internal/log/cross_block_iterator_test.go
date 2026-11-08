package log

import (
	"bytes"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestIteratorTraversesPreviousBlocksNewestToOldest(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 20)
	records := [][]byte{
		[]byte("record-one!"),
		[]byte("two!"),
		[]byte("tri!"),
		[]byte("four"),
	}
	for _, record := range records {
		if _, err := manager.Append(record); err != nil {
			t.Fatalf("Append(%q) error = %v", record, err)
		}
	}

	length, err := files.Length("simpledb.log")
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if length != 3 {
		t.Fatalf("Length() = %d, want 3 blocks", length)
	}

	iterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("Iterator() error = %v", err)
	}
	want := [][]byte{records[3], records[2], records[1], records[0]}
	for index, expected := range want {
		got, ok, err := iterator.Next()
		if err != nil {
			t.Fatalf("Next() record %d error = %v", index, err)
		}
		if !ok {
			t.Fatalf("Next() record %d ok = false, want true", index)
		}
		if !bytes.Equal(got, expected) {
			t.Errorf("Next() record %d = %q, want %q", index, got, expected)
		}
	}
	if got, ok, err := iterator.Next(); err != nil || ok || got != nil {
		t.Errorf("Next() after block 0 = (%v, %t, %v), want (nil, false, nil)", got, ok, err)
	}

	diskCurrent, err := files.Read(storage.NewBlockID("simpledb.log", 2))
	if err != nil {
		t.Fatalf("Read(current disk block) error = %v", err)
	}
	diskBoundary, err := diskCurrent.GetInt(0)
	if err != nil {
		t.Fatalf("diskCurrent.GetInt(0) error = %v", err)
	}
	if diskBoundary != 20 {
		t.Errorf("current disk boundary after traversal = %d, want unchanged boundary 20", diskBoundary)
	}
}

func TestIteratorReportsAnInvalidPreviousBlockBoundary(t *testing.T) {
	t.Parallel()

	manager, files := newLogManager(t, 20)
	if _, err := manager.Append([]byte("record-one!")); err != nil {
		t.Fatalf("Append(first) error = %v", err)
	}
	if _, err := manager.Append([]byte("next")); err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}

	corrupt, err := storage.NewPage(20)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if err := corrupt.SetInt(0, 0); err != nil {
		t.Fatalf("SetInt(0) error = %v", err)
	}
	if err := files.Write(storage.NewBlockID("simpledb.log", 0), corrupt); err != nil {
		t.Fatalf("Write(corrupt previous block) error = %v", err)
	}

	iterator, err := manager.Iterator()
	if err != nil {
		t.Fatalf("Iterator() error = %v", err)
	}
	if _, ok, err := iterator.Next(); err != nil || !ok {
		t.Fatalf("Next(current block) = (_, %t, %v), want a record", ok, err)
	}
	if record, ok, err := iterator.Next(); err == nil || ok || record != nil {
		t.Errorf("Next(corrupt previous block) = (%v, %t, %v), want (nil, false, boundary error)", record, ok, err)
	}
}
