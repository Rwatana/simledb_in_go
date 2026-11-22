package query

import "testing"

func TestNewIntConstantRetainsOnlyIntegerValue(t *testing.T) {
	constant := NewIntConstant(42)
	if got, ok := constant.AsInt(); !ok || got != 42 {
		t.Fatalf("AsInt() = (%d, %v), want (42, true)", got, ok)
	}
	if got, ok := constant.AsString(); ok || got != "" {
		t.Fatalf("AsString() = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestNewStringConstantRetainsOnlyStringValue(t *testing.T) {
	for _, value := range []string{"東京", ""} {
		constant := NewStringConstant(value)
		if got, ok := constant.AsString(); !ok || got != value {
			t.Fatalf("AsString() = (%q, %v), want (%q, true)", got, ok, value)
		}
		if got, ok := constant.AsInt(); ok || got != 0 {
			t.Fatalf("AsInt() for %q = (%d, %v), want (0, false)", value, got, ok)
		}
	}
}
