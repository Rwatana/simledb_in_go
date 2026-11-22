package metadata

import "testing"

func TestNewMetadataManagerWiresSharedDependencies(t *testing.T) {
	manager := NewMetadataManager()
	if manager == nil {
		t.Fatal("NewMetadataManager() returned nil")
	}
	if manager.tables == nil || manager.views == nil || manager.stats == nil || manager.indexes == nil {
		t.Fatalf("managers = (%v, %v, %v, %v), want all initialized", manager.tables, manager.views, manager.stats, manager.indexes)
	}
	if manager.views.tables != manager.tables {
		t.Fatalf("view table manager = %p, want %p", manager.views.tables, manager.tables)
	}
	if manager.stats.tables != manager.tables {
		t.Fatalf("statistics table manager = %p, want %p", manager.stats.tables, manager.tables)
	}
	if manager.indexes.tables != manager.tables || manager.indexes.stats != manager.stats {
		t.Fatalf("index dependencies = (%p, %p), want (%p, %p)", manager.indexes.tables, manager.indexes.stats, manager.tables, manager.stats)
	}
}
