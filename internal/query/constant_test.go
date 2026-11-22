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
