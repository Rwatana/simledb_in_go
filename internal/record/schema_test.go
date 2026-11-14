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

func TestSchemaAddFromCopiesOneFieldDefinition(t *testing.T) {
	var source Schema
	source.AddVarcharField("name", 32)

	var destination Schema
	destination.AddIntegerField("id")
	if ok := destination.AddFrom("name", &source); !ok {
		t.Fatal("AddFrom(name) = false, want true")
	}

	if got, want := destination.Fields(), []string{"id", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	definition, ok := destination.Field("name")
	if !ok || definition.Type != VarcharField || definition.Length != 32 {
		t.Fatalf("Field(name) = (%+v, %v), want varchar length 32", definition, ok)
	}

	before := destination.Fields()
	if ok := destination.AddFrom("missing", &source); ok {
		t.Fatal("AddFrom(missing) = true, want false")
	}
	if got := destination.Fields(); !reflect.DeepEqual(got, before) {
		t.Fatalf("Fields() after missing copy = %v, want unchanged %v", got, before)
	}
}

func TestSchemaAddAllFromPreservesSourceOrderAndMetadata(t *testing.T) {
	var source Schema
	source.AddIntegerField("count")
	source.AddVarcharField("label", 18)

	var destination Schema
	destination.AddVarcharField("prefix", 4)
	destination.AddAllFrom(&source)

	if got, want := destination.Fields(), []string{"prefix", "count", "label"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %v, want %v", got, want)
	}
	count, countOK := destination.Field("count")
	if !countOK || count.Type != IntegerField || count.Length != 0 {
		t.Fatalf("Field(count) = (%+v, %v), want integer length 0", count, countOK)
	}
	label, labelOK := destination.Field("label")
	if !labelOK || label.Type != VarcharField || label.Length != 18 {
		t.Fatalf("Field(label) = (%+v, %v), want varchar length 18", label, labelOK)
	}
}
