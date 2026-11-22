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

func TestConstantEqual(t *testing.T) {
	for _, tt := range []struct {
		name        string
		left, right Constant
		want        bool
	}{
		{"same integer", NewIntConstant(0), NewIntConstant(0), true},
		{"different integers", NewIntConstant(0), NewIntConstant(1), false},
		{"same string", NewStringConstant(""), NewStringConstant(""), true},
		{"different strings", NewStringConstant("A"), NewStringConstant("a"), false},
		{"different kinds", NewIntConstant(1), NewStringConstant("1"), false},
		{"unset and integer", Constant{}, NewIntConstant(0), false},
		{"unset and string", Constant{}, NewStringConstant(""), false},
		{"two unset values", Constant{}, Constant{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.left.Equal(tt.right); got != tt.want {
				t.Errorf("left.Equal(right) = %v, want %v", got, tt.want)
			}
			if got := tt.right.Equal(tt.left); got != tt.want {
				t.Errorf("right.Equal(left) = %v, want %v", got, tt.want)
			}
		})
	}
}
