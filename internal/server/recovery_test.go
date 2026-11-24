package server

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
	"github.com/Rwatana/simledb_in_go/internal/recovery"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestManagersRecoverDurablyUndoesFlushedUncommittedUpdate(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "database")
	managers, err := NewManagers(directory, 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	block := storage.BlockID{}
	committed, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() committed error = %v", err)
	}
	block, err = committed.Append("data.tbl")
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := committed.Pin(block); err != nil {
		t.Fatalf("Pin() committed block error = %v", err)
	}
	if err := committed.SetInt(block, 0, 17, true); err != nil {
		t.Fatalf("SetInt() committed value error = %v", err)
	}
	if _, err := committed.Commit(); err != nil {
		t.Fatalf("Commit() initial value error = %v", err)
	}

	uncommitted, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() uncommitted error = %v", err)
	}
	if err := uncommitted.Pin(block); err != nil {
		t.Fatalf("Pin() uncommitted block error = %v", err)
	}
	if err := uncommitted.SetInt(block, 0, 99, true); err != nil {
		t.Fatalf("SetInt() uncommitted value error = %v", err)
	}
	if err := managers.Buffers.FlushAll(buffer.TransactionID(uncommitted.ID())); err != nil {
		t.Fatalf("FlushAll() uncommitted value error = %v", err)
	}

	managers, err = NewManagers(directory, 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() after simulated restart error = %v", err)
	}
	checkpointLSN, err := managers.Recover()
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if checkpointLSN == 0 {
		t.Fatalf("Recover() checkpoint LSN = %d, want a positive durable LSN", checkpointLSN)
	}
	assertRecoveryCheckpointBeforeCommit(t, managers.Logs)

	verify, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() verification error = %v", err)
	}
	if err := verify.Pin(block); err != nil {
		t.Fatalf("Pin() verification block error = %v", err)
	}
	got, err := verify.GetInt(block, 0)
	if err != nil {
		t.Fatalf("GetInt() recovered value error = %v", err)
	}
	if got != 17 {
		t.Fatalf("GetInt() recovered value = %d, want 17", got)
	}
	if _, err := verify.Commit(); err != nil {
		t.Fatalf("Commit() verification transaction error = %v", err)
	}
}

func TestManagersRecoverRejectsNilAndIncompleteManagerSets(t *testing.T) {
	for _, test := range []struct {
		name     string
		managers *Managers
	}{
		{name: "nil manager set"},
		{name: "incomplete manager set", managers: &Managers{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.managers.Recover(); err == nil || !strings.Contains(err.Error(), "manager") {
				t.Fatalf("Recover() error = %v, want manager validation error", err)
			}
		})
	}
}

func TestManagersRecoverPreservesRestartHistoryError(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "database")
	managers, err := NewManagers(directory, 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	lsn, err := managers.Logs.Append([]byte{0, 0, 0, 127})
	if err != nil {
		t.Fatalf("Append() corrupt history error = %v", err)
	}
	if err := managers.Logs.Flush(lsn); err != nil {
		t.Fatalf("Flush() corrupt history error = %v", err)
	}
	managers, err = NewManagers(directory, 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() before restart error = %v", err)
	}
	if _, err := managers.Recover(); err == nil || !strings.Contains(err.Error(), "restart") {
		t.Fatalf("Recover() error = %v, want contextual restart history error", err)
	}
}

func TestFinishRestartRecoveryReportsRecoveryAndCommitFailures(t *testing.T) {
	commitErr := errors.New("commit failed")
	recoveryErr := errors.New("recovery failed")
	for _, test := range []struct {
		name       string
		recoverErr error
		commitErr  error
		wantLSN    logmanager.LSN
		wantErr    error
		wantCalls  []string
	}{
		{name: "recovery failure", recoverErr: recoveryErr, wantErr: recoveryErr, wantCalls: []string{"recover"}},
		{name: "commit failure", commitErr: commitErr, wantLSN: 12, wantErr: commitErr, wantCalls: []string{"recover", "commit"}},
		{name: "success", wantLSN: 12, wantCalls: []string{"recover", "commit"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction := &fakeRestartTransaction{recoverErr: test.recoverErr, commitErr: test.commitErr}
			gotLSN, err := finishRestartRecovery(transaction)
			if gotLSN != test.wantLSN || !errors.Is(err, test.wantErr) {
				t.Fatalf("finishRestartRecovery() = (%d, %v), want (%d, %v)", gotLSN, err, test.wantLSN, test.wantErr)
			}
			if test.wantErr != nil && !strings.Contains(err.Error(), "restart") {
				t.Fatalf("finishRestartRecovery() error = %v, want restart context", err)
			}
			if len(transaction.calls) != len(test.wantCalls) {
				t.Fatalf("calls = %v, want %v", transaction.calls, test.wantCalls)
			}
			for index, want := range test.wantCalls {
				if transaction.calls[index] != want {
					t.Errorf("calls[%d] = %q, want %q", index, transaction.calls[index], want)
				}
			}
		})
	}
}

type fakeRestartTransaction struct {
	calls      []string
	recoverErr error
	commitErr  error
}

func (t *fakeRestartTransaction) Recover() (logmanager.LSN, error) {
	t.calls = append(t.calls, "recover")
	if t.recoverErr != nil {
		return 0, t.recoverErr
	}
	return 12, nil
}

func (t *fakeRestartTransaction) Commit() (logmanager.LSN, error) {
	t.calls = append(t.calls, "commit")
	if t.commitErr != nil {
		return 0, t.commitErr
	}
	return 13, nil
}

func assertRecoveryCheckpointBeforeCommit(t *testing.T, logs *logmanager.Manager) {
	t.Helper()
	iterator, err := logs.Iterator()
	if err != nil {
		t.Fatalf("Iterator() error = %v", err)
	}
	for index, want := range []recovery.Operation{recovery.OperationCommit, recovery.OperationCheckpoint} {
		encoded, ok, err := iterator.Next()
		if err != nil || !ok {
			t.Fatalf("Next() record %d = (%t, %v), want record", index, ok, err)
		}
		record, err := recovery.DecodeRecord(encoded)
		if err != nil {
			t.Fatalf("DecodeRecord() record %d error = %v", index, err)
		}
		if got := record.Operation(); got != want {
			t.Errorf("record %d operation = %d, want %d", index, got, want)
		}
	}
}
