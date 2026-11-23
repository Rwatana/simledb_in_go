package hash

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestHashIndexGetDataRIDReadsCurrentBlockAndSlot(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	layout := record.NewLayout(schema)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")
	transaction := &hashIndexNextTransaction{
		blockSize: layout.SlotSize(),
		integers: map[int]int32{
			0:                     1,
			blockOffset:           12,
			idOffset:              4,
			layout.SlotSize():     0,
			layout.SlotSize() + 4: 0,
		},
		strings:        map[int]string{},
		getIntErrAt:    -1,
		getStringErrAt: -1,
	}
	index := newHashIndexRIDTestIndex(t, transaction, layout)
	if found, err := index.scan.Next(); err != nil || !found {
		t.Fatalf("TableScan.Next() = (%t, %v), want (true, nil)", found, err)
	}

	got, err := index.GetDataRID()
	if err != nil {
		t.Fatalf("GetDataRID() error = %v", err)
	}
	if want := record.NewRID(12, 4); !got.Equal(want) {
		t.Errorf("GetDataRID() = %v, want %v", got, want)
	}
}

func TestHashIndexGetDataRIDRejectsInvalidStateAndFields(t *testing.T) {
	validSchema := &record.Schema{}
	validSchema.AddIntegerField("block")
	validSchema.AddIntegerField("id")
	validLayout := record.NewLayout(validSchema)
	tests := []struct {
		name  string
		index *HashIndex
	}{
		{name: "nil receiver"},
		{name: "missing scan", index: NewHashIndex(nil, "students", validLayout)},
		{name: "nil layout", index: newHashIndexRIDIndexWithScan(t, nil)},
		{name: "nil schema", index: newHashIndexRIDIndexWithScan(t, record.NewLayoutFromMetadata(nil, map[string]int{"block": 4, "id": 8}, 12))},
		{name: "missing block field", index: newHashIndexRIDIndexWithScan(t, hashIndexRIDLayout("id"))},
		{name: "missing id field", index: newHashIndexRIDIndexWithScan(t, hashIndexRIDLayout("block"))},
		{name: "varchar block field", index: newHashIndexRIDIndexWithScan(t, hashIndexRIDTypedLayout(record.VarcharField, record.IntegerField))},
		{name: "varchar id field", index: newHashIndexRIDIndexWithScan(t, hashIndexRIDTypedLayout(record.IntegerField, record.VarcharField))},
		{name: "no current record", index: newHashIndexRIDIndexWithScan(t, validLayout)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rid, err := test.index.GetDataRID(); err == nil || rid != (record.RID{}) || !strings.Contains(err.Error(), "hash index") {
				t.Fatalf("GetDataRID() = (%v, %v), want contextual validation/read error", rid, err)
			}
		})
	}
}

func TestHashIndexGetDataRIDWrapsBlockAndIDReadFailures(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	layout := record.NewLayout(schema)
	blockOffset, _ := layout.Offset("block")
	idOffset, _ := layout.Offset("id")

	tests := []struct {
		name       string
		readOffset int
	}{
		{name: "block", readOffset: blockOffset},
		{name: "id", readOffset: idOffset},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readErr := errors.New(test.name + " read failed")
			transaction := &hashIndexNextTransaction{
				blockSize:      layout.SlotSize(),
				integers:       map[int]int32{0: 1},
				strings:        map[int]string{},
				getIntErrAt:    test.readOffset,
				getIntErr:      readErr,
				getStringErrAt: -1,
			}
			index := newHashIndexRIDTestIndex(t, transaction, layout)
			if found, err := index.scan.Next(); err != nil || !found {
				t.Fatalf("TableScan.Next() = (%t, %v), want (true, nil)", found, err)
			}
			rid, err := index.GetDataRID()
			if !errors.Is(err, readErr) || rid != (record.RID{}) || !strings.Contains(err.Error(), "students") {
				t.Fatalf("GetDataRID() = (%v, %v), want wrapped %s error", rid, err, test.name)
			}
		})
	}
}

func newHashIndexRIDTestIndex(t *testing.T, transaction *hashIndexNextTransaction, layout *record.Layout) *HashIndex {
	t.Helper()
	scan, err := record.NewTableScan(transaction, "bucket", layout)
	if err != nil {
		t.Fatalf("NewTableScan() error = %v", err)
	}
	index := NewHashIndex(transaction, "students", layout)
	index.scan = scan
	return index
}

func newHashIndexRIDIndexWithScan(t *testing.T, layout *record.Layout) *HashIndex {
	t.Helper()
	scanLayout := layout
	if scanLayout == nil {
		schema := &record.Schema{}
		schema.AddIntegerField("block")
		schema.AddIntegerField("id")
		scanLayout = record.NewLayout(schema)
	}
	transaction := &hashIndexNextTransaction{
		blockSize:      scanLayout.SlotSize(),
		integers:       map[int]int32{0: 1, 4: 1, 8: 2},
		strings:        map[int]string{},
		getIntErrAt:    -1,
		getStringErrAt: -1,
	}
	index := newHashIndexRIDTestIndex(t, transaction, scanLayout)
	index.layout = layout
	return index
}

func hashIndexRIDLayout(field string) *record.Layout {
	schema := &record.Schema{}
	schema.AddIntegerField(field)
	return record.NewLayout(schema)
}

func hashIndexRIDTypedLayout(blockType, idType record.FieldType) *record.Layout {
	schema := &record.Schema{}
	schema.AddField("block", blockType, 2)
	schema.AddField("id", idType, 2)
	return record.NewLayout(schema)
}
