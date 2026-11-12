package recovery

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

const (
	setStringFixedSize = 24
	// OperationSetString identifies a string update recovery record.
	OperationSetString Operation = 5
)

// SetStringRecord stores the previous string needed to undo one update.
type SetStringRecord struct {
	transaction TransactionID
	block       storage.BlockID
	offset      int32
	oldValue    string
}

// Operation returns OperationSetString.
func (SetStringRecord) Operation() Operation { return OperationSetString }

// TransactionID returns the transaction that performed the update.
func (r SetStringRecord) TransactionID() TransactionID { return r.transaction }

// Block returns the updated block.
func (r SetStringRecord) Block() storage.BlockID { return r.block }

// Offset returns the byte offset of the updated string in the page.
func (r SetStringRecord) Offset() int32 { return r.offset }

// OldValue returns the string stored before the update.
func (r SetStringRecord) OldValue() string { return r.oldValue }

// EncodeSetStringRecord returns the SETSTRING record's variable-length on-disk form.
func EncodeSetStringRecord(transaction TransactionID, block storage.BlockID, offset int32, oldValue string) ([]byte, error) {
	fileName, oldBytes := []byte(block.FileName()), []byte(oldValue)
	if len(fileName) > math.MaxInt32 || len(oldBytes) > math.MaxInt32 {
		return nil, fmt.Errorf("encode setstring record: string length exceeds int32: filename %d old value %d", len(fileName), len(oldBytes))
	}
	maxInt := int(^uint(0) >> 1)
	if len(fileName) > maxInt-setStringFixedSize || len(oldBytes) > maxInt-setStringFixedSize-len(fileName) {
		return nil, fmt.Errorf("encode setstring record: combined string lengths overflow record size")
	}
	if block.Number() < math.MinInt32 || block.Number() > math.MaxInt32 {
		return nil, fmt.Errorf("encode setstring record: block number %d exceeds int32", block.Number())
	}
	encoded := make([]byte, setStringFixedSize+len(fileName)+len(oldBytes))
	binary.BigEndian.PutUint32(encoded[0:4], uint32(OperationSetString))
	binary.BigEndian.PutUint32(encoded[4:8], uint32(transaction))
	binary.BigEndian.PutUint32(encoded[8:12], uint32(len(fileName)))
	copy(encoded[12:], fileName)
	fixed := 12 + len(fileName)
	binary.BigEndian.PutUint32(encoded[fixed:fixed+4], uint32(int32(block.Number())))
	binary.BigEndian.PutUint32(encoded[fixed+4:fixed+8], uint32(offset))
	binary.BigEndian.PutUint32(encoded[fixed+8:fixed+12], uint32(len(oldBytes)))
	copy(encoded[fixed+12:], oldBytes)
	return encoded, nil
}

// DecodeSetStringRecord interprets an exact SETSTRING record encoding.
func DecodeSetStringRecord(encoded []byte) (SetStringRecord, error) {
	if len(encoded) < setStringFixedSize {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: expected at least %d bytes, got %d", setStringFixedSize, len(encoded))
	}
	operation := Operation(int32(binary.BigEndian.Uint32(encoded[0:4])))
	if operation != OperationSetString {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: operation %d is not setstring", operation)
	}
	fileLength := int32(binary.BigEndian.Uint32(encoded[8:12]))
	if fileLength < 0 {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: negative filename length %d", fileLength)
	}
	if int(fileLength) > len(encoded)-setStringFixedSize {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: record length %d does not contain filename length %d and fixed fields", len(encoded), fileLength)
	}
	fixed := 12 + int(fileLength)
	oldLength := int32(binary.BigEndian.Uint32(encoded[fixed+8 : fixed+12]))
	if oldLength < 0 {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: negative old value length %d", oldLength)
	}
	if int(oldLength) > len(encoded)-(fixed+12) {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: record length %d does not contain old value length %d", len(encoded), oldLength)
	}
	expected := fixed + 12 + int(oldLength)
	if len(encoded) != expected {
		return SetStringRecord{}, fmt.Errorf("decode setstring record: record length %d, want %d", len(encoded), expected)
	}
	return SetStringRecord{
		transaction: TransactionID(int32(binary.BigEndian.Uint32(encoded[4:8]))),
		block:       storage.NewBlockID(string(encoded[12:fixed]), int(int32(binary.BigEndian.Uint32(encoded[fixed:fixed+4])))),
		offset:      int32(binary.BigEndian.Uint32(encoded[fixed+4 : fixed+8])),
		oldValue:    string(encoded[fixed+12:]),
	}, nil
}
