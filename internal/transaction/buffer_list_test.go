package transaction

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingBufferPinner struct {
	frame    *buffer.Buffer
	frames   map[storage.BlockID]*buffer.Buffer
	err      error
	unpinErr error
	blocks   []storage.BlockID
	unpins   []*buffer.Buffer
	flushes  []buffer.TransactionID
	flushErr error
	events   *[]string
}

func (p *recordingBufferPinner) Pin(block storage.BlockID) (*buffer.Buffer, error) {
	p.blocks = append(p.blocks, block)
	if p.frames != nil {
		return p.frames[block], p.err
	}
	return p.frame, p.err
}

func (p *recordingBufferPinner) Unpin(frame *buffer.Buffer) error {
	p.unpins = append(p.unpins, frame)
	return p.unpinErr
}

func (p *recordingBufferPinner) FlushAll(transaction buffer.TransactionID) error {
	p.flushes = append(p.flushes, transaction)
	if p.events != nil {
		*p.events = append(*p.events, "flush buffers")
	}
	return p.flushErr
}

func TestBufferListPinsAndTracksBlock(t *testing.T) {
	frame := &buffer.Buffer{}
	pinner := &recordingBufferPinner{frame: frame}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)
	other := storage.NewBlockID("tbl", 2)

	if err := list.Pin(block); err != nil {
		t.Fatal(err)
	}

	if len(pinner.blocks) != 1 || pinner.blocks[0] != block {
		t.Fatalf("pin requests = %v, want [%v]", pinner.blocks, block)
	}
	if got, ok := list.Buffer(block); !ok || got != frame {
		t.Fatalf("Buffer(%v) = (%p, %v), want (%p, true)", block, got, ok, frame)
	}
	if _, ok := list.Buffer(other); ok {
		t.Fatalf("Buffer(%v) unexpectedly found", other)
	}
	if len(list.pins) != 1 || list.pins[0] != block {
		t.Fatalf("local pin occurrences = %v, want [%v]", list.pins, block)
	}
}

func TestBufferListDoesNotTrackFailedPin(t *testing.T) {
	wantErr := errors.New("pin failed")
	pinner := &recordingBufferPinner{err: wantErr}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)

	if err := list.Pin(block); !errors.Is(err, wantErr) {
		t.Fatalf("Pin() error = %v, want %v", err, wantErr)
	}
	if _, ok := list.Buffer(block); ok {
		t.Fatal("failed pin was tracked")
	}
	if len(list.pins) != 0 {
		t.Fatalf("pin occurrences after failure = %v, want none", list.pins)
	}
}

func TestBufferListReleasesOneOfRepeatedPins(t *testing.T) {
	frame := &buffer.Buffer{}
	pinner := &recordingBufferPinner{frame: frame}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)

	if err := list.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := list.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := list.Unpin(block); err != nil {
		t.Fatal(err)
	}

	if len(pinner.unpins) != 1 || pinner.unpins[0] != frame {
		t.Fatalf("unpin requests = %v, want [%p]", pinner.unpins, frame)
	}
	if got, ok := list.Buffer(block); !ok || got != frame {
		t.Fatalf("Buffer(%v) after partial unpin = (%p, %v), want (%p, true)", block, got, ok, frame)
	}
	if len(list.pins) != 1 || list.pins[0] != block {
		t.Fatalf("pin occurrences after partial unpin = %v, want [%v]", list.pins, block)
	}
}

func TestBufferListRejectsUnpinForMissingBlock(t *testing.T) {
	pinner := &recordingBufferPinner{}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)

	if err := list.Unpin(block); !errors.Is(err, ErrBufferNotPinned) {
		t.Fatalf("Unpin() error = %v, want ErrBufferNotPinned", err)
	}
	if len(pinner.unpins) != 0 {
		t.Fatalf("unpin requests = %v, want none", pinner.unpins)
	}
}

func TestBufferListRemovesBufferAfterFinalUnpin(t *testing.T) {
	frame := &buffer.Buffer{}
	pinner := &recordingBufferPinner{frame: frame}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)

	if err := list.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := list.Unpin(block); err != nil {
		t.Fatal(err)
	}

	if _, ok := list.Buffer(block); ok {
		t.Fatal("buffer lookup remains after final unpin")
	}
	if len(list.pins) != 0 {
		t.Fatalf("pin occurrences after final unpin = %v, want none", list.pins)
	}
}

func TestBufferListPreservesOwnershipWhenUnpinFails(t *testing.T) {
	wantErr := errors.New("unpin failed")
	frame := &buffer.Buffer{}
	pinner := &recordingBufferPinner{frame: frame, unpinErr: wantErr}
	list := NewBufferList(pinner)
	block := storage.NewBlockID("tbl", 1)

	if err := list.Pin(block); err != nil {
		t.Fatal(err)
	}
	if err := list.Unpin(block); !errors.Is(err, wantErr) {
		t.Fatalf("Unpin() error = %v, want %v", err, wantErr)
	}
	if got, ok := list.Buffer(block); !ok || got != frame {
		t.Fatalf("Buffer(%v) after failed unpin = (%p, %v), want (%p, true)", block, got, ok, frame)
	}
	if len(list.pins) != 1 || list.pins[0] != block {
		t.Fatalf("pin occurrences after failed unpin = %v, want [%v]", list.pins, block)
	}
}

func TestBufferListReleasesAllPinOccurrences(t *testing.T) {
	firstFrame := &buffer.Buffer{}
	secondFrame := &buffer.Buffer{}
	firstBlock := storage.NewBlockID("tbl", 1)
	secondBlock := storage.NewBlockID("tbl", 2)
	pinner := &recordingBufferPinner{frames: map[storage.BlockID]*buffer.Buffer{
		firstBlock:  firstFrame,
		secondBlock: secondFrame,
	}}
	list := NewBufferList(pinner)

	if err := list.Pin(firstBlock); err != nil {
		t.Fatal(err)
	}
	if err := list.Pin(firstBlock); err != nil {
		t.Fatal(err)
	}
	if err := list.Pin(secondBlock); err != nil {
		t.Fatal(err)
	}
	if err := list.UnpinAll(); err != nil {
		t.Fatal(err)
	}
	if err := list.UnpinAll(); err != nil {
		t.Fatal(err)
	}

	wantUnpins := []*buffer.Buffer{firstFrame, firstFrame, secondFrame}
	if len(pinner.unpins) != len(wantUnpins) {
		t.Fatalf("unpin count = %d, want %d", len(pinner.unpins), len(wantUnpins))
	}
	for index, want := range wantUnpins {
		if pinner.unpins[index] != want {
			t.Fatalf("unpin[%d] = %p, want %p", index, pinner.unpins[index], want)
		}
	}
	if len(list.pins) != 0 {
		t.Fatalf("pin occurrences after unpin all = %v, want none", list.pins)
	}
	if _, ok := list.Buffer(firstBlock); ok {
		t.Fatal("first buffer lookup remains after unpin all")
	}
	if _, ok := list.Buffer(secondBlock); ok {
		t.Fatal("second buffer lookup remains after unpin all")
	}
}
