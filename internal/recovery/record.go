package recovery

import (
	"encoding/binary"
	"fmt"
)

// Record is a typed recovery log record.
type Record interface {
	Operation() Operation
}

// DecodeRecord selects a strict record decoder from the operation prefix.
func DecodeRecord(encoded []byte) (Record, error) {
	if len(encoded) < encodedOperationSize {
		return nil, fmt.Errorf("decode recovery record: operation prefix requires %d bytes, got %d", encodedOperationSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded[:encodedOperationSize])))
	switch operation {
	case OperationCheckpoint:
		return DecodeCheckpointRecord(encoded)
	case OperationStart:
		return DecodeStartRecord(encoded)
	case OperationCommit:
		return DecodeCommitRecord(encoded)
	case OperationRollback:
		return DecodeRollbackRecord(encoded)
	case OperationSetInt:
		return DecodeSetIntRecord(encoded)
	case OperationSetString:
		return DecodeSetStringRecord(encoded)
	default:
		return nil, fmt.Errorf("decode recovery record: unknown operation %d", operation)
	}
}
