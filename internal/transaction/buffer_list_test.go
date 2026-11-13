package transaction

import (
	"errors"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type recordingBufferPinner struct {
	frame  *buffer.Buffer
	err    error
	blocks []storage.BlockID
	unpins []*buffer.Buffer
}

func (p *recordingBufferPinner) Pin(block storage.BlockID) (*buffer.Buffer, error) {
	p.blocks = append(p.blocks, block)
	return p.frame, p.err
}

func (p *recordingBufferPinner) Unpin(frame *buffer.Buffer) error {
	p.unpins = append(p.unpins, frame)
	return nil
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
