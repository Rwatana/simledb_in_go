package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageNumRecsTransaction struct {
	value      int32
	readError  error
	readBlock  storage.BlockID
	readOffset int
	reads      int
	pins       int
	unpins     int
}

func (tx *pageNumRecsTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageNumRecsTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageNumRecsTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageNumRecsTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	tx.readBlock = block
	tx.readOffset = offset
	tx.reads++
	if tx.readError != nil {
		return 0, tx.readError
	}
	return tx.value, nil
}

func (*pageNumRecsTransaction) GetString(storage.BlockID, int) (string, error) { return "", nil }
func (*pageNumRecsTransaction) SetInt(storage.BlockID, int32, int32, bool) error {
	return nil
}
func (*pageNumRecsTransaction) SetString(storage.BlockID, int32, string, bool) error {
	return nil
}

func TestPageGetNumRecsReadsSignedCountFromHeader(t *testing.T) {
	block := storage.NewBlockID("students.idx", 7)
	for _, count := range []int32{-1, 0, 42} {
		transaction := &pageNumRecsTransaction{value: count}
		page := &Page{transaction: transaction, block: block}
		got, err := page.GetNumRecs()
		if err != nil || got != count {
			t.Fatalf("GetNumRecs() = (%d, %v), want (%d, nil)", got, err, count)
		}
		if transaction.reads != 1 || transaction.readBlock != block || transaction.readOffset != 4 {
			t.Fatalf("GetInt calls = %d at %v:%d, want one count read at offset four", transaction.reads, transaction.readBlock, transaction.readOffset)
		}
		if page.layout != nil || transaction.pins != 0 || transaction.unpins != 0 {
			t.Fatal("GetNumRecs required layout metadata or changed page pin state")
		}
	}
}

func TestPageGetNumRecsRejectsNilAndClosedPageState(t *testing.T) {
	block := storage.NewBlockID("students.idx", 0)
	var typedNil *pageNumRecsTransaction
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
			if _, err := test.page.GetNumRecs(); err == nil {
				t.Fatal("GetNumRecs() error = nil, want validation error")
			}
		})
	}
}

func TestPageGetNumRecsWrapsReadFailure(t *testing.T) {
	readError := errors.New("record count read failed")
	transaction := &pageNumRecsTransaction{readError: readError}
	page := &Page{transaction: transaction, block: storage.NewBlockID("students.idx", 3)}
	if _, err := page.GetNumRecs(); !errors.Is(err, readError) {
		t.Fatalf("GetNumRecs() error = %v, want wrapped cause", err)
	}
}
