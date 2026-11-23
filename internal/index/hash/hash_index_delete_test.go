package hash

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func seedHashIndexDeleteRows(layout *record.Layout, rows ...hashIndexDeleteRow) (*hashIndexInsertTransaction, string) {
	transaction := newHashIndexInsertTransaction(layout, 1)
	indexName := "students"
	key := query.NewIntConstant(42)
	bucket, _ := BucketNumber(key)
	tableName := fmt.Sprintf("%s%d.tbl", indexName, bucket)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	valueOffset, _ := layout.Offset("dataval")
	for slot, row := range rows {
		base := slot * layout.SlotSize()
		transaction.integers[base] = 1
		transaction.integers[base+blockOffset] = int32(row.rid.BlockNumber())
		transaction.integers[base+idOffset] = int32(row.rid.Slot())
		if integer, ok := row.value.AsInt(); ok {
			transaction.integers[base+valueOffset] = integer
		}
		if text, ok := row.value.AsString(); ok {
			transaction.strings[base+valueOffset] = text
		}
	}
	return transaction, tableName
}

type hashIndexDeleteRow struct {
	value query.Constant
	rid   record.RID
}

func TestHashIndexDeleteRemovesMatchingRIDAfterCollisions(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	wanted := record.NewRID(7, 3)
	transaction, tableName := seedHashIndexDeleteRows(layout,
		hashIndexDeleteRow{value: query.NewIntConstant(99), rid: record.NewRID(1, 1)},
		hashIndexDeleteRow{value: query.NewIntConstant(42), rid: wanted},
		hashIndexDeleteRow{value: query.NewIntConstant(42), rid: wanted},
	)
	transaction.blockSize = layout.SlotSize() * 3
	index := NewHashIndex(transaction, "students", layout)

	if err := index.Delete(query.NewIntConstant(42), wanted); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if transaction.integers[layout.SlotSize()] != 0 {
		t.Errorf("matching slot flag = %d, want empty", transaction.integers[layout.SlotSize()])
	}
	if transaction.integers[0] != 1 {
		t.Errorf("collision slot flag = %d, want occupied", transaction.integers[0])
	}
	if transaction.integers[2*layout.SlotSize()] != 1 {
		t.Errorf("duplicate RID slot flag = %d, want occupied after deleting only the first match", transaction.integers[2*layout.SlotSize()])
	}
	if !index.searchKey.Equal(query.NewIntConstant(42)) || index.scan == nil {
		t.Errorf("state after Delete() = (key %v, scan %v), want positioned bucket retained", index.searchKey, index.scan)
	}
	if len(transaction.unpinnedBlocks) != 0 {
		t.Errorf("unpins after Delete() = %v, want active scan retained", transaction.unpinnedBlocks)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close() after Delete() from %s error = %v", tableName, err)
	}
}

func TestHashIndexDeleteLeavesNonmatchingEntriesUnchanged(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	transaction, _ := seedHashIndexDeleteRows(layout,
		hashIndexDeleteRow{value: query.NewIntConstant(42), rid: record.NewRID(7, 3)},
		hashIndexDeleteRow{value: query.NewIntConstant(42), rid: record.NewRID(7, 4)},
	)
	index := NewHashIndex(transaction, "students", layout)

	if err := index.Delete(query.NewIntConstant(42), record.NewRID(8, 1)); err != nil {
		t.Fatalf("Delete() with absent RID error = %v, want no-op", err)
	}
	if transaction.integers[0] != 1 || transaction.integers[layout.SlotSize()] != 1 {
		t.Errorf("slot flags after absent RID = (%d, %d), want both occupied", transaction.integers[0], transaction.integers[layout.SlotSize()])
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close() after absent RID error = %v", err)
	}
}

func TestHashIndexDeleteLeavesBucketUnchangedWhenKeyIsAbsent(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	transaction, _ := seedHashIndexDeleteRows(layout,
		hashIndexDeleteRow{value: query.NewIntConstant(99), rid: record.NewRID(7, 3)},
	)
	index := NewHashIndex(transaction, "students", layout)

	if err := index.Delete(query.NewIntConstant(42), record.NewRID(7, 3)); err != nil {
		t.Fatalf("Delete() with absent key error = %v, want no-op", err)
	}
	if transaction.integers[0] != 1 {
		t.Errorf("collision slot flag after absent key = %d, want occupied", transaction.integers[0])
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close() after absent key error = %v", err)
	}
}

func TestHashIndexDeleteWrapsPositionIterationRIDAndDeleteErrors(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	valueOffset, _ := layout.Offset("dataval")
	writeErr := errors.New("record delete failed")
	readErr := errors.New("RID read failed")
	sizeErr := errors.New("bucket size failed")
	pinErr := errors.New("bucket pin failed")
	blockSizeErr := errors.New("block size failed")
	tests := []struct {
		name        string
		transaction *hashIndexInsertTransaction
		key         query.Constant
		rid         record.RID
		cause       error
		wantScan    bool
	}{
		{name: "size", transaction: newHashIndexInsertTransaction(layout, 1), key: query.NewIntConstant(42), cause: sizeErr},
		{name: "pin", transaction: newHashIndexInsertTransaction(layout, 1), key: query.NewIntConstant(42), cause: pinErr},
		{name: "iteration", transaction: newHashIndexInsertTransaction(layout, 1), key: query.NewIntConstant(42), cause: blockSizeErr, wantScan: true},
		{name: "RID read", transaction: newHashIndexInsertTransaction(layout, 1), key: query.NewIntConstant(42), rid: record.NewRID(1, 1), cause: readErr, wantScan: true},
		{name: "delete", transaction: newHashIndexInsertTransaction(layout, 1), key: query.NewIntConstant(42), rid: record.NewRID(1, 1), cause: writeErr, wantScan: true},
	}
	tests[0].transaction.sizeErr = sizeErr
	tests[1].transaction.pinErr = pinErr
	tests[2].transaction.blockSizeErr = blockSizeErr
	tests[3].transaction.integers[0] = 1
	tests[3].transaction.integers[valueOffset] = 42
	tests[3].transaction.getIntErrAt = blockOffset
	tests[3].transaction.getIntErr = readErr
	tests[4].transaction.integers[0] = 1
	tests[4].transaction.integers[blockOffset] = 1
	tests[4].transaction.integers[idOffset] = 1
	tests[4].transaction.integers[valueOffset] = 42
	tests[4].transaction.integers[layout.SlotSize()] = 1
	tests[4].transaction.integers[layout.SlotSize()+blockOffset] = 1
	tests[4].transaction.setIntErrAt = 0
	tests[4].transaction.setIntErr = writeErr
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := NewHashIndex(test.transaction, "students", layout)
			err := index.Delete(test.key, test.rid)
			if !errors.Is(err, test.cause) || !strings.Contains(err.Error(), "students") {
				t.Fatalf("Delete() error = %v, want wrapped cause %v", err, test.cause)
			}
			if (index.scan != nil) != test.wantScan {
				t.Errorf("scan after %s error = %v, want present %t", test.name, index.scan, test.wantScan)
			}
		})
	}
}

func TestHashIndexDeleteRejectsInvalidState(t *testing.T) {
	layout := hashIndexInsertLayout(record.IntegerField)
	transaction := newHashIndexInsertTransaction(layout, 1)
	tests := []struct {
		name  string
		index *HashIndex
		key   query.Constant
	}{
		{name: "nil receiver", key: query.NewIntConstant(42)},
		{name: "nil transaction", index: NewHashIndex(nil, "students", layout), key: query.NewIntConstant(42)},
		{name: "nil layout", index: NewHashIndex(transaction, "students", nil), key: query.NewIntConstant(42)},
		{name: "nil schema", index: NewHashIndex(transaction, "students", record.NewLayoutFromMetadata(nil, map[string]int{"block": 4, "id": 8, "dataval": 12}, 16)), key: query.NewIntConstant(42)},
		{name: "zero slot size", index: NewHashIndex(transaction, "students", record.NewLayoutFromMetadata(layout.Schema(), map[string]int{"block": 4, "id": 8, "dataval": 12}, 0)), key: query.NewIntConstant(42)},
		{name: "missing block", index: NewHashIndex(transaction, "students", hashIndexInsertSingleFieldLayout("id", "dataval")), key: query.NewIntConstant(42)},
		{name: "missing id", index: NewHashIndex(transaction, "students", hashIndexInsertSingleFieldLayout("block", "dataval")), key: query.NewIntConstant(42)},
		{name: "missing dataval", index: NewHashIndex(transaction, "students", hashIndexInsertSingleFieldLayout("block", "id")), key: query.NewIntConstant(42)},
		{name: "missing block offset", index: NewHashIndex(transaction, "students", record.NewLayoutFromMetadata(layout.Schema(), map[string]int{"id": 8, "dataval": 12}, 16)), key: query.NewIntConstant(42)},
		{name: "missing id offset", index: NewHashIndex(transaction, "students", record.NewLayoutFromMetadata(layout.Schema(), map[string]int{"block": 4, "dataval": 12}, 16)), key: query.NewIntConstant(42)},
		{name: "missing dataval offset", index: NewHashIndex(transaction, "students", record.NewLayoutFromMetadata(layout.Schema(), map[string]int{"block": 4, "id": 8}, 16)), key: query.NewIntConstant(42)},
		{name: "varchar block", index: NewHashIndex(transaction, "students", hashIndexInsertTypedLayout(record.VarcharField, record.IntegerField, record.IntegerField)), key: query.NewIntConstant(42)},
		{name: "varchar id", index: NewHashIndex(transaction, "students", hashIndexInsertTypedLayout(record.IntegerField, record.VarcharField, record.IntegerField)), key: query.NewIntConstant(42)},
		{name: "unsupported dataval", index: NewHashIndex(transaction, "students", hashIndexInsertTypedLayout(record.IntegerField, record.IntegerField, record.FieldType(255))), key: query.NewIntConstant(42)},
		{name: "value does not match dataval type", index: NewHashIndex(transaction, "students", hashIndexInsertLayout(record.VarcharField)), key: query.NewIntConstant(42)},
		{name: "unset key", index: NewHashIndex(transaction, "students", layout)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.index.Delete(test.key, record.NewRID(7, 3))
			if err == nil || !strings.Contains(err.Error(), "hash index") {
				t.Fatalf("Delete() error = %v, want contextual validation error", err)
			}
		})
	}
}
