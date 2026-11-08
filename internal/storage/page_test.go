package storage_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestNewPageHasFixedCapacity(t *testing.T) {
	t.Parallel()

	page, err := storage.NewPage(32)
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}

	if got, want := page.Capacity(), 32; got != want {
		t.Errorf("Capacity() = %d, want %d", got, want)
	}
	if _, err := storage.NewPage(-1); err == nil {
		t.Error("NewPage(-1) error = nil, want an invalid size error")
	}
}

func TestPageStoresFourByteIntegers(t *testing.T) {
	t.Parallel()

	page := newPage(t, 24)
	tests := []struct {
		offset int
		value  int32
	}{
		{offset: 0, value: 0},
		{offset: 4, value: 1},
		{offset: 8, value: -1},
		{offset: 12, value: math.MaxInt32},
		{offset: 16, value: math.MinInt32},
	}

	for _, tt := range tests {
		if err := page.SetInt(tt.offset, tt.value); err != nil {
			t.Fatalf("SetInt(%d, %d) error = %v", tt.offset, tt.value, err)
		}
	}
	for _, tt := range tests {
		got, err := page.GetInt(tt.offset)
		if err != nil {
			t.Fatalf("GetInt(%d) error = %v", tt.offset, err)
		}
		if got != tt.value {
			t.Errorf("GetInt(%d) = %d, want %d", tt.offset, got, tt.value)
		}
	}

	wantBigEndianOne := []byte{0x00, 0x00, 0x00, 0x01}
	if got := page.Snapshot()[4:8]; !bytes.Equal(got, wantBigEndianOne) {
		t.Errorf("integer bytes = %v, want big-endian %v", got, wantBigEndianOne)
	}
}

func TestPageStoresLengthPrefixedBytes(t *testing.T) {
	t.Parallel()

	page := newPage(t, 64)
	tests := []struct {
		offset int
		value  []byte
	}{
		{offset: 0, value: []byte{}},
		{offset: 8, value: []byte{0x00}},
		{offset: 16, value: []byte{0xde, 0xad, 0x00, 0xbe, 0xef}},
	}

	for _, tt := range tests {
		if err := page.SetBytes(tt.offset, tt.value); err != nil {
			t.Fatalf("SetBytes(%d, %v) error = %v", tt.offset, tt.value, err)
		}
	}
	for _, tt := range tests {
		got, err := page.GetBytes(tt.offset)
		if err != nil {
			t.Fatalf("GetBytes(%d) error = %v", tt.offset, err)
		}
		if !bytes.Equal(got, tt.value) {
			t.Errorf("GetBytes(%d) = %v, want %v", tt.offset, got, tt.value)
		}
		if len(got) > 0 {
			got[0] ^= 0xff
			again, err := page.GetBytes(tt.offset)
			if err != nil {
				t.Fatalf("GetBytes(%d) after mutation error = %v", tt.offset, err)
			}
			if !bytes.Equal(again, tt.value) {
				t.Errorf("GetBytes(%d) aliases page memory: got %v, want %v", tt.offset, again, tt.value)
			}
		}
	}
}

func TestPageRejectsOutOfBoundsAccess(t *testing.T) {
	t.Parallel()

	page := newPage(t, 8)

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "set integer before start", run: func() error { return page.SetInt(-1, 1) }},
		{name: "set integer past end", run: func() error { return page.SetInt(5, 1) }},
		{name: "get integer past end", run: func() error { _, err := page.GetInt(5); return err }},
		{name: "set bytes past end", run: func() error { return page.SetBytes(1, []byte{1, 2, 3, 4}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.run(); err == nil {
				t.Error("error = nil, want an out-of-bounds error")
			}
		})
	}

	if err := page.SetInt(0, 10); err != nil {
		t.Fatalf("SetInt() error = %v", err)
	}
	if _, err := page.GetBytes(0); err == nil {
		t.Error("GetBytes() error = nil, want an invalid length error")
	}
}

func TestPageSnapshotDoesNotExposeMutableMemory(t *testing.T) {
	t.Parallel()

	page := newPage(t, 8)
	if err := page.SetInt(0, 42); err != nil {
		t.Fatalf("SetInt() error = %v", err)
	}

	snapshot := page.Snapshot()
	snapshot[3] = 0

	got, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt() error = %v", err)
	}
	if got != 42 {
		t.Errorf("GetInt() = %d after snapshot mutation, want 42", got)
	}
}

func TestNewPageFromBytesCopiesContents(t *testing.T) {
	t.Parallel()

	contents := []byte{0x00, 0x00, 0x00, 0x2a}
	page := storage.NewPageFromBytes(contents)
	contents[3] = 0

	got, err := page.GetInt(0)
	if err != nil {
		t.Fatalf("GetInt() error = %v", err)
	}
	if got != 42 {
		t.Errorf("GetInt() = %d after source mutation, want 42", got)
	}
}

func newPage(t *testing.T, size int) *storage.Page {
	t.Helper()

	page, err := storage.NewPage(size)
	if err != nil {
		t.Fatalf("NewPage(%d) error = %v", size, err)
	}
	return page
}
