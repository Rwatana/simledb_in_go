package query

import "github.com/Rwatana/simledb_in_go/internal/record"

// UpdateScan is a scan that can modify records and reposition by record ID.
// Operations that can access storage return errors instead of panicking.
type UpdateScan interface {
	Scan
	SetInt(fieldName string, value int32) error
	SetString(fieldName, value string) error
	SetVal(fieldName string, value Constant) error
	Insert() error
	Delete() error
	GetRID() (record.RID, error)
	MoveToRID(rid record.RID) error
}
