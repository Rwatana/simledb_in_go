package record

import (
	"reflect"
	"testing"
)

func TestSchemaAddFieldPreservesOrderAndMetadata(t *testing.T) {
	var schema Schema
	schema.AddField("id", IntegerField, 0)
	schema.AddField("name", VarcharField, 20)

	if got, want := schema.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	id, ok := schema.Field("id")
	if !ok || id.Type != IntegerField || id.Length != 0 {
		t.Fatalf("Field(id) = (%+v, %v), want integer length 0", id, ok)
	}
	name, ok := schema.Field("name")
	if !ok || name.Type != VarcharField || name.Length != 20 {
		t.Fatalf("Field(name) = (%+v, %v), want varchar length 20", name, ok)
	}
	if _, ok := schema.Field("missing"); ok {
		t.Fatal("Field(missing) unexpectedly found")
	}
}

func TestSchemaAddIntegerFieldUsesIntegerMetadata(t *testing.T) {
	var schema Schema
	schema.AddField("prefix", VarcharField, 4)
	schema.AddIntegerField("count")

	if got, want := schema.Fields(), []string{"prefix", "count"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	definition, ok := schema.Field("count")
	if !ok || definition.Type != IntegerField || definition.Length != 0 {
		t.Fatalf("Field(count) = (%+v, %v), want integer length 0", definition, ok)
	}
}

func TestSchemaAddVarcharFieldUsesSuppliedLength(t *testing.T) {
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 24)

	if got, want := schema.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	definition, ok := schema.Field("name")
	if !ok || definition.Type != VarcharField || definition.Length != 24 {
		t.Fatalf("Field(name) = (%+v, %v), want varchar length 24", definition, ok)
	}
}
