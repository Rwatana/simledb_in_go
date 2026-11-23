package query_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
)

func TestProjectScanDelegatesRowsAndReportsOnlyProjectedFields(t *testing.T) {
	source := &selectScanStub{
		rows: []map[string]query.Constant{{
			"id":    query.NewIntConstant(7),
			"name":  query.NewStringConstant("Ada"),
			"grade": query.NewIntConstant(95),
		}},
		fields: map[string]bool{"id": true, "name": true, "grade": true},
	}
	scan, err := query.NewProjectScan(source, []string{"id", "grade"})
	if err != nil {
		t.Fatalf("NewProjectScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", next, err)
	}
	if got, err := scan.GetInt("id"); err != nil || got != 7 {
		t.Errorf("GetInt(id) = (%d, %v), want (7, nil)", got, err)
	}
	if got, err := scan.GetString("name"); err != nil || got != "Ada" {
		t.Errorf("GetString(name) = (%q, %v), want delegated value even though name is not projected", got, err)
	}
	if got, err := scan.GetVal("grade"); err != nil || got != query.NewIntConstant(95) {
		t.Errorf("GetVal(grade) = (%v, %v), want integer constant 95", got, err)
	}
	if !scan.HasField("id") || !scan.HasField("grade") || scan.HasField("name") || scan.HasField("missing") {
		t.Fatal("HasField() does not match the projected field list")
	}
	if got, want := source.calls, []string{"before-first", "next", "get-int:id", "get-string:name", "get-value:grade"}; !reflect.DeepEqual(got, want) {
		t.Errorf("source calls = %v, want %v", got, want)
	}
	if err := scan.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestProjectScanCopiesFieldNamesAndHandlesDuplicatesAndEmptyLists(t *testing.T) {
	fields := []string{"id", "id"}
	scan, err := query.NewProjectScan(&selectScanStub{}, fields)
	if err != nil {
		t.Fatalf("NewProjectScan() error = %v", err)
	}
	fields[0] = "name"
	if !scan.HasField("id") || scan.HasField("name") {
		t.Fatal("ProjectScan field membership changed after caller mutated the input slice")
	}

	empty, err := query.NewProjectScan(&selectScanStub{}, nil)
	if err != nil {
		t.Fatalf("NewProjectScan() with empty projection error = %v", err)
	}
	if empty.HasField("") || empty.HasField("id") {
		t.Fatal("empty projection reported a field")
	}
}

func TestNewProjectScanRejectsNilSource(t *testing.T) {
	scan, err := query.NewProjectScan(nil, []string{"id"})
	if err == nil || scan != nil || !strings.Contains(err.Error(), "source scan is nil") {
		t.Fatalf("NewProjectScan(nil) = (%v, %v), want nil scan and contextual error", scan, err)
	}
}

func TestProjectScanWrapsSourceErrors(t *testing.T) {
	wantErr := errors.New("source failure")
	source := &selectScanStub{
		index:           -1,
		beforeFirstErr:  wantErr,
		nextErrors:      map[int]error{0: wantErr},
		getIntErrors:    map[string]error{"id": wantErr},
		getStringErrors: map[string]error{"name": wantErr},
		getValErrors:    map[string]error{"id": wantErr},
		closeErr:        wantErr,
	}
	scan, err := query.NewProjectScan(source, []string{"id", "name"})
	if err != nil {
		t.Fatalf("NewProjectScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "position before first row in project scan") {
		t.Errorf("BeforeFirst() error = %v, want contextual source error", err)
	}
	if next, err := scan.Next(); next || !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "advance project scan") {
		t.Errorf("Next() = (%t, %v), want false and contextual source error", next, err)
	}
	if _, err := scan.GetInt("id"); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "read integer field") {
		t.Errorf("GetInt() error = %v, want contextual source error", err)
	}
	if _, err := scan.GetString("name"); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "read string field") {
		t.Errorf("GetString() error = %v, want contextual source error", err)
	}
	if _, err := scan.GetVal("id"); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "read value field") {
		t.Errorf("GetVal() error = %v, want contextual source error", err)
	}
	if err := scan.Close(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "close project scan") {
		t.Errorf("Close() error = %v, want contextual source error", err)
	}
}

func TestProjectScanZeroValueReportsUninitializedSource(t *testing.T) {
	var scan query.ProjectScan
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("BeforeFirst() error = %v, want uninitialized error", err)
	}
	if next, err := scan.Next(); next || err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Next() = (%t, %v), want false and uninitialized error", next, err)
	}
	if _, err := scan.GetInt("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetInt() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetString("name"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetString() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetVal("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetVal() error = %v, want uninitialized error", err)
	}
	if scan.HasField("id") {
		t.Error("HasField(id) = true, want false for the zero value")
	}
	var nilScan *query.ProjectScan
	if nilScan.HasField("id") {
		t.Error("HasField(id) = true, want false for a nil receiver")
	}
	if err := scan.Close(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("Close() error = %v, want uninitialized error", err)
	}
}
