package buffer

import (
	"bytes"
	"path/filepath"
	"testing"

	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestAssignToBlockLoadsTheFirstBlockIntoAnUnassignedFrame(t *testing.T) {
	t.Parallel()

	frame, files := newBufferWithFiles(t)
	block, page := appendDataBlock(t, files, 41)

	if err := frame.assignToBlock(block); err != nil {
		t.Fatalf("assignToBlock() error = %v", err)
	}
	assigned, ok := frame.Block()
	if !ok || assigned != block {
		t.Errorf("Block() = (%v, %t), want (%v, true)", assigned, ok, block)
	}
	if got, want := frame.Contents().Snapshot(), page.Snapshot(); !bytes.Equal(got, want) {
		t.Errorf("Contents() = %v, want disk Page %v", got, want)
	}
	if frame.PinCount() != 0 {
		t.Errorf("PinCount() after assignment = %d, want 0", frame.PinCount())
	}
}

func TestAssignToBlockRejectsAnAlreadyAssignedDirtyOrPinnedFrame(t *testing.T) {
	t.Parallel()

	t.Run("already assigned", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		first, firstPage := appendDataBlock(t, files, 1)
		second, _ := appendDataBlock(t, files, 2)
		if err := frame.assignToBlock(first); err != nil {
			t.Fatalf("first assignToBlock() error = %v", err)
		}
		if err := frame.assignToBlock(second); err == nil {
			t.Fatal("second assignToBlock() error = nil, want an already-assigned error")
		}
		if assigned, ok := frame.Block(); !ok || assigned != first {
			t.Errorf("Block() after rejected reassignment = (%v, %t), want (%v, true)", assigned, ok, first)
		}
		if got := frame.Contents().Snapshot(); !bytes.Equal(got, firstPage.Snapshot()) {
			t.Error("Contents() changed after rejected reassignment")
		}
	})

	t.Run("dirty", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		block, _ := appendDataBlock(t, files, 3)
		frame.MarkModified(TransactionID(7))
		if err := frame.assignToBlock(block); err == nil {
			t.Fatal("assignToBlock() dirty frame error = nil, want a dirty-state error")
		}
		if _, assigned := frame.Block(); assigned {
			t.Error("dirty frame became assigned after rejected assignment")
		}
	})

	t.Run("pinned", func(t *testing.T) {
		frame, files := newBufferWithFiles(t)
		block, _ := appendDataBlock(t, files, 4)
		frame.pin()
		if err := frame.assignToBlock(block); err == nil {
			t.Fatal("assignToBlock() pinned frame error = nil, want a pinned-state error")
		}
		if _, assigned := frame.Block(); assigned {
			t.Error("pinned frame became assigned after rejected assignment")
		}
	})
}

func TestAssignToBlockLeavesStateUnchangedWhenReadFails(t *testing.T) {
	t.Parallel()

	frame, _ := newBufferWithFiles(t)
	before := frame.Contents().Snapshot()
	missing := storage.NewBlockID("missing.tbl", 0)
	if err := frame.assignToBlock(missing); err == nil {
		t.Fatal("assignToBlock(missing) error = nil, want a read error")
	}
	if _, assigned := frame.Block(); assigned {
		t.Error("Block() assigned after failed read")
	}
	if got := frame.Contents().Snapshot(); !bytes.Equal(got, before) {
		t.Error("Contents() changed after failed read")
	}
}

func newBufferWithFiles(t *testing.T) (*Buffer, *storagefile.Manager) {
	t.Helper()

	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 32)
	if err != nil {
		t.Fatalf("file.NewManager() error = %v", err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatalf("log.NewManager() error = %v", err)
	}
	frame, err := New(files, logs)
	if err != nil {
		t.Fatalf("buffer.New() error = %v", err)
	}
	return frame, files
}

func appendDataBlock(t *testing.T, files *storagefile.Manager, value int32) (storage.BlockID, *storage.Page) {
	t.Helper()

	block, err := files.Append("data.tbl")
	if err != nil {
		t.Fatalf("Append(data.tbl) error = %v", err)
	}
	page, err := storage.NewPage(files.BlockSize())
	if err != nil {
		t.Fatalf("storage.NewPage() error = %v", err)
	}
	if err := page.SetInt(0, value); err != nil {
		t.Fatalf("page.SetInt() error = %v", err)
	}
	if err := files.Write(block, page); err != nil {
		t.Fatalf("Write(data block) error = %v", err)
	}
	return block, page
}
