package query

type constantKind uint8

const intConstantKind constantKind = 1

// Constant is a typed value produced and consumed by query scans.
type Constant struct {
	kind    constantKind
	integer int32
}

// NewIntConstant constructs an integer query value.
func NewIntConstant(value int32) Constant {
	return Constant{kind: intConstantKind, integer: value}
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
	return "", false
}
