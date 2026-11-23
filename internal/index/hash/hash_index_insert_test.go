package hash

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type hashIndexInsertTransaction struct {
	sizes          map[string]int
	defaultSize    int
	blockSize      int
	integers       map[int]int32
	strings        map[int]string
	sizeErr        error
	appendErr      error
	pinErr         error
	unpinErr       error
	blockSizeErr   error
	getIntErrAt    int
	getIntErr      error
	setIntErrAt    int
	setIntErr      error
	getStringErrAt int
	getStringErr   error
	setStringErrAt int
	setStringErr   error
	events         []string
	sizeFiles      []string
	appendFiles    []string
	pinnedBlocks   []storage.BlockID
	unpinnedBlocks []storage.BlockID
	intWrites      []hashIndexInsertIntWrite
	stringWrites   []hashIndexInsertStringWrite
}

type hashIndexInsertIntWrite struct {
	block   storage.BlockID
	offset  int32
	value   int32
	logging bool
}

type hashIndexInsertStringWrite struct {
	block   storage.BlockID
	offset  int32
	value   string
	logging bool
}

func (transaction *hashIndexInsertTransaction) Pin(block storage.BlockID) error {
	transaction.events = append(transaction.events, "pin:"+block.String())
	transaction.pinnedBlocks = append(transaction.pinnedBlocks, block)
	return transaction.pinErr
}

func (transaction *hashIndexInsertTransaction) BlockSize() (int, error) {
	transaction.events = append(transaction.events, "block-size")
	return transaction.blockSize, transaction.blockSizeErr
}

func (transaction *hashIndexInsertTransaction) GetInt(_ storage.BlockID, offset int) (int32, error) {
	transaction.events = append(transaction.events, fmt.Sprintf("get-int:%d", offset))
	if offset == transaction.getIntErrAt && transaction.getIntErr != nil {
		return 0, transaction.getIntErr
	}
	return transaction.integers[offset], nil
}

func (transaction *hashIndexInsertTransaction) GetString(_ storage.BlockID, offset int) (string, error) {
	transaction.events = append(transaction.events, fmt.Sprintf("get-string:%d", offset))
	if offset == transaction.getStringErrAt && transaction.getStringErr != nil {
		return "", transaction.getStringErr
	}
	return transaction.strings[offset], nil
}

func (transaction *hashIndexInsertTransaction) SetInt(block storage.BlockID, offset, value int32, logging bool) error {
	transaction.events = append(transaction.events, fmt.Sprintf("set-int:%d=%d", offset, value))
	transaction.intWrites = append(transaction.intWrites, hashIndexInsertIntWrite{block: block, offset: offset, value: value, logging: logging})
	if int(offset) == transaction.setIntErrAt && transaction.setIntErr != nil {
		return transaction.setIntErr
	}
	transaction.integers[int(offset)] = value
	return nil
}

func (transaction *hashIndexInsertTransaction) SetString(block storage.BlockID, offset int32, value string, logging bool) error {
	transaction.events = append(transaction.events, fmt.Sprintf("set-string:%d=%s", offset, value))
	transaction.stringWrites = append(transaction.stringWrites, hashIndexInsertStringWrite{block: block, offset: offset, value: value, logging: logging})
	if int(offset) == transaction.setStringErrAt && transaction.setStringErr != nil {
		return transaction.setStringErr
	}
	transaction.strings[int(offset)] = value
	return nil
}

func (transaction *hashIndexInsertTransaction) Size(fileName string) (int, error) {
	transaction.events = append(transaction.events, "size:"+fileName)
	transaction.sizeFiles = append(transaction.sizeFiles, fileName)
	if transaction.sizeErr != nil {
		return 0, transaction.sizeErr
	}
	if size, ok := transaction.sizes[fileName]; ok {
		return size, nil
	}
	return transaction.defaultSize, nil
}

func (transaction *hashIndexInsertTransaction) Append(fileName string) (storage.BlockID, error) {
	transaction.events = append(transaction.events, "append:"+fileName)
	transaction.appendFiles = append(transaction.appendFiles, fileName)
	if transaction.appendErr != nil {
		return storage.BlockID{}, transaction.appendErr
	}
	if transaction.sizes == nil {
		transaction.sizes = make(map[string]int)
	}
	transaction.sizes[fileName] = 1
	return storage.NewBlockID(fileName, 0), nil
}

func (transaction *hashIndexInsertTransaction) Unpin(block storage.BlockID) error {
	transaction.events = append(transaction.events, "unpin:"+block.String())
	transaction.unpinnedBlocks = append(transaction.unpinnedBlocks, block)
	return transaction.unpinErr
}

func newHashIndexInsertTransaction(layout *record.Layout, tableSize int) *hashIndexInsertTransaction {
	return &hashIndexInsertTransaction{
		defaultSize:    tableSize,
		blockSize:      layout.SlotSize() * 2,
		integers:       make(map[int]int32),
		strings:        make(map[int]string),
		getIntErrAt:    -1,
		setIntErrAt:    -1,
		getStringErrAt: -1,
		setStringErrAt: -1,
	}
}

func hashIndexInsertLayout(dataType record.FieldType) *record.Layout {
	schema := &record.Schema{}
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	schema.AddField("dataval", dataType, 12)
	return record.NewLayout(schema)
}

func TestHashIndexInsertWritesIntegerEntryAndRetainsBucketScan(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	transaction := newHashIndexInsertTransaction(layout, 1)
	index := NewHashIndex(transaction, "student_id", layout)
	key := query.NewIntConstant(42)
	rid := record.NewRID(7, 3)

	if err := index.Insert(key, rid); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	wantFile := "student_id42.tbl"
	if len(transaction.sizeFiles) != 1 || transaction.sizeFiles[0] != wantFile {
		t.Errorf("bucket table size calls = %v, want [%s]", transaction.sizeFiles, wantFile)
	}
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	valueOffset, _ := layout.Offset("dataval")
	if transaction.integers[blockOffset] != 7 || transaction.integers[idOffset] != 3 || transaction.integers[valueOffset] != 42 {
		t.Errorf("written index values = %v, want block=7 id=3 dataval=42", transaction.integers)
	}
	if !index.searchKey.Equal(key) || index.scan == nil {
		t.Fatalf("state after Insert() = (key %v, scan %v), want key retained and scan open", index.searchKey, index.scan)
	}
	if got, err := index.GetDataRID(); err != nil || !got.Equal(rid) {
		t.Errorf("GetDataRID() after Insert() = (%v, %v), want %v", got, err, rid)
	}
	if len(transaction.unpinnedBlocks) != 0 {
		t.Errorf("Insert() unpinned blocks %v, want the opened scan to remain active", transaction.unpinnedBlocks)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close() after Insert() error = %v", err)
	}
	if len(transaction.unpinnedBlocks) != 1 || transaction.unpinnedBlocks[0].FileName() != wantFile {
		t.Errorf("unpins after Close() = %v, want the opened hash bucket block", transaction.unpinnedBlocks)
	}
}

func TestHashIndexInsertCreatesVarcharBucketTable(t *testing.T) {
	layout := hashIndexInsertLayout(record.VarcharField)
	transaction := newHashIndexInsertTransaction(layout, 0)
	index := NewHashIndex(transaction, "student_name", layout)
	key := query.NewStringConstant("abc")
	rid := record.NewRID(9, 5)

	if err := index.Insert(key, rid); err != nil {
		t.Fatalf("Insert() into new table error = %v", err)
	}
	bucket, _ := BucketNumber(key)
	wantFile := fmt.Sprintf("student_name%d.tbl", bucket)
	if len(transaction.appendFiles) != 1 || transaction.appendFiles[0] != wantFile {
		t.Fatalf("append calls = %v, want [%s]", transaction.appendFiles, wantFile)
	}
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	valueOffset, _ := layout.Offset("dataval")
	if transaction.integers[blockOffset] != 9 || transaction.integers[idOffset] != 5 || transaction.strings[valueOffset] != "abc" {
		t.Errorf("new varchar bucket row = ints %v strings %v, want block=9 id=5 dataval=abc", transaction.integers, transaction.strings)
	}
	if index.scan == nil || !index.searchKey.Equal(key) {
		t.Errorf("state after varchar Insert() = (key %v, scan %v), want key retained and scan open", index.searchKey, index.scan)
	}
}

func TestHashIndexInsertRejectsInvalidInputsBeforeTableIO(t *testing.T) {
	validLayout := hashIndexInsertLayout(record.IntegerField)
	validKey := query.NewIntConstant(4)
	transaction := newHashIndexInsertTransaction(validLayout, 1)
	wrongTypeLayout := hashIndexInsertLayout(record.VarcharField)
	validSchema := validLayout.Schema()
	tests := []struct {
		name  string
		index *HashIndex
		key   query.Constant
	}{
		{name: "nil receiver", key: validKey},
		{name: "nil transaction", index: NewHashIndex(nil, "idx", validLayout), key: validKey},
		{name: "nil layout", index: NewHashIndex(transaction, "idx", nil), key: validKey},
		{name: "nil schema", index: NewHashIndex(transaction, "idx", record.NewLayoutFromMetadata(nil, map[string]int{"block": 4, "id": 8, "dataval": 12}, 16)), key: validKey},
		{name: "zero slot size", index: NewHashIndex(transaction, "idx", record.NewLayoutFromMetadata(validSchema, map[string]int{"block": 4, "id": 8, "dataval": 12}, 0)), key: validKey},
		{name: "missing block", index: NewHashIndex(transaction, "idx", hashIndexInsertSingleFieldLayout("id", "dataval")), key: validKey},
		{name: "missing id", index: NewHashIndex(transaction, "idx", hashIndexInsertSingleFieldLayout("block", "dataval")), key: validKey},
		{name: "missing dataval", index: NewHashIndex(transaction, "idx", hashIndexInsertSingleFieldLayout("block", "id")), key: validKey},
		{name: "missing block offset", index: NewHashIndex(transaction, "idx", record.NewLayoutFromMetadata(validSchema, map[string]int{"id": 8, "dataval": 12}, 16)), key: validKey},
		{name: "missing id offset", index: NewHashIndex(transaction, "idx", record.NewLayoutFromMetadata(validSchema, map[string]int{"block": 4, "dataval": 12}, 16)), key: validKey},
		{name: "missing dataval offset", index: NewHashIndex(transaction, "idx", record.NewLayoutFromMetadata(validSchema, map[string]int{"block": 4, "id": 8}, 16)), key: validKey},
		{name: "varchar block", index: NewHashIndex(transaction, "idx", hashIndexInsertTypedLayout(record.VarcharField, record.IntegerField, record.IntegerField)), key: validKey},
		{name: "varchar id", index: NewHashIndex(transaction, "idx", hashIndexInsertTypedLayout(record.IntegerField, record.VarcharField, record.IntegerField)), key: validKey},
		{name: "unsupported dataval", index: NewHashIndex(transaction, "idx", hashIndexInsertTypedLayout(record.IntegerField, record.IntegerField, record.FieldType(255))), key: validKey},
		{name: "value does not match dataval type", index: NewHashIndex(transaction, "idx", wrongTypeLayout), key: validKey},
		{name: "unset value", index: NewHashIndex(transaction, "idx", validLayout), key: query.Constant{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beforeCalls := len(transaction.events)
			if err := test.index.Insert(test.key, record.NewRID(1, 0)); err == nil || !strings.Contains(err.Error(), "hash index") {
				t.Fatalf("Insert() error = %v, want contextual validation error", err)
			}
			if len(transaction.events) != beforeCalls {
				t.Errorf("invalid Insert() made transaction calls %v, want no additional I/O", transaction.events[beforeCalls:])
			}
		})
	}

	for _, test := range []struct {
		name string
		rid  record.RID
	}{
		{name: "block exceeds signed int32", rid: record.NewRID(int64ToInt(1<<31), 0)},
		{name: "slot exceeds signed int32", rid: record.NewRID(0, int64ToInt(1<<31))},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := NewHashIndex(transaction, "idx", validLayout)
			beforeCalls := len(transaction.events)
			if err := index.Insert(validKey, test.rid); err == nil || !strings.Contains(err.Error(), "hash index") {
				t.Fatalf("Insert() error = %v, want contextual RID range error", err)
			}
			if len(transaction.events) != beforeCalls {
				t.Errorf("out-of-range RID made transaction calls %v, want no I/O", transaction.events[beforeCalls:])
			}
		})
	}
}

func int64ToInt(value int64) int { return int(value) }

func hashIndexInsertSingleFieldLayout(first, second string) *record.Layout {
	schema := &record.Schema{}
	schema.AddIntegerField(first)
	schema.AddIntegerField(second)
	return record.NewLayout(schema)
}

func hashIndexInsertTypedLayout(blockType, idType, valueType record.FieldType) *record.Layout {
	schema := &record.Schema{}
	schema.AddField("block", blockType, 2)
	schema.AddField("id", idType, 2)
	schema.AddField("dataval", valueType, 12)
	return record.NewLayout(schema)
}

func TestHashIndexInsertWrapsBucketOpenAndReplacementCloseErrors(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	sizeErr := errors.New("bucket size failed")
	appendErr := errors.New("bucket append failed")
	pinErr := errors.New("bucket pin failed")
	tests := []struct {
		name        string
		transaction *hashIndexInsertTransaction
		cause       error
	}{
		{name: "size", transaction: newHashIndexInsertTransaction(layout, 1), cause: sizeErr},
		{name: "pin", transaction: newHashIndexInsertTransaction(layout, 1), cause: pinErr},
		{name: "append", transaction: newHashIndexInsertTransaction(layout, 0), cause: appendErr},
	}
	tests[0].transaction.sizeErr = sizeErr
	tests[1].transaction.pinErr = pinErr
	tests[2].transaction.appendErr = appendErr
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := NewHashIndex(test.transaction, "students", layout)
			err := index.Insert(query.NewIntConstant(1), record.NewRID(0, 0))
			if !errors.Is(err, test.cause) || !strings.Contains(err.Error(), "students") {
				t.Fatalf("Insert() open error = %v, want wrapped bucket cause", err)
			}
			if index.scan != nil {
				t.Errorf("scan after bucket open failure = %v, want nil", index.scan)
			}
			if _, initialized := index.searchKey.HashCode(); initialized {
				t.Errorf("search key after bucket open failure = %v, want unset", index.searchKey)
			}
		})
	}

	unpinErr := errors.New("old scan unpin failed")
	transaction := newHashIndexInsertTransaction(layout, 1)
	transaction.unpinErr = unpinErr
	oldScan, err := record.NewTableScan(transaction, "old_bucket", layout)
	if err != nil {
		t.Fatalf("NewTableScan() old bucket error = %v", err)
	}
	index := NewHashIndex(transaction, "students", layout)
	index.searchKey = query.NewIntConstant(7)
	index.scan = oldScan
	if err := index.Insert(query.NewIntConstant(2), record.NewRID(0, 0)); !errors.Is(err, unpinErr) || index.scan != oldScan || !index.searchKey.Equal(query.NewIntConstant(7)) {
		t.Fatalf("Insert() with old-scan close failure = %v; state (key %v, scan %v), want wrapped error and old state", err, index.searchKey, index.scan)
	}
}

func TestHashIndexInsertWrapsSlotAllocationAndFieldWriteErrors(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	valueOffset, _ := layout.Offset("dataval")
	tests := []struct {
		name      string
		configure func(*hashIndexInsertTransaction, error)
	}{
		{name: "block size", configure: func(tx *hashIndexInsertTransaction, err error) { tx.blockSizeErr = err }},
		{name: "slot flag read", configure: func(tx *hashIndexInsertTransaction, err error) { tx.getIntErrAt = 0; tx.getIntErr = err }},
		{name: "slot occupancy write", configure: func(tx *hashIndexInsertTransaction, err error) { tx.setIntErrAt = 0; tx.setIntErr = err }},
		{name: "block write", configure: func(tx *hashIndexInsertTransaction, err error) { tx.setIntErrAt = blockOffset; tx.setIntErr = err }},
		{name: "id write", configure: func(tx *hashIndexInsertTransaction, err error) { tx.setIntErrAt = idOffset; tx.setIntErr = err }},
		{name: "dataval write", configure: func(tx *hashIndexInsertTransaction, err error) { tx.setIntErrAt = valueOffset; tx.setIntErr = err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeErr := errors.New(test.name + " failed")
			transaction := newHashIndexInsertTransaction(layout, 1)
			test.configure(transaction, writeErr)
			index := NewHashIndex(transaction, "students", layout)
			err := index.Insert(query.NewIntConstant(1), record.NewRID(2, 3))
			if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "students") {
				t.Fatalf("Insert() error = %v, want wrapped %s cause", err, test.name)
			}
			if index.scan == nil || !index.searchKey.Equal(query.NewIntConstant(1)) {
				t.Errorf("state after failed write = (key %v, scan %v), want positioned scan retained", index.searchKey, index.scan)
			}
		})
	}
}

func TestHashIndexInsertWrapsVarcharWriteFailure(t *testing.T) {
	layout := hashIndexInsertLayout(record.VarcharField)
	valueOffset, _ := layout.Offset("dataval")
	writeErr := errors.New("varchar write failed")
	transaction := newHashIndexInsertTransaction(layout, 1)
	transaction.setStringErrAt = valueOffset
	transaction.setStringErr = writeErr
	index := NewHashIndex(transaction, "student_name", layout)

	err := index.Insert(query.NewStringConstant("abc"), record.NewRID(2, 3))
	if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "student_name") {
		t.Fatalf("Insert() varchar error = %v, want wrapped cause", err)
	}
	if index.scan == nil || !index.searchKey.Equal(query.NewStringConstant("abc")) {
		t.Errorf("state after failed varchar write = (key %v, scan %v), want positioned scan retained", index.searchKey, index.scan)
	}
}
