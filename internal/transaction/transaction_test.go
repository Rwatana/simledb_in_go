package transaction

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingTransactionLog struct {
	records  [][]byte
	err      error
	flushed  []logmanager.LSN
	flushErr error
	events   *[]string
}

func (l *recordingTransactionLog) Append(record []byte) (logmanager.LSN, error) {
	if l.err != nil {
		return 0, l.err
	}
	l.records = append(l.records, append([]byte(nil), record...))
	if l.events != nil {
		*l.events = append(*l.events, "append record")
	}
	return logmanager.LSN(len(l.records)), nil
}

func (l *recordingTransactionLog) Flush(lsn logmanager.LSN) error {
	l.flushed = append(l.flushed, lsn)
	if l.events != nil {
		*l.events = append(*l.events, "flush log")
	}
	return l.flushErr
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

func TestTransactionCommitInvokesDurabilityForOwnID(t *testing.T) {
	events := []string{}
	var allocator IDAllocator
	var table concurrency.LockTable
	bufferManager := &recordingBufferPinner{frame: &buffer.Buffer{}, events: &events}
	logs := &recordingTransactionLog{events: &events}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	events = events[:0]

	lsn, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if lsn != 2 {
		t.Fatalf("Commit() LSN = %d, want 2", lsn)
	}
	wantEvents := []string{"flush buffers", "append record", "flush log"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("commit events = %v, want %v", events, wantEvents)
	}
	if len(bufferManager.flushes) != 1 || bufferManager.flushes[0] != buffer.TransactionID(tx.ID()) {
		t.Fatalf("buffer flush IDs = %v, want [%d]", bufferManager.flushes, tx.ID())
	}
	if len(logs.records) != 2 {
		t.Fatalf("log records = %d, want START and COMMIT", len(logs.records))
	}
	record, err := recovery.DecodeCommitRecord(logs.records[1])
	if err != nil {
		t.Fatal(err)
	}
	if got := record.TransactionID(); got != tx.ID() {
		t.Fatalf("COMMIT transaction = %d, want %d", got, tx.ID())
	}
	if len(logs.flushed) != 1 || logs.flushed[0] != lsn {
		t.Fatalf("log flushes = %v, want [%d]", logs.flushed, lsn)
	}
}

func TestTransactionCommitFailureKeepsPinnedBuffers(t *testing.T) {
	wantErr := errors.New("flush failed")
	var allocator IDAllocator
	var table concurrency.LockTable
	frame := &buffer.Buffer{}
	bufferManager := &recordingBufferPinner{frame: frame, flushErr: wantErr}
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 1)
	if err := tx.buffers.Pin(block); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Commit(); !errors.Is(err, wantErr) {
		t.Fatalf("Commit() error = %v, want %v", err, wantErr)
	}
	if got, ok := tx.buffers.Buffer(block); !ok || got != frame {
		t.Fatalf("pinned buffer after failed commit = (%p, %v), want (%p, true)", got, ok, frame)
	}
	if len(logs.records) != 1 {
		t.Fatalf("records after failed commit = %d, want START only", len(logs.records))
	}
	if len(logs.flushed) != 0 {
		t.Fatalf("log flushes after failed commit = %v, want none", logs.flushed)
	}
}

func TestTransactionCommitReleasesLocksAfterDurability(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	bufferManager := &recordingBufferPinner{}
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 1)
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition before commit error = %v, want ErrLockConflict", err)
	}

	if _, err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatalf("shared acquisition after commit: %v", err)
	}
}

func TestTransactionCommitFailureKeepsLocksHeld(t *testing.T) {
	wantErr := errors.New("flush failed")
	var allocator IDAllocator
	var table concurrency.LockTable
	bufferManager := &recordingBufferPinner{flushErr: wantErr}
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 1)
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Commit(); !errors.Is(err, wantErr) {
		t.Fatalf("Commit() error = %v, want %v", err, wantErr)
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition after failed commit error = %v, want ErrLockConflict", err)
	}
}

func TestTransactionCommitUnpinsBuffersAfterLockRelease(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	frame := &buffer.Buffer{}
	var lockErrorAtUnpin error
	bufferManager := &recordingBufferPinner{frame: frame}
	bufferManager.onUnpin = func(*buffer.Buffer) {
		lockErrorAtUnpin = table.LockShared(block)
	}
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.buffers.Pin(block); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if lockErrorAtUnpin != nil {
		t.Fatalf("shared acquisition during unpin = %v, want released lock", lockErrorAtUnpin)
	}
	if len(bufferManager.unpins) != 1 || bufferManager.unpins[0] != frame {
		t.Fatalf("unpin requests = %v, want [%p]", bufferManager.unpins, frame)
	}
	if _, ok := tx.buffers.Buffer(block); ok {
		t.Fatal("buffer remains tracked after committed cleanup")
	}
	if len(tx.buffers.pins) != 0 {
		t.Fatalf("pin occurrences after committed cleanup = %v, want none", tx.buffers.pins)
	}
}

func TestTransactionCommitReturnsDurableLSNWhenUnpinFails(t *testing.T) {
	wantErr := errors.New("unpin failed")
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	frame := &buffer.Buffer{}
	bufferManager := &recordingBufferPinner{frame: frame, unpinErr: wantErr}
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.buffers.Pin(block); err != nil {
		t.Fatal(err)
	}

	lsn, err := tx.Commit()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Commit() error = %v, want %v", err, wantErr)
	}
	if lsn != 2 {
		t.Fatalf("Commit() LSN after cleanup failure = %d, want durable LSN 2", lsn)
	}
	if len(logs.flushed) != 1 || logs.flushed[0] != lsn {
		t.Fatalf("log flushes = %v, want [%d]", logs.flushed, lsn)
	}
	if got, ok := tx.buffers.Buffer(block); !ok || got != frame {
		t.Fatalf("buffer after failed cleanup = (%p, %v), want (%p, true)", got, ok, frame)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatalf("shared acquisition after cleanup failure: %v", err)
	}
}

func TestTransactionPinsBlockWithoutAcquiringLock(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	if err := table.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	frame := &buffer.Buffer{}
	bufferManager := &recordingBufferPinner{frame: frame}
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if len(bufferManager.blocks) != 1 || bufferManager.blocks[0] != block {
		t.Fatalf("pin requests = %v, want [%v]", bufferManager.blocks, block)
	}
	if got, ok := tx.buffers.Buffer(block); !ok || got != frame {
		t.Fatalf("transaction buffer = (%p, %v), want (%p, true)", got, ok, frame)
	}
}

func TestTransactionPinFailureDoesNotRecordBuffer(t *testing.T) {
	wantErr := errors.New("pin failed")
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	bufferManager := &recordingBufferPinner{err: wantErr}
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Pin(block); !errors.Is(err, wantErr) {
		t.Fatalf("Pin() error = %v, want %v", err, wantErr)
	}
	if _, ok := tx.buffers.Buffer(block); ok {
		t.Fatal("failed transaction pin recorded a buffer")
	}
}

func TestTransactionUnpinsFinalOccurrenceWithoutReleasingLock(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	frame := &buffer.Buffer{}
	bufferManager := &recordingBufferPinner{frame: frame}
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	if err := tx.Unpin(block); err != nil {
		t.Fatal(err)
	}
	if len(bufferManager.unpins) != 1 || bufferManager.unpins[0] != frame {
		t.Fatalf("unpin requests = %v, want [%p]", bufferManager.unpins, frame)
	}
	if _, ok := tx.buffers.Buffer(block); ok {
		t.Fatal("buffer remains tracked after final transaction unpin")
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition after unpin error = %v, want ErrLockConflict", err)
	}
}

func TestTransactionUnpinPropagatesOwnershipErrors(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	block := storage.NewBlockID("tbl", 1)
	tx, err := New(&allocator, &table, &recordingBufferPinner{}, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Unpin(block); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("Unpin() error = %v, want ErrBufferNotPinned", err)
	}
}

func TestTransactionGetIntReadsPinnedPageAndReusesSharedLock(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		got, err := tx.GetInt(block, 8)
		if err != nil {
			t.Fatal(err)
		}
		if got != 42 {
			t.Fatalf("GetInt() = %d, want 42", got)
		}
	}
	if err := table.LockExclusive(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("exclusive acquisition with competing shared hold error = %v, want ErrLockConflict", err)
	}
	table.Unlock(block)
	if err := table.LockExclusive(block); err != nil {
		t.Fatalf("repeated GetInt acquired more than one transaction shared hold: %v", err)
	}
}

func TestTransactionGetIntReturnsMissingPinAndPageFailures(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.GetInt(block, 8); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("GetInt() without pin error = %v, want ErrBufferNotPinned", err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.GetInt(block, 62); err == nil {
		t.Fatal("GetInt() outside page returned nil error")
	}
}

func TestTransactionGetStringReadsPinnedPageAndReusesSharedLock(t *testing.T) {
	bufferManager, logs, block := newTransactionStringBlock(t, 8, "こんにちは")
	var allocator IDAllocator
	var table concurrency.LockTable
	if err := table.LockShared(block); err != nil {
		t.Fatal(err)
	}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		got, err := tx.GetString(block, 8)
		if err != nil {
			t.Fatal(err)
		}
		if got != "こんにちは" {
			t.Fatalf("GetString() = %q, want %q", got, "こんにちは")
		}
	}
	if err := table.LockExclusive(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("exclusive acquisition with competing shared hold error = %v, want ErrLockConflict", err)
	}
	table.Unlock(block)
	if err := table.LockExclusive(block); err != nil {
		t.Fatalf("repeated GetString acquired more than one transaction shared hold: %v", err)
	}
}

func TestTransactionGetStringReturnsMissingPinAndPageFailures(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, -1)
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.GetString(block, 8); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("GetString() without pin error = %v, want ErrBufferNotPinned", err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.GetString(block, 8); err == nil {
		t.Fatal("GetString() with negative length returned nil error")
	}
	if _, err := tx.GetString(block, 62); err == nil {
		t.Fatal("GetString() outside page returned nil error")
	}
}

func TestTransactionSetIntWithoutLoggingUpdatesPinnedPageExclusively(t *testing.T) {
	bufferManager, _, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	if err := tx.SetInt(block, 8, 77, false); err != nil {
		t.Fatal(err)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("SetInt removed transaction buffer ownership")
	}
	got, err := frame.Contents().GetInt(8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 77 {
		t.Fatalf("stored integer = %d, want 77", got)
	}
	if modifiedBy, dirty := frame.ModifyingTransaction(); !dirty || modifiedBy != buffer.TransactionID(tx.ID()) {
		t.Fatalf("modifying transaction = (%d, %v), want (%d, true)", modifiedBy, dirty, tx.ID())
	}
	if lsn, ok := frame.RecoveryLSN(); ok {
		t.Fatalf("no-log SetInt recovery LSN = %d, want absent", lsn)
	}
	if len(logs.records) != 1 {
		t.Fatalf("log records = %d, want START only", len(logs.records))
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition after SetInt error = %v, want ErrLockConflict", err)
	}
}

func TestTransactionSetIntWithoutLoggingReturnsPreconditionAndPageFailures(t *testing.T) {
	bufferManager, _, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.SetInt(block, 8, 77, false); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("SetInt() without pin error = %v, want ErrBufferNotPinned", err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetInt(block, 62, 77, false); err == nil {
		t.Fatal("SetInt() outside page returned nil error")
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("failed SetInt removed transaction buffer ownership")
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Fatal("failed SetInt marked buffer modified")
	}
	if err := tx.SetInt(block, 8, 77, true); err == nil {
		t.Fatal("logged SetInt silently used the no-log path")
	}
}

func newTransactionDataBlock(t *testing.T, offset int, value int32) (*buffer.Manager, *logmanager.Manager, storage.BlockID) {
	t.Helper()
	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 64)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatal(err)
	}
	page, err := storage.NewPage(files.BlockSize())
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetInt(offset, value); err != nil {
		t.Fatal(err)
	}
	block, err := files.Append("table.tbl")
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Write(block, page); err != nil {
		t.Fatal(err)
	}
	bufferManager, err := buffer.NewManager(files, logs, 1)
	if err != nil {
		t.Fatal(err)
	}
	return bufferManager, logs, block
}

func newTransactionStringBlock(t *testing.T, offset int, value string) (*buffer.Manager, *logmanager.Manager, storage.BlockID) {
	t.Helper()
	files, err := storagefile.NewManager(filepath.Join(t.TempDir(), "database"), 64)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := logmanager.NewManager(files, "simpledb.log")
	if err != nil {
		t.Fatal(err)
	}
	page, err := storage.NewPage(files.BlockSize())
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetBytes(offset, []byte(value)); err != nil {
		t.Fatal(err)
	}
	block, err := files.Append("table.tbl")
	if err != nil {
		t.Fatal(err)
	}
	if err := files.Write(block, page); err != nil {
		t.Fatal(err)
	}
	bufferManager, err := buffer.NewManager(files, logs, 1)
	if err != nil {
		t.Fatal(err)
	}
	return bufferManager, logs, block
}
