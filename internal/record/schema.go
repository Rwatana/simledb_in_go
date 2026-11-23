// Package record describes and stores fixed-layout database records.
package record

// FieldType identifies the logical value representation of a schema field.
type FieldType uint8

const (
	// IntegerField stores a four-byte signed integer.
	IntegerField FieldType = iota + 1
	// VarcharField stores a length-prefixed string with a conceptual character limit.
	VarcharField
)

// FieldDefinition stores one field's logical type and conceptual length.
type FieldDefinition struct {
	Type   FieldType
	Length int
}

// Schema stores ordered field names and their definitions.
// The zero value is ready for use.
type Schema struct {
	fields      []string
	definitions map[string]FieldDefinition
}

// AddField registers name at the end of the declaration order with its type and conceptual length.
func (s *Schema) AddField(name string, fieldType FieldType, length int) {
	if s.definitions == nil {
		s.definitions = make(map[string]FieldDefinition)
	}
	s.fields = append(s.fields, name)
	s.definitions[name] = FieldDefinition{Type: fieldType, Length: length}
}

// AddIntegerField registers a four-byte integer field with conceptual length zero.
func (s *Schema) AddIntegerField(name string) {
	s.AddField(name, IntegerField, 0)
}

// AddVarcharField registers a varchar field with the supplied conceptual character length.
func (s *Schema) AddVarcharField(name string, length int) {
	s.AddField(name, VarcharField, length)
}

// Fields returns field names in declaration order.
func (s *Schema) Fields() []string {
	return append([]string(nil), s.fields...)
}

// Field returns the definition registered for name.
func (s *Schema) Field(name string) (FieldDefinition, bool) {
	definition, ok := s.definitions[name]
	return definition, ok
}
