// Package storage defines values and rules used by the database storage engine.
package storage

import "fmt"

// BlockID identifies one logical block within a database file.
// Block numbers are zero-based.
type BlockID struct {
	fileName string
	number   int
}

// NewBlockID returns an identifier for number in fileName.
func NewBlockID(fileName string, number int) BlockID {
	return BlockID{
		fileName: fileName,
		number:   number,
	}
}

// FileName returns the database file containing the block.
func (b BlockID) FileName() string {
	return b.fileName
}

// Number returns the block's zero-based logical number within its file.
func (b BlockID) Number() int {
	return b.number
}

// String returns a human-readable description of the block location.
func (b BlockID) String() string {
	return fmt.Sprintf("[file %s, block %d]", b.fileName, b.number)
}
