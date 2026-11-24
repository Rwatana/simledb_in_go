package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSetNumRecsTransaction struct {
	writeError  error
	writeBlock  storage.BlockID
	writeOffset int32
	writeValue  int32
	writeLogged bool
	writes      int
	reads       int
	pins        int
	unpins      int
}

func (tx *pageSetNumRecsTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageSetNumRecsTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageSetNumRecsTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageSetNumRecsTransaction) GetInt(storage.BlockID, int) (int32, error) {
	tx.reads++
	return 0, nil
}

func (*pageSetNumRecsTransaction) GetString(storage.BlockID, int) (string, error) { return "", nil }

func (tx *pageSetNumRecsTransaction) SetInt(block storage.BlockID, offset, value int32, log bool) error {
	tx.writeBlock = block
	tx.writeOffset = offset
	tx.writeValue = value
	tx.writeLogged = log
	tx.writes++
	return tx.writeError
}

func (*pageSetNumRecsTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func TestPageSetNumRecsWritesSignedCountToLoggedHeaderField(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 7)
	for _, count := range []int32{-1, 0, 42} {
		transaction := &pageSetNumRecsTransaction{}
		page := &Page{transaction: transaction, block: block}
		if err := page.setNumRecs(count); err != nil {
			t.Fatalf("setNumRecs(%d) error = %v", count, err)
		}
		if transaction.writes != 1 || transaction.writeBlock != block || transaction.writeOffset != 4 || transaction.writeValue != count || !transaction.writeLogged {
			t.Fatalf("SetInt calls = %d at %v:%d value %d logged %t, want one logged write of %d at block offset four", transaction.writes, transaction.writeBlock, transaction.writeOffset, transaction.writeValue, transaction.writeLogged, count)
		}
		if transaction.reads != 0 || transaction.pins != 0 || transaction.unpins != 0 || page.layout != nil {
			t.Fatal("setNumRecs read page data, required layout metadata, or changed pin state")
		}
	}
}

func TestPageSetNumRecsRejectsNilAndClosedPageState(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	var typedNil *pageSetNumRecsTransaction
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
			if err := test.page.setNumRecs(1); err == nil {
				t.Fatal("setNumRecs() error = nil, want validation error")
			}
		})
	}
}

func TestPageSetNumRecsWrapsWriteFailure(t *testing.T) {
	writeError := errors.New("record count write failed")
	transaction := &pageSetNumRecsTransaction{writeError: writeError}
	page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 3)}
	if err := page.setNumRecs(-1); !errors.Is(err, writeError) {
		t.Fatalf("setNumRecs() error = %v, want wrapped cause", err)
	}
}
