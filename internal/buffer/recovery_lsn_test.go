package buffer

import (
	"sync"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

func TestRecordRecoveryLSNRequiresADirtyBufferAndPositiveLSN(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	if lsn, exists := frame.RecoveryLSN(); exists {
		t.Errorf("fresh RecoveryLSN() = (%d, true), want (_, false)", lsn)
	}
	if err := frame.RecordRecoveryLSN(logmanager.LSN(1)); err == nil {
		t.Error("RecordRecoveryLSN(1) on clean frame error = nil, want a dirty-state error")
	}

	frame.MarkModified(TransactionID(7))
	if lsn, exists := frame.RecoveryLSN(); exists {
		t.Errorf("RecoveryLSN() after transaction-only mark = (%d, true), want (_, false)", lsn)
	}
	if err := frame.RecordRecoveryLSN(0); err == nil {
		t.Error("RecordRecoveryLSN(0) error = nil, want an unallocated LSN error")
	}
	if _, exists := frame.RecoveryLSN(); exists {
		t.Error("RecoveryLSN() exists after rejected LSN 0")
	}

	if err := frame.RecordRecoveryLSN(logmanager.LSN(1)); err != nil {
		t.Fatalf("RecordRecoveryLSN(1) error = %v", err)
	}
	if lsn, exists := frame.RecoveryLSN(); !exists || lsn != 1 {
		t.Errorf("RecoveryLSN() = (%d, %t), want (1, true)", lsn, exists)
	}
	if err := frame.RecordRecoveryLSN(logmanager.LSN(9)); err != nil {
		t.Fatalf("RecordRecoveryLSN(9) error = %v", err)
	}
	if lsn, exists := frame.RecoveryLSN(); !exists || lsn != 9 {
		t.Errorf("RecoveryLSN() after replacement = (%d, %t), want (9, true)", lsn, exists)
	}
}

func TestRecoveryLSNMetadataIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	frame := newBufferForTest(t)
	frame.MarkModified(TransactionID(7))
	const goroutines = 32
	var wait sync.WaitGroup
	wait.Add(goroutines)
	for index := range goroutines {
		go func(lsn logmanager.LSN) {
			defer wait.Done()
			if err := frame.RecordRecoveryLSN(lsn); err != nil {
				t.Errorf("RecordRecoveryLSN(%d) error = %v", lsn, err)
			}
			if got, exists := frame.RecoveryLSN(); !exists || got == 0 || got > goroutines {
				t.Errorf("RecoveryLSN() = (%d, %t), want a concurrently written LSN", got, exists)
			}
		}(logmanager.LSN(index + 1))
	}
	wait.Wait()

	lsn, exists := frame.RecoveryLSN()
	if !exists || lsn == 0 || lsn > goroutines {
		t.Errorf("final RecoveryLSN() = (%d, %t), want one of [1, %d]", lsn, exists, goroutines)
	}
}
