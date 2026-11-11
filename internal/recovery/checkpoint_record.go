// Package recovery encodes and interprets transaction recovery log records.
package recovery

import (
	"encoding/binary"
	"fmt"
)

const encodedOperationSize = 4

// Operation identifies the kind of recovery log record stored on disk.
type Operation int32

const (
	// OperationCheckpoint identifies a quiescent checkpoint record.
	OperationCheckpoint Operation = 0
)

// CheckpointRecord marks a point before which recovery need not scan.
// A quiescent checkpoint has no associated transaction or payload.
type CheckpointRecord struct{}

// Operation returns OperationCheckpoint.
func (CheckpointRecord) Operation() Operation {
	return OperationCheckpoint
}

// EncodeCheckpointRecord returns the checkpoint's four-byte on-disk form.
func EncodeCheckpointRecord() []byte {
	encoded := make([]byte, encodedOperationSize)
	binary.BigEndian.PutUint32(encoded, uint32(OperationCheckpoint))
	return encoded
}

// DecodeCheckpointRecord interprets an exact checkpoint record encoding.
func DecodeCheckpointRecord(encoded []byte) (CheckpointRecord, error) {
	if len(encoded) != encodedOperationSize {
		return CheckpointRecord{}, fmt.Errorf("decode checkpoint record: expected exactly %d bytes, got %d", encodedOperationSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded)))
	if operation != OperationCheckpoint {
		return CheckpointRecord{}, fmt.Errorf("decode checkpoint record: operation %d is not checkpoint", operation)
	}
	return CheckpointRecord{}, nil
}
