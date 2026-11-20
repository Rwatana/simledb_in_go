package metadata

import (
	"errors"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewIndexInfoBuildsIndexRecordLayout(t *testing.T) {
	transaction := newEmptyCatalogTransaction(400)
	stats := NewStatInfo(3, 12)
	tests := []struct {
		name       string
		addField   func(*record.Schema)
		wantType   record.FieldType
		wantLength int
	}{
		{name: "integer", addField: func(schema *record.Schema) { schema.AddIntegerField("key") }, wantType: record.IntegerField},
		{name: "varchar", addField: func(schema *record.Schema) { schema.AddVarcharField("key", 24) }, wantType: record.VarcharField, wantLength: 24},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var tableSchema record.Schema
			test.addField(&tableSchema)
			info, err := NewIndexInfo("key_idx", "key", &tableSchema, transaction, stats)
			if err != nil {
				t.Fatalf("NewIndexInfo() error = %v", err)
			}
			if info.indexName != "key_idx" || info.fieldName != "key" || info.transaction != transaction || info.tableSchema != &tableSchema || info.stats != stats {
				t.Fatalf("retained dependencies = %#v, want supplied identity, schema, transaction, and statistics", info)
			}
			if got := info.indexLayout.Schema().Fields(); len(got) != 3 || got[0] != "block" || got[1] != "id" || got[2] != "dataval" {
				t.Fatalf("index fields = %v, want [block id dataval]", got)
			}
			if fieldType, ok := info.indexLayout.Schema().TypeOf("dataval"); !ok || fieldType != test.wantType {
				t.Fatalf("dataval type = (%v, %t), want (%v, true)", fieldType, ok, test.wantType)
			}
			if length, ok := info.indexLayout.Schema().LengthOf("dataval"); !ok || length != test.wantLength {
				t.Fatalf("dataval length = (%d, %t), want (%d, true)", length, ok, test.wantLength)
			}
		})
	}
}

func TestIndexInfoBlocksAccessedEstimatesStaticHashBucketCost(t *testing.T) {
	transaction := newEmptyCatalogTransaction(400)
	var tableSchema record.Schema
	tableSchema.AddIntegerField("key")
	info, err := NewIndexInfo("key_idx", "key", &tableSchema, transaction, NewStatInfo(999, 25_000))
	if err != nil {
		t.Fatalf("NewIndexInfo() error = %v", err)
	}
	cost, err := info.BlocksAccessed()
	if err != nil {
		t.Fatalf("BlocksAccessed() error = %v", err)
	}
	if cost != 10 {
		t.Fatalf("BlocksAccessed() = %d, want 10", cost)
	}
}

type failingBlockSizeTransaction struct {
	*catalogTransaction
	err error
}

func (t *failingBlockSizeTransaction) BlockSize() (int, error) {
	return 0, t.err
}

func TestIndexInfoBlocksAccessedRejectsUnavailableOrTooSmallBlocks(t *testing.T) {
	var tableSchema record.Schema
	tableSchema.AddVarcharField("key", 24)
	blockSizeErr := errors.New("block size unavailable")
	tests := []struct {
		name        string
		transaction record.TableScanTransaction
		want        string
	}{
		{name: "unavailable", transaction: &failingBlockSizeTransaction{catalogTransaction: newEmptyCatalogTransaction(400), err: blockSizeErr}, want: "block size unavailable"},
		{name: "too small", transaction: newEmptyCatalogTransaction(8), want: "does not fit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := NewIndexInfo("key_idx", "key", &tableSchema, test.transaction, NewStatInfo(1, 1))
			if err != nil {
				t.Fatalf("NewIndexInfo() error = %v", err)
			}
			_, err = info.BlocksAccessed()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("BlocksAccessed() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestNewIndexInfoRejectsInvalidMetadata(t *testing.T) {
	transaction := newEmptyCatalogTransaction(400)
	stats := NewStatInfo(3, 12)
	var schema record.Schema
	schema.AddIntegerField("key")
	tests := []struct {
		name        string
		tableSchema *record.Schema
		transaction record.TableScanTransaction
		stats       *StatInfo
		want        string
	}{
		{name: "nil schema", transaction: transaction, stats: stats, want: "schema is nil"},
		{name: "nil transaction", tableSchema: &schema, stats: stats, want: "transaction is nil"},
		{name: "nil statistics", tableSchema: &schema, transaction: transaction, want: "statistics are nil"},
		{name: "missing field", tableSchema: &schema, transaction: transaction, stats: stats, want: `field "missing" does not exist`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fieldName := "key"
			if test.name == "missing field" {
				fieldName = "missing"
			}
			_, err := NewIndexInfo("key_idx", fieldName, test.tableSchema, test.transaction, test.stats)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewIndexInfo() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
