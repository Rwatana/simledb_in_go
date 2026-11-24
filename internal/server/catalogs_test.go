package server

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestManagersInitializeCatalogsRegistersCompleteCatalogSet(t *testing.T) {
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	if err := managers.InitializeCatalogs(transaction); err != nil {
		t.Fatalf("InitializeCatalogs() error = %v", err)
	}
	wantFields := map[string][]string{
		"tblcat":  {"tblname", "slotsize"},
		"fldcat":  {"tblname", "fldname", "type", "length", "offset"},
		"viewcat": {"viewname", "viewdef"},
		"idxcat":  {"indexname", "tablename", "fieldname"},
	}
	adapter := tableScanTransaction{transaction: transaction}
	for tableName, fields := range wantFields {
		layout, err := managers.Metadata.Layout(tableName, adapter)
		if err != nil {
			t.Fatalf("Layout(%q) error = %v", tableName, err)
		}
		if got := layout.Schema().Fields(); !reflect.DeepEqual(got, fields) {
			t.Errorf("Layout(%q) fields = %v, want %v", tableName, got, fields)
		}
	}
	if _, err := transaction.Commit(); err != nil {
		t.Fatalf("Commit() after catalog initialization error = %v", err)
	}

	for _, name := range []string{"tblcat.tbl", "fldcat.tbl"} {
		blocks, err := managers.Files.Length(name)
		if err != nil {
			t.Fatalf("Length(%q) error = %v", name, err)
		}
		if blocks == 0 {
			t.Errorf("Length(%q) = 0, want initialized catalog file", name)
		}
	}
}

func TestManagersInitializeCatalogsRejectsMissingState(t *testing.T) {
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 400, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	for _, test := range []struct {
		name     string
		managers *Managers
	}{
		{name: "nil manager set"},
		{name: "incomplete manager set", managers: &Managers{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.managers.InitializeCatalogs(transaction); err == nil {
				t.Fatal("InitializeCatalogs() error = nil, want manager validation error")
			} else if !strings.Contains(err.Error(), "manager") {
				t.Fatalf("InitializeCatalogs() error = %q, want manager context", err)
			}
		})
	}

	if err := managers.InitializeCatalogs(nil); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Fatalf("InitializeCatalogs(nil) error = %v, want transaction validation error", err)
	}
}

func TestManagersInitializeCatalogsReportsMetadataCreationFailure(t *testing.T) {
	managers, err := NewManagers(filepath.Join(t.TempDir(), "database"), 48, 8)
	if err != nil {
		t.Fatalf("NewManagers() error = %v", err)
	}
	transaction, err := managers.NewTransaction()
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	err = managers.InitializeCatalogs(transaction)
	if err == nil || !strings.Contains(err.Error(), "initialize server metadata catalogs") {
		t.Fatalf("InitializeCatalogs() error = %v, want wrapped catalog initialization error", err)
	}
}
