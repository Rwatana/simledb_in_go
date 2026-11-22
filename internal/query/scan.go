package query

// Scan is the common traversal and value-reading contract for query operators.
// Operations that may access storage return errors instead of panicking.
type Scan interface {
	BeforeFirst() error
	Next() (bool, error)
	GetInt(fieldName string) (int32, error)
	GetString(fieldName string) (string, error)
	ValueScan
	HasField(fieldName string) bool
	Close() error
}
