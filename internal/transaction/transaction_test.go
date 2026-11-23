package transaction

import (
	"errors"
	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/concurrency"
	storagefile "github.com/Rwatana/simledb_in_go/internal/infrastructure/file"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/storage"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestTransactionRejectsRepeatedCommitAfterDurability(t *testing.T) {
	var allocator IDAllocator
	var table concurrency.LockTable
	bufferManager := &recordingBufferPinner{}
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	flushes := len(bufferManager.flushes)
	records := len(logs.records)
	logFlushes := len(logs.flushed)

	if _, err := tx.Commit(); !errors.Is(err, ErrTransactionCompleted) {
		t.Fatalf("second Commit() error = %v, want ErrTransactionCompleted", err)
	}
	if len(bufferManager.flushes) != flushes || len(logs.records) != records || len(logs.flushed) != logFlushes {
		t.Fatalf("rejected commit performed I/O: buffer flushes %d->%d records %d->%d log flushes %d->%d", flushes, len(bufferManager.flushes), records, len(logs.records), logFlushes, len(logs.flushed))
	}
}

func TestTransactionCommitDurabilityFailureRemainsRetryable(t *testing.T) {
	wantErr := errors.New("flush failed")
	var allocator IDAllocator
	var table concurrency.LockTable
	bufferManager := &recordingBufferPinner{flushErr: wantErr}
	logs := &recordingTransactionLog{}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Commit(); !errors.Is(err, wantErr) {
		t.Fatalf("first Commit() error = %v, want %v", err, wantErr)
	}
	bufferManager.flushErr = nil
	if _, err := tx.Commit(); err != nil {
		t.Fatalf("Commit() retry after pre-durability failure: %v", err)
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
	if _, err := tx.Commit(); !errors.Is(err, ErrTransactionCompleted) {
		t.Fatalf("Commit() after durable cleanup failure error = %v, want ErrTransactionCompleted", err)
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

func TestTransactionGetIntPreservesSharedLockFailure(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	wantErr := concurrency.ErrLockConflict
	locks := &recordingTransactionLockManager{sharedErr: wantErr}
	tx.locks = locks

	value, err := tx.GetInt(block, 64)
	if value != 0 {
		t.Errorf("GetInt() value = %d, want zero after lock failure", value)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("GetInt() error = %v, want lock conflict", err)
	}
	wantContext := "get integer from transaction buffer " + block.String() + " at offset 64: acquire shared lock"
	if err == nil || !strings.Contains(err.Error(), wantContext) {
		t.Errorf("GetInt() error = %v, want context %q", err, wantContext)
	}
	if locks.sharedCalls != 1 {
		t.Errorf("shared lock calls = %d, want 1", locks.sharedCalls)
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

func TestTransactionGetStringPreservesSharedLockFailure(t *testing.T) {
	bufferManager, logs, block := newTransactionStringBlock(t, 8, "値")
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	wantErr := concurrency.ErrLockConflict
	locks := &recordingTransactionLockManager{sharedErr: wantErr}
	tx.locks = locks

	value, err := tx.GetString(block, 64)
	if value != "" {
		t.Errorf("GetString() value = %q, want empty string after lock failure", value)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("GetString() error = %v, want lock conflict", err)
	}
	wantContext := "get string from transaction buffer " + block.String() + " at offset 64: acquire shared lock"
	if err == nil || !strings.Contains(err.Error(), wantContext) {
		t.Errorf("GetString() error = %v, want context %q", err, wantContext)
	}
	if locks.sharedCalls != 1 {
		t.Errorf("shared lock calls = %d, want 1", locks.sharedCalls)
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

func TestTransactionSetIntLogsPreviousValueBeforeMutation(t *testing.T) {
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

	if err := tx.SetInt(block, 8, 77, true); err != nil {
		t.Fatal(err)
	}
	if len(logs.records) != 2 {
		t.Fatalf("log records = %d, want START and SETINT", len(logs.records))
	}
	record, err := recovery.DecodeSetIntRecord(logs.records[1])
	if err != nil {
		t.Fatal(err)
	}
	if record.TransactionID() != tx.ID() || record.Block() != block || record.Offset() != 8 || record.OldValue() != 42 {
		t.Fatalf("SETINT = tx %d block %v offset %d old %d", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
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
	if lsn, ok := frame.RecoveryLSN(); !ok || lsn != 2 {
		t.Fatalf("recovery LSN = (%d, %v), want (2, true)", lsn, ok)
	}
}

func TestTransactionSetIntLogFailurePreservesPageAndMetadata(t *testing.T) {
	wantErr := errors.New("append setint failed")
	bufferManager, _, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	logs := &failingUpdateTransactionLog{updateErr: wantErr}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	if err := tx.SetInt(block, 8, 77, true); !errors.Is(err, wantErr) {
		t.Fatalf("SetInt() error = %v, want %v", err, wantErr)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("failed SetInt removed transaction buffer ownership")
	}
	got, err := frame.Contents().GetInt(8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("integer after log failure = %d, want old value 42", got)
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Fatal("log failure marked buffer modified")
	}
	if _, ok := frame.RecoveryLSN(); ok {
		t.Fatal("log failure recorded a recovery LSN")
	}
}

func TestTransactionSetIntPreservesExclusiveLockFailure(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	wantErr := concurrency.ErrLockConflict
	locks := &recordingTransactionLockManager{exclusiveErr: wantErr}
	tx.locks = locks

	err = tx.SetInt(block, 8, 77, false)
	if !errors.Is(err, wantErr) {
		t.Errorf("SetInt() error = %v, want lock conflict", err)
	}
	wantContext := "set integer in transaction buffer " + block.String() + " at offset 8: acquire exclusive lock"
	if err == nil || !strings.Contains(err.Error(), wantContext) {
		t.Errorf("SetInt() error = %v, want context %q", err, wantContext)
	}
	if locks.exclusiveCall != 1 {
		t.Errorf("exclusive lock calls = %d, want 1", locks.exclusiveCall)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("failed SetInt removed transaction buffer ownership")
	}
	got, err := frame.Contents().GetInt(8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Errorf("integer after lock failure = %d, want original value 42", got)
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Error("exclusive lock failure marked the page modified")
	}
}

func TestSetIntRecoveryLSNPreservesRecorderFailure(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 4)
	wantErr := errors.New("recovery LSN rejected")
	var gotLSN logmanager.LSN
	err := recordRecoveryLSN(func(lsn logmanager.LSN) error {
		gotLSN = lsn
		return wantErr
	}, 17, "set integer", block, 8)

	if gotLSN != 17 {
		t.Errorf("recorded LSN = %d, want 17", gotLSN)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("recordRecoveryLSN() error = %v, want wrapped recorder failure", err)
	}
	for _, context := range []string{"set integer", block.String(), "offset 8", "LSN 17"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("recordRecoveryLSN() error = %q, want context %q", err, context)
		}
	}
}

func TestTransactionSetStringWithoutLoggingUpdatesPinnedPageExclusively(t *testing.T) {
	bufferManager, _, block := newTransactionStringBlock(t, 8, "old")
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

	if err := tx.SetString(block, 8, "新しい値", false); err != nil {
		t.Fatal(err)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("SetString removed transaction buffer ownership")
	}
	got, err := frame.Contents().GetBytes(8)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "新しい値" {
		t.Fatalf("stored string = %q, want %q", got, "新しい値")
	}
	if modifiedBy, dirty := frame.ModifyingTransaction(); !dirty || modifiedBy != buffer.TransactionID(tx.ID()) {
		t.Fatalf("modifying transaction = (%d, %v), want (%d, true)", modifiedBy, dirty, tx.ID())
	}
	if lsn, ok := frame.RecoveryLSN(); ok {
		t.Fatalf("no-log SetString recovery LSN = %d, want absent", lsn)
	}
	if len(logs.records) != 1 {
		t.Fatalf("log records = %d, want START only", len(logs.records))
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition after SetString error = %v, want ErrLockConflict", err)
	}
}

func TestTransactionSetStringWithoutLoggingReturnsPreconditionAndPageFailures(t *testing.T) {
	bufferManager, _, block := newTransactionStringBlock(t, 8, "old")
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.SetString(block, 8, "new", false); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("SetString() without pin error = %v, want ErrBufferNotPinned", err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetString(block, 60, "too long", false); err == nil {
		t.Fatal("SetString() outside page returned nil error")
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("failed SetString removed transaction buffer ownership")
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Fatal("failed SetString marked buffer modified")
	}
	if err := tx.SetString(block, 8, "new", true); err == nil {
		t.Fatal("logged SetString silently used the no-log path")
	}
}

func TestTransactionSetStringLogsPreviousValueBeforeMutation(t *testing.T) {
	bufferManager, _, block := newTransactionStringBlock(t, 8, "old")
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

	if err := tx.SetString(block, 8, "新しい値", true); err != nil {
		t.Fatal(err)
	}
	if len(logs.records) != 2 {
		t.Fatalf("log records = %d, want START and SETSTRING", len(logs.records))
	}
	record, err := recovery.DecodeSetStringRecord(logs.records[1])
	if err != nil {
		t.Fatal(err)
	}
	if record.TransactionID() != tx.ID() || record.Block() != block || record.Offset() != 8 || record.OldValue() != "old" {
		t.Fatalf("SETSTRING = tx %d block %v offset %d old %q", record.TransactionID(), record.Block(), record.Offset(), record.OldValue())
	}
	frame, _ := tx.buffers.Buffer(block)
	got, err := frame.Contents().GetBytes(8)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "新しい値" {
		t.Fatalf("stored string = %q, want %q", got, "新しい値")
	}
	if modifiedBy, dirty := frame.ModifyingTransaction(); !dirty || modifiedBy != buffer.TransactionID(tx.ID()) {
		t.Fatalf("modifying transaction = (%d, %v), want (%d, true)", modifiedBy, dirty, tx.ID())
	}
	if lsn, ok := frame.RecoveryLSN(); !ok || lsn != 2 {
		t.Fatalf("recovery LSN = (%d, %v), want (2, true)", lsn, ok)
	}
}

func TestTransactionSetStringLogFailurePreservesPageAndMetadata(t *testing.T) {
	wantErr := errors.New("append setstring failed")
	bufferManager, _, block := newTransactionStringBlock(t, 8, "old")
	var allocator IDAllocator
	var table concurrency.LockTable
	logs := &failingUpdateTransactionLog{updateErr: wantErr}
	tx, err := New(&allocator, &table, bufferManager, logs)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}

	if err := tx.SetString(block, 8, "new", true); !errors.Is(err, wantErr) {
		t.Fatalf("SetString() error = %v, want %v", err, wantErr)
	}
	frame, _ := tx.buffers.Buffer(block)
	got, err := frame.Contents().GetBytes(8)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("string after log failure = %q, want old value %q", got, "old")
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Fatal("log failure marked buffer modified")
	}
	if _, ok := frame.RecoveryLSN(); ok {
		t.Fatal("log failure recorded a recovery LSN")
	}
}

func TestTransactionSetStringPreservesExclusiveLockFailure(t *testing.T) {
	bufferManager, _, block := newTransactionStringBlock(t, 8, "old")
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, &recordingTransactionLog{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	wantErr := concurrency.ErrLockConflict
	locks := &recordingTransactionLockManager{exclusiveErr: wantErr}
	tx.locks = locks

	err = tx.SetString(block, 8, "new", false)
	if !errors.Is(err, wantErr) {
		t.Errorf("SetString() error = %v, want lock conflict", err)
	}
	wantContext := "set string in transaction buffer " + block.String() + " at offset 8: acquire exclusive lock"
	if err == nil || !strings.Contains(err.Error(), wantContext) {
		t.Errorf("SetString() error = %v, want context %q", err, wantContext)
	}
	if locks.exclusiveCall != 1 {
		t.Errorf("exclusive lock calls = %d, want 1", locks.exclusiveCall)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("failed SetString removed transaction buffer ownership")
	}
	got, err := frame.Contents().GetBytes(8)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Errorf("string after lock failure = %q, want original value %q", got, "old")
	}
	if _, dirty := frame.ModifyingTransaction(); dirty {
		t.Error("exclusive lock failure marked the page modified")
	}
}

func TestSetStringRecoveryLSNPreservesRecorderFailure(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 7)
	wantErr := errors.New("recovery LSN rejected")
	var gotLSN logmanager.LSN
	err := recordStringRecoveryLSN(func(lsn logmanager.LSN) error {
		gotLSN = lsn
		return wantErr
	}, 23, block, 12)

	if gotLSN != 23 {
		t.Errorf("recorded LSN = %d, want 23", gotLSN)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("recordRecoveryLSN() error = %v, want wrapped recorder failure", err)
	}
	for _, context := range []string{"set string", block.String(), "offset 12", "LSN 23"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("recordRecoveryLSN() error = %q, want context %q", err, context)
		}
	}
}

func TestTransactionRollbackInvokesDurabilityForOwnID(t *testing.T) {
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	factory := func() (recovery.EncodedRecordIterator, error) {
		return logs.Iterator()
	}
	tx, err := New(&allocator, &table, bufferManager, logs, factory)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetInt(block, 8, 77, true); err != nil {
		t.Fatal(err)
	}

	lsn, err := tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if lsn != 3 {
		t.Fatalf("Rollback() LSN = %d, want 3", lsn)
	}
	frame, ok := tx.buffers.Buffer(block)
	if !ok {
		t.Fatal("rollback released transaction buffer ownership")
	}
	got, err := frame.Contents().GetInt(8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("integer after rollback = %d, want old value 42", got)
	}
	iterator, err := logs.Iterator()
	if err != nil {
		t.Fatal(err)
	}
	encoded, ok, err := iterator.Next()
	if err != nil || !ok {
		t.Fatalf("latest log record = (%v, %v), want ROLLBACK", ok, err)
	}
	record, err := recovery.DecodeRecord(encoded)
	if err != nil {
		t.Fatal(err)
	}
	recordTransaction, owned := recovery.RecordTransaction(record)
	if record.Operation() != recovery.OperationRollback || !owned || recordTransaction != tx.ID() {
		t.Fatalf("latest record = operation %d transaction (%d, %v), want ROLLBACK for %d", record.Operation(), recordTransaction, owned, tx.ID())
	}
	if err := table.LockShared(block); err != nil {
		t.Fatalf("shared acquisition after durable rollback: %v", err)
	}
}

func TestTransactionRollbackFailureKeepsLocksAndPins(t *testing.T) {
	wantErr := errors.New("iterator failed")
	bufferManager, logs, block := newTransactionDataBlock(t, 8, 42)
	var allocator IDAllocator
	var table concurrency.LockTable
	fail := true
	factory := func() (recovery.EncodedRecordIterator, error) {
		if fail {
			return nil, wantErr
		}
		return logs.Iterator()
	}
	tx, err := New(&allocator, &table, bufferManager, logs, factory)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Rollback(); !errors.Is(err, wantErr) {
		t.Fatalf("Rollback() error = %v, want %v", err, wantErr)
	}
	if _, ok := tx.buffers.Buffer(block); !ok {
		t.Fatal("failed rollback released transaction buffer ownership")
	}
	if err := table.LockShared(block); !errors.Is(err, concurrency.ErrLockConflict) {
		t.Fatalf("shared acquisition after failed rollback error = %v, want retained exclusive lock", err)
	}
	fail = false
	if _, err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() retry after pre-durability failure: %v", err)
	}
}

func TestTransactionRejectsRepeatedRollbackAfterDurability(t *testing.T) {
	start := recovery.EncodeStartRecord(1)
	bufferManager := &recordingBufferPinner{}
	logs := &recordingTransactionLog{}
	iteratorCalls := 0
	factory := func() (recovery.EncodedRecordIterator, error) {
		iteratorCalls++
		return &transactionRecordIterator{records: [][]byte{start}}, nil
	}
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs, factory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	flushes := len(bufferManager.flushes)
	records := len(logs.records)
	logFlushes := len(logs.flushed)

	if _, err := tx.Rollback(); !errors.Is(err, ErrTransactionCompleted) {
		t.Fatalf("second Rollback() error = %v, want ErrTransactionCompleted", err)
	}
	if iteratorCalls != 1 || len(bufferManager.flushes) != flushes || len(logs.records) != records || len(logs.flushed) != logFlushes {
		t.Fatalf("rejected rollback performed I/O: iterators %d buffer flushes %d->%d records %d->%d log flushes %d->%d", iteratorCalls, flushes, len(bufferManager.flushes), records, len(logs.records), logFlushes, len(logs.flushed))
	}
}

func TestTransactionRollbackRequiresRecoveryIterator(t *testing.T) {
	buffers := &recordingBufferPinner{}
	logs := &recordingTransactionLog{}
	tx := &Transaction{bufferManager: buffers, logs: logs}

	lsn, err := tx.Rollback()
	if lsn != 0 {
		t.Errorf("Rollback() LSN = %d, want zero without a recovery iterator", lsn)
	}
	wantErr := "rollback transaction 0: recovery iterator factory is not configured"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Rollback() error = %v, want %q", err, wantErr)
	}
	if len(buffers.flushes) != 0 {
		t.Errorf("buffer flushes = %v, want none without a recovery iterator", buffers.flushes)
	}
	if len(logs.records) != 0 {
		t.Errorf("log records = %d, want none without a recovery iterator", len(logs.records))
	}
}

type failingUpdateTransactionLog struct {
	recordingTransactionLog
	updateErr error
}

type transactionRecordIterator struct {
	records [][]byte
	index   int
}

func (i *transactionRecordIterator) Next() ([]byte, bool, error) {
	if i.index >= len(i.records) {
		return nil, false, nil
	}
	record := i.records[i.index]
	i.index++
	return record, true, nil
}

func TestTransactionRollbackReturnsDurableLSNWhenUnpinFails(t *testing.T) {
	wantErr := errors.New("unpin failed")
	start := recovery.EncodeStartRecord(1)
	frame := &buffer.Buffer{}
	bufferManager := &recordingBufferPinner{frame: frame, unpinErr: wantErr}
	logs := &recordingTransactionLog{}
	factory := func() (recovery.EncodedRecordIterator, error) {
		return &transactionRecordIterator{records: [][]byte{start}}, nil
	}
	var allocator IDAllocator
	var table concurrency.LockTable
	tx, err := New(&allocator, &table, bufferManager, logs, factory)
	if err != nil {
		t.Fatal(err)
	}
	block := storage.NewBlockID("tbl", 1)
	if err := tx.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := tx.locks.LockExclusive(block); err != nil {
		t.Fatal(err)
	}

	lsn, err := tx.Rollback()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Rollback() error = %v, want %v", err, wantErr)
	}
	if lsn != 2 {
		t.Fatalf("Rollback() LSN after cleanup failure = %d, want durable LSN 2", lsn)
	}
	if len(logs.flushed) != 1 || logs.flushed[0] != lsn {
		t.Fatalf("log flushes = %v, want [%d]", logs.flushed, lsn)
	}
	if got, ok := tx.buffers.Buffer(block); !ok || got != frame {
		t.Fatalf("buffer after failed cleanup = (%p, %v), want (%p, true)", got, ok, frame)
	}
	if _, err := tx.Rollback(); !errors.Is(err, ErrTransactionCompleted) {
		t.Fatalf("Rollback() after durable cleanup failure error = %v, want ErrTransactionCompleted", err)
	}
	if err := table.LockShared(block); err != nil {
		t.Fatalf("lock remained held after durable rollback cleanup failure: %v", err)
	}
}
