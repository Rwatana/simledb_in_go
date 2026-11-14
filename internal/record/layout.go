package record

import "unicode/utf8"

const encodedIntegerSize = 4

// Layout stores physical byte offsets calculated for an ordered schema.
type Layout struct {
	offsets  map[string]int
	slotSize int
}

// NewLayout calculates physical field offsets for schema.
func NewLayout(schema *Schema) *Layout {
	layout := &Layout{offsets: make(map[string]int)}
	position := encodedIntegerSize
	for _, name := range schema.Fields() {
		layout.offsets[name] = position
		definition, _ := schema.Field(name)
		position += fieldLengthInBytes(definition)
	}
	layout.slotSize = position
	return layout
}

// Offset returns the byte offset registered for name.
func (l *Layout) Offset(name string) (int, bool) {
	offset, ok := l.offsets[name]
	return offset, ok
}

func fieldLengthInBytes(definition FieldDefinition) int {
	if definition.Type == IntegerField {
		return encodedIntegerSize
	}
	return encodedIntegerSize + definition.Length*utf8.UTFMax
}
