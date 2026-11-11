package recovery

import (
	"encoding/binary"
	"fmt"
)

const encodedTransactionRecordSize = 2 * encodedOperationSize

func encodeTransactionRecord(operation Operation, transaction TransactionID) []byte {
	encoded := make([]byte, encodedTransactionRecordSize)
	binary.BigEndian.PutUint32(encoded[:encodedOperationSize], uint32(operation))
	binary.BigEndian.PutUint32(encoded[encodedOperationSize:], uint32(transaction))
	return encoded
}

func decodeTransactionRecord(encoded []byte, expected Operation, name string) (TransactionID, error) {
	if len(encoded) != encodedTransactionRecordSize {
		return 0, fmt.Errorf("decode %s record: expected exactly %d bytes, got %d", name, encodedTransactionRecordSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded[:encodedOperationSize])))
	if operation != expected {
		return 0, fmt.Errorf("decode %s record: operation %d is not %s", name, operation, name)
	}
	return TransactionID(int32(binary.BigEndian.Uint32(encoded[encodedOperationSize:]))), nil
}
