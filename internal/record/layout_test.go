package record

import "testing"

func TestLayoutCalculatesOrderedFieldOffsets(t *testing.T) {
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 3)
	schema.AddIntegerField("score")

	layout := NewLayout(&schema)
	for name, want := range map[string]int{
		"id":    4,
		"name":  8,
		"score": 24,
	} {
		got, ok := layout.Offset(name)
		if !ok || got != want {
			t.Fatalf("Offset(%q) = (%d, %v), want (%d, true)", name, got, ok, want)
		}
	}
	if _, ok := layout.Offset("missing"); ok {
		t.Fatal("Offset(missing) unexpectedly found")
	}
}

func TestLayoutSlotSizeIncludesFlagAndEncodedFields(t *testing.T) {
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 3)
	schema.AddIntegerField("score")

	if got, want := NewLayout(&schema).SlotSize(), 28; got != want {
		t.Fatalf("SlotSize() = %d, want %d", got, want)
	}
	if got, want := NewLayout(&Schema{}).SlotSize(), 4; got != want {
		t.Fatalf("empty SlotSize() = %d, want %d", got, want)
	}
}

func TestLayoutExposesAssociatedSchema(t *testing.T) {
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 20)

	layout := NewLayout(&schema)
	if got := layout.Schema(); got != &schema {
		t.Fatalf("Schema() = %p, want original %p", got, &schema)
	}
	if !layout.Schema().HasField("name") {
		t.Fatal("Schema() does not expose the associated declarations")
	}
}

func TestLayoutFromMetadataPreservesPhysicalDescription(t *testing.T) {
	var schema Schema
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 20)
	offsets := map[string]int{"id": 12, "name": 40}

	layout := NewLayoutFromMetadata(&schema, offsets, 128)
	if layout.Schema() != &schema {
		t.Fatal("Schema() does not return the metadata schema")
	}
	if got := layout.SlotSize(); got != 128 {
		t.Fatalf("SlotSize() = %d, want 128", got)
	}
	for name, want := range offsets {
		got, ok := layout.Offset(name)
		if !ok || got != want {
			t.Fatalf("Offset(%q) = (%d, %v), want (%d, true)", name, got, ok, want)
		}
	}

	offsets["id"] = 99
	if got, _ := layout.Offset("id"); got != 12 {
		t.Fatalf("Offset(id) after caller mutation = %d, want 12", got)
	}
}
