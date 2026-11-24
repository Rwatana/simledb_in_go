package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageFlagTransaction struct {
	value      int32
	readError  error
	readBlock  storage.BlockID
	readOffset int
	reads      int
	pins       int
	unpins     int
}

func (tx *pageFlagTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}
func (tx *pageFlagTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}
func (*pageFlagTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageFlagTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	tx.readBlock = block
	tx.readOffset = offset
	tx.reads++
	if tx.readError != nil {
		return 0, tx.readError
	}
	return tx.value, nil
}

func (*pageFlagTransaction) GetString(storage.BlockID, int) (string, error)   { return "", nil }
func (*pageFlagTransaction) SetInt(storage.BlockID, int32, int32, bool) error { return nil }
func (*pageFlagTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageGetFlagReadsSignedValueFromPageHeader(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 7)
	for _, value := range []int32{-1, 0, 42} {
		transaction := &pageFlagTransaction{value: value}
		page := &Page{transaction: transaction, block: block}
		got, err := page.GetFlag()
		if err != nil || got != value {
			t.Fatalf("GetFlag() = (%d, %v), want (%d, nil)", got, err, value)
		}
		if transaction.reads != 1 || transaction.readBlock != block || transaction.readOffset != 0 {
			t.Fatalf("GetInt calls = %d at %v:%d, want one read of the page flag at offset zero", transaction.reads, transaction.readBlock, transaction.readOffset)
		}
		if page.layout != nil || transaction.pins != 0 || transaction.unpins != 0 {
			t.Fatal("GetFlag required layout metadata or changed page pin state")
		}
	}
}

func TestPageGetFlagRejectsNilAndClosedPageState(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	var typedNil *pageFlagTransaction
	for _, test := range []struct {
		name string
		page *Page
	}{
		{name: "nil page"},
		{name: "closed page", page: &Page{block: storage.BlockID{}}},
		{name: "nil transaction", page: &Page{block: block}},
		{name: "typed-nil transaction", page: &Page{transaction: typedNil, block: block}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.GetFlag(); err == nil {
				t.Fatal("GetFlag() error = nil, want validation error")
			}
		})
	}
}

func TestPageGetFlagWrapsReadFailure(t *testing.T) {
	readError := errors.New("flag read failed")
	transaction := &pageFlagTransaction{readError: readError}
	page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 2)}
	if _, err := page.GetFlag(); !errors.Is(err, readError) {
		t.Fatalf("GetFlag() error = %v, want wrapped cause", err)
	}
}
