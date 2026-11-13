package recovery

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	logmanager "github.com/Rwatana/simledb_in_go/internal/log"
)

type orderedCommitLog struct {
	events  *[]string
	records [][]byte
	lsn     logmanager.LSN
}

func (l *orderedCommitLog) Append(record []byte) (logmanager.LSN, error) {
	*l.events = append(*l.events, "append commit")
	l.records = append(l.records, append([]byte(nil), record...))
	return l.lsn, nil
}

func (l *orderedCommitLog) Flush(lsn logmanager.LSN) error {
	*l.events = append(*l.events, fmt.Sprintf("flush LSN %d", lsn))
	return nil
}

func TestCommitTransactionFlushesBuffersBeforeDurableMarker(t *testing.T) {
	events := []string{}
	logs := &orderedCommitLog{events: &events, lsn: 73}
	transaction := TransactionID(9)
	flush := func(got TransactionID) error {
		events = append(events, fmt.Sprintf("flush buffers %d", got))
		return nil
	}

	lsn, err := CommitTransaction(flush, logs, transaction)
	if err != nil {
		t.Fatal(err)
	}
	if lsn != 73 {
		t.Fatalf("CommitTransaction() LSN = %d, want 73", lsn)
	}
	wantEvents := []string{"flush buffers 9", "append commit", "flush LSN 73"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("commit events = %v, want %v", events, wantEvents)
	}
	if len(logs.records) != 1 {
		t.Fatalf("COMMIT records = %d, want 1", len(logs.records))
	}
	record, err := DecodeCommitRecord(logs.records[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := record.TransactionID(); got != transaction {
		t.Fatalf("COMMIT transaction = %d, want %d", got, transaction)
	}
}

func TestCommitTransactionStopsAfterBufferFlushFailure(t *testing.T) {
	wantErr := errors.New("buffer flush failed")
	events := []string{}
	logs := &orderedCommitLog{events: &events, lsn: 73}
	flush := func(TransactionID) error {
		events = append(events, "flush buffers")
		return wantErr
	}

	_, err := CommitTransaction(flush, logs, TransactionID(10))
	if !errors.Is(err, wantErr) {
		t.Fatalf("CommitTransaction() error = %v, want %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "transaction 10") {
		t.Fatalf("CommitTransaction() error = %q, want transaction context", err)
	}
	if !reflect.DeepEqual(events, []string{"flush buffers"}) {
		t.Fatalf("commit events = %v, want only buffer flush", events)
	}
}
