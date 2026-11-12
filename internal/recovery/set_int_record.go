package recovery

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

const (
	setIntFixedSize = 24

	// OperationSetInt identifies an integer update recovery record.
	OperationSetInt Operation = 4
)

// SetIntRecord stores the previous integer value needed to undo one update.
type SetIntRecord struct {
	transaction TransactionID
	block       storage.BlockID
	offset      int32
	oldValue    int32
}

// Operation returns OperationSetInt.
func (SetIntRecord) Operation() Operation { return OperationSetInt }

// TransactionID returns the transaction that performed the update.
func (r SetIntRecord) TransactionID() TransactionID { return r.transaction }

// Block returns the updated block.
func (r SetIntRecord) Block() storage.BlockID { return r.block }

// Offset returns the byte offset of the updated integer in the page.
func (r SetIntRecord) Offset() int32 { return r.offset }

// OldValue returns the integer value stored before the update.
func (r SetIntRecord) OldValue() int32 { return r.oldValue }

// EncodeSetIntRecord returns the SETINT record's variable-length on-disk form.
func EncodeSetIntRecord(transaction TransactionID, block storage.BlockID, offset, oldValue int32) ([]byte, error) {
	fileName := []byte(block.FileName())
	if len(fileName) > math.MaxInt32 {
		return nil, fmt.Errorf("encode setint record: filename length %d exceeds int32", len(fileName))
	}
	maxInt := int(^uint(0) >> 1)
	if len(fileName) > maxInt-setIntFixedSize {
		return nil, fmt.Errorf("encode setint record: filename length %d overflows record size", len(fileName))
	}
	if block.Number() < math.MinInt32 || block.Number() > math.MaxInt32 {
		return nil, fmt.Errorf("encode setint record: block number %d exceeds int32", block.Number())
	}
	encoded := make([]byte, setIntFixedSize+len(fileName))
	binary.BigEndian.PutUint32(encoded[0:4], uint32(OperationSetInt))
	binary.BigEndian.PutUint32(encoded[4:8], uint32(transaction))
	binary.BigEndian.PutUint32(encoded[8:12], uint32(len(fileName)))
	copy(encoded[12:], fileName)
	fixed := 12 + len(fileName)
	binary.BigEndian.PutUint32(encoded[fixed:fixed+4], uint32(int32(block.Number())))
	binary.BigEndian.PutUint32(encoded[fixed+4:fixed+8], uint32(offset))
	binary.BigEndian.PutUint32(encoded[fixed+8:fixed+12], uint32(oldValue))
	return encoded, nil
}

// DecodeSetIntRecord interprets an exact SETINT record encoding.
func DecodeSetIntRecord(encoded []byte) (SetIntRecord, error) {
	if len(encoded) < setIntFixedSize {
		return SetIntRecord{}, fmt.Errorf("decode setint record: expected at least %d bytes, got %d", setIntFixedSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded[0:4])))
	if operation != OperationSetInt {
		return SetIntRecord{}, fmt.Errorf("decode setint record: operation %d is not setint", operation)
	}
	nameLength := int32(binary.BigEndian.Uint32(encoded[8:12]))
	if nameLength < 0 {
		return SetIntRecord{}, fmt.Errorf("decode setint record: negative filename length %d", nameLength)
	}
	if int(nameLength) > len(encoded)-setIntFixedSize {
		return SetIntRecord{}, fmt.Errorf("decode setint record: record length %d does not contain filename length %d and fixed fields", len(encoded), nameLength)
	}
	expected := setIntFixedSize + int(nameLength)
	if len(encoded) != expected {
		return SetIntRecord{}, fmt.Errorf("decode setint record: record length %d, want %d", len(encoded), expected)
	}
	fixed := 12 + int(nameLength)
	transaction := TransactionID(int32(binary.BigEndian.Uint32(encoded[4:8])))
	blockNumber := int32(binary.BigEndian.Uint32(encoded[fixed : fixed+4]))
	offset := int32(binary.BigEndian.Uint32(encoded[fixed+4 : fixed+8]))
	oldValue := int32(binary.BigEndian.Uint32(encoded[fixed+8 : fixed+12]))
	return SetIntRecord{
		transaction: transaction,
		block:       storage.NewBlockID(string(encoded[12:fixed]), int(blockNumber)),
		offset:      offset,
		oldValue:    oldValue,
	}, nil
}
