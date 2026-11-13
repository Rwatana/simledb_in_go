package transaction

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
)

type recordingTransactionLog struct {
	records [][]byte
	err     error
}

func (l *recordingTransactionLog) Append(record []byte) (logmanager.LSN, error) {
	if l.err != nil {
		return 0, l.err
	}
	l.records = append(l.records, append([]byte(nil), record...))
	return logmanager.LSN(len(l.records)), nil
}

func TestNewInitializesIsolatedTransactionLocalState(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	buffers := &recordingBufferPinner{}
	logs := &recordingTransactionLog{}

	first, err := New(&allocator, &table, buffers, logs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(&allocator, &table, buffers, logs)
	if err != nil {
		t.Fatal(err)
	}

	if got := first.ID(); got != recovery.TransactionID(1) {
		t.Fatalf("first ID = %d, want 1", got)
	}
	if got := second.ID(); got != recovery.TransactionID(2) {
		t.Fatalf("second ID = %d, want 2", got)
	}
	if first.locks == second.locks {
		t.Fatal("transactions share local concurrency manager")
	}
	if first.buffers == second.buffers {
		t.Fatal("transactions share local buffer ownership")
	}
	if len(first.buffers.pins) != 0 || len(second.buffers.pins) != 0 {
		t.Fatalf("new transaction pins = (%v, %v), want empty", first.buffers.pins, second.buffers.pins)
	}
	if len(logs.records) != 2 {
		t.Fatalf("START record count = %d, want 2", len(logs.records))
	}
	for index, encoded := range logs.records {
		record, err := recovery.DecodeStartRecord(encoded)
		if err != nil {
			t.Fatalf("decode START record %d: %v", index, err)
		}
		want := recovery.TransactionID(index + 1)
		if got := record.TransactionID(); got != want {
			t.Fatalf("START record %d transaction = %d, want %d", index, got, want)
		}
	}
}

func TestNewReturnsNoTransactionWhenStartAppendFails(t *testing.T) {
	wantErr := errors.New("append failed")
	var allocator IDAllocator
	var table concurrency.LockTable

	got, err := New(&allocator, &table, &recordingBufferPinner{}, &recordingTransactionLog{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("New() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Fatalf("New() transaction = %#v, want nil", got)
	}
}
