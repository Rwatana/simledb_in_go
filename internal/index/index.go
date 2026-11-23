// Package index defines the contracts used to search and maintain database indexes.
package index

import (
	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

// Index traverses index entries for one search key and maintains value/RID pairs.
type Index interface {
	// BeforeFirst positions the index before entries matching searchKey.
	BeforeFirst(searchKey query.Constant) error

	// Next advances to the next matching entry and reports whether one exists.
	Next() (bool, error)

	// GetDataRID returns the data record identifier at the current index entry.
	GetDataRID() (record.RID, error)

	// Insert adds an index entry for dataValue and dataRID.
	Insert(dataValue query.Constant, dataRID record.RID) error

	// Delete removes the index entry for dataValue and dataRID.
	Delete(dataValue query.Constant, dataRID record.RID) error

	// Close releases resources held by the index.
	Close() error
}
