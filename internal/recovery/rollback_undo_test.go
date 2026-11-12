package recovery

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type sliceRollbackRecordSource struct {
	records []Record
	next    int
	err     error
}

func (s *sliceRollbackRecordSource) Next() (Record, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	if s.next == len(s.records) {
		return nil, false, nil
	}
	record := s.records[s.next]
	s.next++
	return record, true, nil
}

type recordingRollbackUndoTarget struct {
	events       []string
	setStringErr error
}

func (t *recordingRollbackUndoTarget) Pin(block storage.BlockID) error {
	t.events = append(t.events, "pin:"+block.String())
	return nil
}

func (t *recordingRollbackUndoTarget) Unpin(block storage.BlockID) error {
	t.events = append(t.events, "unpin:"+block.String())
	return nil
}

func (t *recordingRollbackUndoTarget) SetInt(_ storage.BlockID, _ int32, value int32, logUpdate bool) error {
	t.events = append(t.events, "setint:"+strconv.FormatInt(int64(value), 10)+":"+boolText(logUpdate))
	return nil
}

func (t *recordingRollbackUndoTarget) SetString(_ storage.BlockID, _ int32, value string, logUpdate bool) error {
	t.events = append(t.events, "setstring:"+value+":"+boolText(logUpdate))
	return t.setStringErr
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func TestUndoRollbackRecordsRestoresUpdatesInSourceOrder(t *testing.T) {
	block := storage.NewBlockID("tbl", 2)
	source := &sliceRollbackRecordSource{records: []Record{
		SetStringRecord{transaction: 7, block: block, offset: 16, oldValue: "以前"},
		CommitRecord{transaction: 7},
		SetIntRecord{transaction: 7, block: block, offset: 12, oldValue: 4},
	}}
	target := &recordingRollbackUndoTarget{}

	if err := UndoRollbackRecords(source, target); err != nil {
		t.Fatalf("UndoRollbackRecords() error = %v", err)
	}
	want := []string{
		"pin:" + block.String(), "setstring:以前:false", "unpin:" + block.String(),
		"pin:" + block.String(), "setint:4:false", "unpin:" + block.String(),
	}
	if !reflect.DeepEqual(target.events, want) {
		t.Fatalf("events = %#v, want %#v", target.events, want)
	}
}

func TestUndoRollbackRecordsReturnsSourceFailure(t *testing.T) {
	want := errors.New("source failed")
	err := UndoRollbackRecords(&sliceRollbackRecordSource{err: want}, &recordingRollbackUndoTarget{})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "next") {
		t.Fatalf("error = %v", err)
	}
}

func TestUndoRollbackRecordsStopsAfterUndoFailure(t *testing.T) {
	want := errors.New("restore failed")
	block := storage.NewBlockID("tbl", 2)
	source := &sliceRollbackRecordSource{records: []Record{
		SetStringRecord{transaction: 7, block: block, oldValue: "以前"},
		SetIntRecord{transaction: 7, block: block, oldValue: 4},
	}}
	target := &recordingRollbackUndoTarget{setStringErr: want}

	err := UndoRollbackRecords(source, target)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "SETSTRING") || source.next != 1 {
		t.Fatalf("error = %v source.next = %d", err, source.next)
	}
	if !reflect.DeepEqual(target.events, []string{"pin:" + block.String(), "setstring:以前:false", "unpin:" + block.String()}) {
		t.Fatalf("events = %v", target.events)
	}
}
