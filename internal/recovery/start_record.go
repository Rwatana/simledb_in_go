package recovery

import (
	"encoding/binary"
	"fmt"
)

const encodedStartRecordSize = 2 * encodedOperationSize

// TransactionID identifies a transaction in a recovery log record.
// Its four-byte width is part of the on-disk record format.
type TransactionID int32

const (
	// OperationStart identifies a transaction start record.
	OperationStart Operation = 1
)

// StartRecord marks the beginning of one transaction's recovery history.
type StartRecord struct {
	transaction TransactionID
}

// Operation returns OperationStart.
func (StartRecord) Operation() Operation {
	return OperationStart
}

// TransactionID returns the transaction stored in the record.
func (r StartRecord) TransactionID() TransactionID {
	return r.transaction
}

// EncodeStartRecord returns the start record's eight-byte on-disk form.
func EncodeStartRecord(transaction TransactionID) []byte {
	encoded := make([]byte, encodedStartRecordSize)
	binary.BigEndian.PutUint32(encoded[:encodedOperationSize], uint32(OperationStart))
	binary.BigEndian.PutUint32(encoded[encodedOperationSize:], uint32(transaction))
	return encoded
}

// DecodeStartRecord interprets an exact start record encoding.
func DecodeStartRecord(encoded []byte) (StartRecord, error) {
	if len(encoded) != encodedStartRecordSize {
		return StartRecord{}, fmt.Errorf("decode start record: expected exactly %d bytes, got %d", encodedStartRecordSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded[:encodedOperationSize])))
	if operation != OperationStart {
		return StartRecord{}, fmt.Errorf("decode start record: operation %d is not start", operation)
	}
	transaction := TransactionID(int32(binary.BigEndian.Uint32(encoded[encodedOperationSize:])))
	return StartRecord{transaction: transaction}, nil
}
