package recovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type sliceEncodedRecordIterator struct {
	records [][]byte
	next    int
	err     error
}

func (i *sliceEncodedRecordIterator) Next() ([]byte, bool, error) {
	if i.err != nil {
		return nil, false, i.err
	}
	if i.next == len(i.records) {
		return nil, false, nil
	}
	record := i.records[i.next]
	i.next++
	return record, true, nil
}

func TestNextTransactionRecordSkipsUnownedAndOtherTransactions(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	other, err := EncodeSetIntRecord(8, block, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	target, err := EncodeSetStringRecord(7, block, 16, "以前")
	if err != nil {
		t.Fatal(err)
	}
	iterator := &sliceEncodedRecordIterator{records: [][]byte{
		EncodeCheckpointRecord(), other, target, EncodeStartRecord(7),
	}}

	record, ok, err := NextTransactionRecord(iterator, 7)
	if err != nil || !ok {
		t.Fatalf("first NextTransactionRecord() = (%T, %t, %v)", record, ok, err)
	}
	update, ok := record.(SetStringRecord)
	if !ok || update.OldValue() != "以前" {
		t.Fatalf("first record = %#v", record)
	}
	record, ok, err = NextTransactionRecord(iterator, 7)
	if err != nil || !ok || record.Operation() != OperationStart {
		t.Fatalf("second NextTransactionRecord() = (%T, %t, %v)", record, ok, err)
	}
	record, ok, err = NextTransactionRecord(iterator, 7)
	if err != nil || ok || record != nil {
		t.Fatalf("exhausted NextTransactionRecord() = (%T, %t, %v)", record, ok, err)
	}
}

func TestNextTransactionRecordReturnsIteratorFailure(t *testing.T) {
	want := errors.New("next failed")
	_, _, err := NextTransactionRecord(&sliceEncodedRecordIterator{err: want}, 7)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "transaction 7") {
		t.Fatalf("error = %v", err)
	}
}

func TestNextTransactionRecordReturnsDecodeFailure(t *testing.T) {
	_, _, err := NextTransactionRecord(&sliceEncodedRecordIterator{records: [][]byte{{0}}}, 7)
	if err == nil || !strings.Contains(err.Error(), "decode") || !strings.Contains(err.Error(), "transaction 7") {
		t.Fatalf("error = %v", err)
	}
}
