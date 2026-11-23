package query

import "cmp"

type constantKind uint8

const (
	intConstantKind constantKind = iota + 1
	stringConstantKind
)

// Constant is a typed value produced and consumed by query scans.
type Constant struct {
	kind    constantKind
	integer int32
	str     string
}

// NewIntConstant constructs an integer query value.
func NewIntConstant(value int32) Constant {
	return Constant{kind: intConstantKind, integer: value}
}

// NewStringConstant constructs a string query value.
func NewStringConstant(value string) Constant {
	return Constant{kind: stringConstantKind, str: value}
}

// AsInt returns the stored integer and whether this constant contains one.
func (constant Constant) AsInt() (int32, bool) {
	if constant.kind != intConstantKind {
		return 0, false
	}
	return constant.integer, true
}

// AsString returns the stored string and whether this constant contains one.
func (constant Constant) AsString() (string, bool) {
	if constant.kind != stringConstantKind {
		return "", false
	}
	return constant.str, true
}

// Equal reports whether both constants hold the same value of the same kind.
// An unset constant never matches.
func (constant Constant) Equal(other Constant) bool {
	return constant.kind != 0 && constant == other
}

// Compare orders two constants of the same kind and reports whether they are comparable.
// The order is negative, zero, or positive when constant is less than, equal to, or greater than other.
func (constant Constant) Compare(other Constant) (int, bool) {
	if constant.kind != other.kind {
		return 0, false
	}
	switch constant.kind {
	case intConstantKind:
		return cmp.Compare(constant.integer, other.integer), true
	case stringConstantKind:
		return cmp.Compare(constant.str, other.str), true
	default:
		return 0, false
	}
}
