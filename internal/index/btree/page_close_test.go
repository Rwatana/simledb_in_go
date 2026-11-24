package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageCloseTransaction struct {
	pins       []storage.BlockID
	unpins     []storage.BlockID
	unpinError error
}

func (tx *pageCloseTransaction) Pin(block storage.BlockID) error {
	tx.pins = append(tx.pins, block)
	return nil
}

func (tx *pageCloseTransaction) Unpin(block storage.BlockID) error {
	tx.unpins = append(tx.unpins, block)
	return tx.unpinError
}

func (*pageCloseTransaction) BlockSize() (int, error) { return 0, nil }
func (*pageCloseTransaction) GetInt(storage.BlockID, int) (int32, error) {
	return 0, nil
}
func (*pageCloseTransaction) GetString(storage.BlockID, int) (string, error) {
	return "", nil
}
func (*pageCloseTransaction) SetInt(storage.BlockID, int32, int32, bool) error {
	return nil
}
func (*pageCloseTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageCloseUnpinsAndClearsBlockOnce(t *testing.T) {
	transaction := &pageCloseTransaction{}
	block := storage.NewBlockID("students.idx", 5)
	var schema record.Schema
	schema.AddIntegerField("dataval")
	layout := record.NewLayout(&schema)
	page, err := NewPage(transaction, block, layout)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if err := page.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if len(transaction.unpins) != 1 || transaction.unpins[0] != block {
		t.Fatalf("Unpin calls = %v, want [%v]", transaction.unpins, block)
	}
	if page.block.FileName() != "" || page.transaction != transaction || page.layout != layout {
		t.Fatalf("page after Close() = block %v, transaction %v, layout %p; want cleared block and retained dependencies", page.block, page.transaction, page.layout)
	}
	if _, err := page.FindSlotBefore(query.NewIntConstant(1)); err == nil {
		t.Fatal("FindSlotBefore() after Close() error = nil, want closed-page error")
	}
	if err := page.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if len(transaction.unpins) != 1 {
		t.Fatalf("Unpin calls after repeated close = %v, want one", transaction.unpins)
	}
}

func TestPageCloseHandlesMissingState(t *testing.T) {
	var nilPage *Page
	if err := nilPage.Close(); err == nil {
		t.Fatal("nil page Close() error = nil, want error")
	}
	emptyTransaction := &pageCloseTransaction{}
	if err := (&Page{transaction: emptyTransaction}).Close(); err != nil {
		t.Fatalf("Close() on a page without a block error = %v, want nil", err)
	}
	if len(emptyTransaction.unpins) != 0 {
		t.Fatalf("Unpin calls without a block = %v, want none", emptyTransaction.unpins)
	}

	activeWithoutTransaction := &Page{block: storage.NewBlockID("students.idx", 0)}
	if err := activeWithoutTransaction.Close(); err == nil {
		t.Fatal("Close() with active block and nil transaction error = nil, want error")
	}
	var typedNil *pageCloseTransaction
	activeWithTypedNil := &Page{transaction: typedNil, block: storage.NewBlockID("students.idx", 0)}
	if err := activeWithTypedNil.Close(); err == nil {
		t.Fatal("Close() with active block and typed-nil transaction error = nil, want error")
	}
}

func TestPageClosePreservesPinAfterUnpinFailure(t *testing.T) {
	unpinError := errors.New("unpin failed")
	transaction := &pageCloseTransaction{unpinError: unpinError}
	block := storage.NewBlockID("students.idx", 2)
	var schema record.Schema
	schema.AddIntegerField("dataval")
	page, err := NewPage(transaction, block, record.NewLayout(&schema))
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if err := page.Close(); !errors.Is(err, unpinError) {
		t.Fatalf("Close() error = %v, want wrapped unpin error", err)
	}
	if page.block != block {
		t.Fatalf("block after failed close = %v, want retained %v", page.block, block)
	}
	transaction.unpinError = nil
	if err := page.Close(); err != nil {
		t.Fatalf("Close() retry error = %v", err)
	}
	if len(transaction.unpins) != 2 || transaction.unpins[0] != block || transaction.unpins[1] != block || page.block.FileName() != "" {
		t.Fatalf("retry state = unpins %v, block %v; want two unpins then cleared block", transaction.unpins, page.block)
	}
}
