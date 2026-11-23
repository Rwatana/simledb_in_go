package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewIndexCatalogSchemaDescribesStoredIndexRegistrations(t *testing.T) {
	schema := NewIndexCatalogSchema()
	if schema == nil {
		t.Fatal("NewIndexCatalogSchema() returned nil")
	}
	if got, want := schema.Fields(), []string{"indexname", "tablename", "fieldname"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for _, name := range schema.Fields() {
		if got, ok := schema.TypeOf(name); !ok || got != record.VarcharField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.VarcharField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != MaxNameLength {
			t.Fatalf("%s length = (%d, %v), want (%d, true)", name, got, ok, MaxNameLength)
		}
	}
}
