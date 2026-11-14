package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewTableCatalogLayoutDescribesTableMetadata(t *testing.T) {
	layout := NewTableCatalogLayout()
	schema := layout.Schema()

	if got, want := schema.Fields(), []string{"tblname", "slotsize"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	if got, ok := schema.TypeOf("tblname"); !ok || got != record.VarcharField {
		t.Fatalf("tblname type = (%v, %v), want (%v, true)", got, ok, record.VarcharField)
	}
	if got, ok := schema.LengthOf("tblname"); !ok || got != MaxNameLength {
		t.Fatalf("tblname length = (%d, %v), want (%d, true)", got, ok, MaxNameLength)
	}
	if got, ok := schema.TypeOf("slotsize"); !ok || got != record.IntegerField {
		t.Fatalf("slotsize type = (%v, %v), want (%v, true)", got, ok, record.IntegerField)
	}
	if got, ok := schema.LengthOf("slotsize"); !ok || got != 0 {
		t.Fatalf("slotsize length = (%d, %v), want (0, true)", got, ok)
	}
	if got, ok := layout.Offset("tblname"); !ok || got != 4 {
		t.Fatalf("tblname offset = (%d, %v), want (4, true)", got, ok)
	}
	if got, ok := layout.Offset("slotsize"); !ok || got != 72 {
		t.Fatalf("slotsize offset = (%d, %v), want (72, true)", got, ok)
	}
	if got, want := layout.SlotSize(), 76; got != want {
		t.Fatalf("slot size = %d, want %d", got, want)
	}
}

func TestNewFieldCatalogLayoutDescribesFieldMetadata(t *testing.T) {
	layout := NewFieldCatalogLayout()
	schema := layout.Schema()

	if got, want := schema.Fields(), []string{"tblname", "fldname", "type", "length", "offset"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for _, name := range []string{"tblname", "fldname"} {
		if got, ok := schema.TypeOf(name); !ok || got != record.VarcharField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.VarcharField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != MaxNameLength {
			t.Fatalf("%s length = (%d, %v), want (%d, true)", name, got, ok, MaxNameLength)
		}
	}
	for _, name := range []string{"type", "length", "offset"} {
		if got, ok := schema.TypeOf(name); !ok || got != record.IntegerField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.IntegerField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != 0 {
			t.Fatalf("%s length = (%d, %v), want (0, true)", name, got, ok)
		}
	}
	for name, want := range map[string]int{
		"tblname": 4,
		"fldname": 72,
		"type":    140,
		"length":  144,
		"offset":  148,
	} {
		if got, ok := layout.Offset(name); !ok || got != want {
			t.Fatalf("%s offset = (%d, %v), want (%d, true)", name, got, ok, want)
		}
	}
	if got, want := layout.SlotSize(), 152; got != want {
		t.Fatalf("slot size = %d, want %d", got, want)
	}
}
