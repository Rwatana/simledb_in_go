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
