package storage_test

import (
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/storage"
)

func TestBlockIDIdentifiesAFileBlock(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 23)

	if got, want := block.FileName(), "student.tbl"; got != want {
		t.Errorf("FileName() = %q, want %q", got, want)
	}
	if got, want := block.Number(), 23; got != want {
		t.Errorf("Number() = %d, want %d", got, want)
	}
}

func TestBlockIDEqualityUsesFileNameAndNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  storage.BlockID
		right storage.BlockID
		want  bool
	}{
		{
			name:  "same location",
			left:  storage.NewBlockID("student.tbl", 23),
			right: storage.NewBlockID("student.tbl", 23),
			want:  true,
		},
		{
			name:  "different file",
			left:  storage.NewBlockID("student.tbl", 23),
			right: storage.NewBlockID("major.tbl", 23),
			want:  false,
		},
		{
			name:  "different block number",
			left:  storage.NewBlockID("student.tbl", 23),
			right: storage.NewBlockID("student.tbl", 24),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.left == tt.right; got != tt.want {
				t.Errorf("(%v == %v) = %t, want %t", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestBlockIDCanBeUsedAsMapKey(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 23)
	pages := map[storage.BlockID]string{block: "cached"}

	if got, want := pages[storage.NewBlockID("student.tbl", 23)], "cached"; got != want {
		t.Errorf("map lookup = %q, want %q", got, want)
	}
}

func TestBlockIDStringDescribesTheLocation(t *testing.T) {
	t.Parallel()

	block := storage.NewBlockID("student.tbl", 23)

	if got, want := block.String(), "[file student.tbl, block 23]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
