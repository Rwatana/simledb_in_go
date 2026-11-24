package btree

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type pageSetFlagTransaction struct {
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

func (tx *pageSetFlagTransaction) Pin(storage.BlockID) error {
	tx.pins++
	return nil
}

func (tx *pageSetFlagTransaction) Unpin(storage.BlockID) error {
	tx.unpins++
	return nil
}

func (*pageSetFlagTransaction) BlockSize() (int, error) { return 0, nil }

func (tx *pageSetFlagTransaction) GetInt(storage.BlockID, int) (int32, error) {
	tx.reads++
	return 0, nil
}

func (*pageSetFlagTransaction) GetString(storage.BlockID, int) (string, error) { return "", nil }

func (tx *pageSetFlagTransaction) SetInt(block storage.BlockID, offset, value int32, log bool) error {
	tx.writeBlock = block
	tx.writeOffset = offset
	tx.writeValue = value
	tx.writeLogged = log
	tx.writes++
	return tx.writeError
}

func (*pageSetFlagTransaction) SetString(storage.BlockID, int32, string, bool) error { return nil }

func TestPageSetFlagWritesSignedValueToLoggedHeaderField(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 7)
	for _, value := range []int32{-1, 0, 42} {
		transaction := &pageSetFlagTransaction{}
		page := &Page{transaction: transaction, block: block}
		if err := page.SetFlag(value); err != nil {
			t.Fatalf("SetFlag(%d) error = %v", value, err)
		}
		if transaction.writes != 1 || transaction.writeBlock != block || transaction.writeOffset != 0 || transaction.writeValue != value || !transaction.writeLogged {
			t.Fatalf("SetInt calls = %d at %v:%d value %d logged %t, want one logged write of %d at block offset zero", transaction.writes, transaction.writeBlock, transaction.writeOffset, transaction.writeValue, transaction.writeLogged, value)
		}
		if transaction.reads != 0 || transaction.pins != 0 || transaction.unpins != 0 || page.layout != nil {
			t.Fatal("SetFlag read data, required layout metadata, or changed page pin state")
		}
	}
}

func TestPageSetFlagRejectsNilAndClosedPageState(t *testing.T) {
	block := storage.NewBlockID("leaves.idx", 0)
	var typedNil *pageSetFlagTransaction
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
			if err := test.page.SetFlag(1); err == nil {
				t.Fatal("SetFlag() error = nil, want validation error")
			}
		})
	}
}

func TestPageSetFlagWrapsWriteFailure(t *testing.T) {
	writeError := errors.New("flag write failed")
	transaction := &pageSetFlagTransaction{writeError: writeError}
	page := &Page{transaction: transaction, block: storage.NewBlockID("leaves.idx", 3)}
	if err := page.SetFlag(-1); !errors.Is(err, writeError) {
		t.Fatalf("SetFlag() error = %v, want wrapped cause", err)
	}
}
