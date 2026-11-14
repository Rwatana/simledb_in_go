package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewTableManagerRetainsCatalogLayouts(t *testing.T) {
	manager := NewTableManager()
	if manager == nil {
		t.Fatal("NewTableManager() returned nil")
	}

	assertSameLayout(t, manager.tableCatalogLayout, NewTableCatalogLayout())
	assertSameLayout(t, manager.fieldCatalogLayout, NewFieldCatalogLayout())
}

func assertSameLayout(t *testing.T, got, want *record.Layout) {
	t.Helper()
	if got == nil {
		t.Fatal("layout is nil")
	}
	if got.SlotSize() != want.SlotSize() {
		t.Fatalf("slot size = %d, want %d", got.SlotSize(), want.SlotSize())
	}
	if gotFields, wantFields := got.Schema().Fields(), want.Schema().Fields(); !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("fields = %v, want %v", gotFields, wantFields)
	}
	for _, name := range want.Schema().Fields() {
		gotDefinition, gotOK := got.Schema().Field(name)
		wantDefinition, wantOK := want.Schema().Field(name)
		if !gotOK || !wantOK || gotDefinition != wantDefinition {
			t.Fatalf("field %q = (%v, %v), want (%v, %v)", name, gotDefinition, gotOK, wantDefinition, wantOK)
		}
		gotOffset, gotOK := got.Offset(name)
		wantOffset, wantOK := want.Offset(name)
		if !gotOK || !wantOK || gotOffset != wantOffset {
			t.Fatalf("offset %q = (%d, %v), want (%d, %v)", name, gotOffset, gotOK, wantOffset, wantOK)
		}
	}
}
