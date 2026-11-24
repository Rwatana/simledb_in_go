package btree

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestDirectoryLayoutFromLeafCopiesIntegerKey(t *testing.T) {
	leafLayout := pageInsertLeafLayout(record.IntegerField)
	leafFields := leafLayout.Schema().Fields()
	leafDataval, _ := leafLayout.Schema().Field("dataval")

	directoryLayout, err := directoryLayoutFromLeaf(leafLayout)
	if err != nil {
		t.Fatalf("directoryLayoutFromLeaf() error = %v", err)
	}
	assertDerivedDirectoryLayout(t, directoryLayout, record.IntegerField, 0, 12)
	if got := leafLayout.Schema().Fields(); !reflect.DeepEqual(got, leafFields) {
		t.Errorf("leaf schema fields changed = %v, want %v", got, leafFields)
	}
	if got, _ := leafLayout.Schema().Field("dataval"); got != leafDataval {
		t.Errorf("leaf dataval definition changed = %+v, want %+v", got, leafDataval)
	}
}

func TestDirectoryLayoutFromLeafCopiesVarcharKeyLength(t *testing.T) {
	var schema record.Schema
	schema.AddIntegerField("block")
	schema.AddIntegerField("id")
	schema.AddVarcharField("dataval", 7)

	layout, err := directoryLayoutFromLeaf(record.NewLayout(&schema))
	if err != nil {
		t.Fatalf("directoryLayoutFromLeaf() error = %v", err)
	}
	assertDerivedDirectoryLayout(t, layout, record.VarcharField, 7, 40)
}

func TestDirectoryLayoutFromLeafRejectsInvalidLeafLayouts(t *testing.T) {
	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("dataval", record.FieldType(99), 0)
	unsupportedSchema.AddIntegerField("block")
	unsupportedSchema.AddIntegerField("id")

	var negativeVarcharSchema record.Schema
	negativeVarcharSchema.AddVarcharField("dataval", -1)
	negativeVarcharSchema.AddIntegerField("block")
	negativeVarcharSchema.AddIntegerField("id")

	tests := []struct {
		name   string
		layout *record.Layout
	}{
		{name: "nil layout"},
		{name: "nil schema", layout: record.NewLayoutFromMetadata(nil, nil, 12)},
		{name: "missing dataval", layout: newLeafLayoutWithoutDataval()},
		{name: "missing block", layout: pageInsertLeafLayoutWithoutBlock()},
		{name: "missing id", layout: pageInsertLeafLayoutWithoutID()},
		{name: "directory layout", layout: pageInsertDirLayout(record.IntegerField)},
		{name: "unsupported dataval type", layout: record.NewLayout(&unsupportedSchema)},
		{name: "negative varchar length", layout: record.NewLayout(&negativeVarcharSchema)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout, err := directoryLayoutFromLeaf(test.layout)
			if err == nil {
				t.Fatalf("directoryLayoutFromLeaf() = %v, nil error; want invalid-layout error", layout)
			}
			if layout != nil {
				t.Errorf("directoryLayoutFromLeaf() layout = %v, want nil on error", layout)
			}
		})
	}
}

func assertDerivedDirectoryLayout(t *testing.T, layout *record.Layout, keyType record.FieldType, keyLength, slotSize int) {
	t.Helper()
	if layout == nil {
		t.Fatal("directory layout is nil")
	}
	if got, want := layout.Schema().Fields(), []string{"block", "dataval"}; !reflect.DeepEqual(got, want) {
		t.Errorf("directory schema fields = %v, want %v", got, want)
	}
	if got, _ := layout.Schema().Field("block"); got != (record.FieldDefinition{Type: record.IntegerField}) {
		t.Errorf("block field = %+v, want integer", got)
	}
	if got, _ := layout.Schema().Field("dataval"); got != (record.FieldDefinition{Type: keyType, Length: keyLength}) {
		t.Errorf("dataval field = %+v, want type %v and length %d", got, keyType, keyLength)
	}
	if got, _ := layout.Offset("block"); got != 4 {
		t.Errorf("block offset = %d, want 4", got)
	}
	if got, _ := layout.Offset("dataval"); got != 8 {
		t.Errorf("dataval offset = %d, want 8", got)
	}
	if got := layout.SlotSize(); got != slotSize {
		t.Errorf("directory slot size = %d, want %d", got, slotSize)
	}
}
