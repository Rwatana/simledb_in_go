package storage

import (
	"encoding/binary"
	"fmt"
	"math"
)

const encodedIntSize = 4

// Page stores fixed-capacity database values in memory.
// Integers and length prefixes use four-byte big-endian encoding.
type Page struct {
	contents []byte
}

// NewPage returns an empty Page with the requested fixed capacity.
func NewPage(capacity int) (*Page, error) {
	if capacity < 0 {
		return nil, fmt.Errorf("create page with capacity %d: capacity must not be negative", capacity)
	}
	return &Page{contents: make([]byte, capacity)}, nil
}

// Capacity returns the number of bytes available in the page.
func (p *Page) Capacity() int {
	return len(p.contents)
}

// GetInt returns the signed four-byte integer stored at offset.
func (p *Page) GetInt(offset int) (int32, error) {
	value, err := p.bytesAt(offset, encodedIntSize)
	if err != nil {
		return 0, fmt.Errorf("get integer at offset %d: %w", offset, err)
	}
	return int32(binary.BigEndian.Uint32(value)), nil
}

// SetInt stores value as a signed four-byte integer at offset.
func (p *Page) SetInt(offset int, value int32) error {
	destination, err := p.bytesAt(offset, encodedIntSize)
	if err != nil {
		return fmt.Errorf("set integer at offset %d: %w", offset, err)
	}
	binary.BigEndian.PutUint32(destination, uint32(value))
	return nil
}

// GetBytes returns a copy of the length-prefixed byte sequence stored at offset.
func (p *Page) GetBytes(offset int) ([]byte, error) {
	length, err := p.GetInt(offset)
	if err != nil {
		return nil, fmt.Errorf("get bytes at offset %d: %w", offset, err)
	}
	if length < 0 {
		return nil, fmt.Errorf("get bytes at offset %d: negative length %d", offset, length)
	}

	payload, err := p.bytesAt(offset+encodedIntSize, int(length))
	if err != nil {
		return nil, fmt.Errorf("get bytes at offset %d with length %d: %w", offset, length, err)
	}
	result := make([]byte, len(payload))
	copy(result, payload)
	return result, nil
}

// SetBytes stores value at offset after a four-byte length prefix.
func (p *Page) SetBytes(offset int, value []byte) error {
	if len(value) > math.MaxInt32 {
		return fmt.Errorf("set bytes at offset %d: length %d exceeds four-byte signed prefix", offset, len(value))
	}
	destination, err := p.bytesAt(offset, encodedIntSize+len(value))
	if err != nil {
		return fmt.Errorf("set bytes at offset %d with length %d: %w", offset, len(value), err)
	}
	binary.BigEndian.PutUint32(destination[:encodedIntSize], uint32(len(value)))
	copy(destination[encodedIntSize:], value)
	return nil
}

// Snapshot returns a copy of all encoded bytes in the page.
func (p *Page) Snapshot() []byte {
	result := make([]byte, len(p.contents))
	copy(result, p.contents)
	return result
}

func (p *Page) bytesAt(offset, length int) ([]byte, error) {
	if offset < 0 || length < 0 || offset > len(p.contents) || length > len(p.contents)-offset {
		return nil, fmt.Errorf("byte range [%d:%d] exceeds page capacity %d", offset, offset+length, len(p.contents))
	}
	return p.contents[offset : offset+length], nil
}
