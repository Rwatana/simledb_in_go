package metadata

import (
	"reflect"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewViewCatalogSchemaDescribesStoredViewDefinitions(t *testing.T) {
	schema := NewViewCatalogSchema()
	if schema == nil {
		t.Fatal("NewViewCatalogSchema() returned nil")
	}
	if got, want := schema.Fields(), []string{"viewname", "viewdef"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for name, wantLength := range map[string]int{
		"viewname": MaxNameLength,
		"viewdef":  MaxViewDefinitionLength,
	} {
		if got, ok := schema.TypeOf(name); !ok || got != record.VarcharField {
			t.Fatalf("%s type = (%v, %v), want (%v, true)", name, got, ok, record.VarcharField)
		}
		if got, ok := schema.LengthOf(name); !ok || got != wantLength {
			t.Fatalf("%s length = (%d, %v), want (%d, true)", name, got, ok, wantLength)
		}
	}
}
