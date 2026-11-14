// Package metadata describes and stores database catalog metadata.
package metadata

import "github.com/Rwatana/simledb_in_go/internal/record"

// MaxNameLength is the maximum number of characters stored for catalog object names.
const MaxNameLength = 16

// NewTableCatalogLayout constructs the physical layout of the table catalog.
func NewTableCatalogLayout() *record.Layout {
	var schema record.Schema
	schema.AddVarcharField("tblname", MaxNameLength)
	schema.AddIntegerField("slotsize")
	return record.NewLayout(&schema)
}

// NewFieldCatalogLayout constructs the physical layout of the field catalog.
func NewFieldCatalogLayout() *record.Layout {
	var schema record.Schema
	schema.AddVarcharField("tblname", MaxNameLength)
	schema.AddVarcharField("fldname", MaxNameLength)
	schema.AddIntegerField("type")
	schema.AddIntegerField("length")
	schema.AddIntegerField("offset")
	return record.NewLayout(&schema)
}
