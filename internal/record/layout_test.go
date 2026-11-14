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
