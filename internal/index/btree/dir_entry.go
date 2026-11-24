package btree

import "github.com/Rwatana/simledb_in_go/internal/query"

// DirEntry identifies a child B-tree block by its first key and block number.
type DirEntry struct {
	dataVal     query.Constant
	blockNumber int
}

// NewDirEntry returns a directory entry containing dataVal and blockNumber.
func NewDirEntry(dataVal query.Constant, blockNumber int) DirEntry {
	return DirEntry{dataVal: dataVal, blockNumber: blockNumber}
}

// DataVal returns the key associated with the child block.
func (entry DirEntry) DataVal() query.Constant {
	return entry.dataVal
}

// BlockNumber returns the child block number.
func (entry DirEntry) BlockNumber() int {
	return entry.blockNumber
}
