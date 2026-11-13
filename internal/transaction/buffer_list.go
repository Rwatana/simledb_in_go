// Package transaction composes transaction-local resource ownership.
package transaction

import (
	"errors"
	"fmt"

	"github.com/Rwatana/simledb_in_go/internal/buffer"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

// ErrBufferNotPinned reports that a transaction tried to release a block it does not own.
var ErrBufferNotPinned = errors.New("transaction buffer is not pinned")

// BufferPinner acquires and releases shared buffer frames for disk blocks.
type BufferPinner interface {
	Pin(block storage.BlockID) (*buffer.Buffer, error)
	Unpin(frame *buffer.Buffer) error
}

// BufferList tracks the buffer pins owned by one transaction.
type BufferList struct {
	pinner  BufferPinner
	buffers map[storage.BlockID]*buffer.Buffer
	pins    []storage.BlockID
}

// NewBufferList creates empty transaction-local buffer ownership backed by pinner.
func NewBufferList(pinner BufferPinner) *BufferList {
	return &BufferList{
		pinner:  pinner,
		buffers: make(map[storage.BlockID]*buffer.Buffer),
	}
}

// Pin acquires block and records its buffer and one local pin occurrence after success.
func (l *BufferList) Pin(block storage.BlockID) error {
	frame, err := l.pinner.Pin(block)
	if err != nil {
		return fmt.Errorf("pin transaction buffer %v: %w", block, err)
	}
	l.buffers[block] = frame
	l.pins = append(l.pins, block)
	return nil
}

// Buffer returns the buffer currently tracked for block.
func (l *BufferList) Buffer(block storage.BlockID) (*buffer.Buffer, bool) {
	frame, ok := l.buffers[block]
	return frame, ok
}

// Unpin releases one local pin occurrence for block while retaining any remaining ownership.
func (l *BufferList) Unpin(block storage.BlockID) error {
	frame, ok := l.buffers[block]
	if !ok {
		return fmt.Errorf("unpin transaction buffer %v: %w", block, ErrBufferNotPinned)
	}
	if err := l.pinner.Unpin(frame); err != nil {
		return fmt.Errorf("unpin transaction buffer %v: %w", block, err)
	}
	for index, pinned := range l.pins {
		if pinned == block {
			l.pins = append(l.pins[:index], l.pins[index+1:]...)
			break
		}
	}
	for _, pinned := range l.pins {
		if pinned == block {
			return nil
		}
	}
	delete(l.buffers, block)
	return nil
}
