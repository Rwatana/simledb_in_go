package query_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestSelectScanFiltersRowsAndCanRestart(t *testing.T) {
	source := &selectScanStub{rows: []map[string]query.Constant{
		{"id": query.NewIntConstant(1)},
		{"id": query.NewIntConstant(2)},
		{"id": query.NewIntConstant(2)},
		{"id": query.NewIntConstant(3)},
	}}
	scan, err := query.NewSelectScan(source, query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"),
		query.NewConstantExpression(query.NewIntConstant(2)),
	)))
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
	}

	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		for wantCount := 2; wantCount > 0; wantCount-- {
			next, err := scan.Next()
			if err != nil || !next {
				t.Fatalf("Next() = (%t, %v), want (true, nil)", next, err)
			}
			id, err := scan.GetInt("id")
			if err != nil || id != 2 {
				t.Fatalf("GetInt(id) = (%d, %v), want (2, nil)", id, err)
			}
		}
		next, err := scan.Next()
		if err != nil || next {
			t.Fatalf("Next() at end = (%t, %v), want (false, nil)", next, err)
		}
		if i == 0 {
			if err := scan.BeforeFirst(); err != nil {
				t.Fatalf("second BeforeFirst() error = %v", err)
			}
		}
	}
}

func TestSelectScanEmptyPredicateReturnsEveryRow(t *testing.T) {
	source := &selectScanStub{rows: []map[string]query.Constant{{"id": query.NewIntConstant(1)}}}
	scan, err := query.NewSelectScan(source, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if next, err := scan.Next(); err != nil || !next {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", next, err)
	}
	if err := scan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewSelectScanRejectsNilSource(t *testing.T) {
	scan, err := query.NewSelectScan(nil, query.Predicate{})
	if err == nil || scan != nil || !strings.Contains(err.Error(), "scan is nil") {
		t.Fatalf("NewSelectScan(nil) = (%v, %v), want nil scan and contextual error", scan, err)
	}
}

func TestSelectScanZeroValueReportsUninitializedSource(t *testing.T) {
	var scan query.SelectScan
	if err := scan.BeforeFirst(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value BeforeFirst() error = %v, want uninitialized error", err)
	}
	if next, err := scan.Next(); err == nil || next || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value Next() = (%t, %v), want false and uninitialized error", next, err)
	}
	if _, err := scan.GetInt("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value GetInt() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetString("name"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value GetString() error = %v, want uninitialized error", err)
	}
	if _, err := scan.GetVal("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value GetVal() error = %v, want uninitialized error", err)
	}
	if err := scan.Close(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value Close() error = %v, want uninitialized error", err)
	}
	if err := scan.SetInt("id", 1); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("zero-value SetInt() error = %v, want uninitialized error", err)
	}
	if scan.HasField("id") {
		t.Fatal("zero-value HasField(id) = true, want false")
	}
}

func TestSelectScanPropagatesSourceAndPredicateErrors(t *testing.T) {
	wantErr := errors.New("source failure")
	source := &selectScanStub{beforeFirstErr: wantErr}
	scan, err := query.NewSelectScan(source, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
	}
	if err := scan.BeforeFirst(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "position before first row") {
		t.Fatalf("BeforeFirst() error = %v, want contextual source error", err)
	}

	source = &selectScanStub{index: -1, nextErrors: map[int]error{0: wantErr}}
	scan, _ = query.NewSelectScan(source, query.Predicate{})
	if _, err := scan.Next(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "advance select scan") {
		t.Fatalf("Next() error = %v, want contextual source error", err)
	}

	source = &selectScanStub{index: -1, rows: []map[string]query.Constant{{}}, getValErrors: map[string]error{"id": wantErr}}
	scan, _ = query.NewSelectScan(source, query.NewPredicateWithTerm(query.NewTerm(
		query.NewFieldExpression("id"), query.NewConstantExpression(query.NewIntConstant(1)),
	)))
	if _, err := scan.Next(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "evaluate select predicate") {
		t.Fatalf("Next() predicate error = %v, want contextual evaluation error", err)
	}
}

func TestSelectScanDelegatesScanOperationsAndWrapsErrors(t *testing.T) {
	wantErr := errors.New("read failure")
	source := &selectScanStub{
		rows:            []map[string]query.Constant{{"id": query.NewIntConstant(1), "name": query.NewStringConstant("Ada")}},
		getIntErrors:    map[string]error{"id": wantErr},
		getStringErrors: map[string]error{"name": wantErr},
		getValErrors:    map[string]error{"id": wantErr},
		closeErr:        wantErr,
		fields:          map[string]bool{"id": true},
	}
	scan, err := query.NewSelectScan(source, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
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
	if !scan.HasField("id") || scan.HasField("missing") {
		t.Errorf("HasField results = (%t, %t), want (true, false)", scan.HasField("id"), scan.HasField("missing"))
	}
	if err := scan.Close(); !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "close select scan") {
		t.Errorf("Close() error = %v, want contextual source error", err)
	}
	if got, want := source.calls, []string{"get-int:id", "get-string:name", "get-value:id", "has-field:id", "has-field:missing", "close"}; !reflect.DeepEqual(got, want) {
		t.Errorf("source calls = %v, want %v", got, want)
	}

	successSource := &selectScanStub{rows: []map[string]query.Constant{{"id": query.NewIntConstant(3), "name": query.NewStringConstant("Ada")}}}
	successScan, err := query.NewSelectScan(successSource, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() for successful reads error = %v", err)
	}
	if got, err := successScan.GetString("name"); err != nil || got != "Ada" {
		t.Errorf("GetString(name) = (%q, %v), want (Ada, nil)", got, err)
	}
	if got, err := successScan.GetVal("id"); err != nil || got != query.NewIntConstant(3) {
		t.Errorf("GetVal(id) = (%v, %v), want integer constant 3", got, err)
	}
}

func TestSelectScanDelegatesUpdateOperations(t *testing.T) {
	wantErr := errors.New("update failure")
	source := &selectUpdateScanStub{selectScanStub: &selectScanStub{updateErrors: map[string]error{
		"set-int": wantErr, "set-string": wantErr, "set-value": wantErr, "insert": wantErr,
		"delete": wantErr, "get-rid": wantErr, "move-to-rid": wantErr,
	}}}
	scan, err := query.NewSelectScan(source, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
	}
	var updateScan query.UpdateScan = scan

	checks := []struct {
		name string
		call func() error
	}{
		{"SetInt", func() error { return updateScan.SetInt("id", 2) }},
		{"SetString", func() error { return updateScan.SetString("name", "Ada") }},
		{"SetVal", func() error { return updateScan.SetVal("id", query.NewIntConstant(2)) }},
		{"Insert", updateScan.Insert},
		{"Delete", updateScan.Delete},
		{"GetRID", func() error { _, err := updateScan.GetRID(); return err }},
		{"MoveToRID", func() error { return updateScan.MoveToRID(record.NewRID(1, 2)) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "select scan") {
				t.Fatalf("%s() error = %v, want contextual source error", check.name, err)
			}
		})
	}
	if want := []string{"set-int", "set-string", "set-value", "insert", "delete", "get-rid", "move-to-rid"}; !reflect.DeepEqual(source.updateCalls, want) {
		t.Fatalf("update calls = %v, want %v", source.updateCalls, want)
	}

	initialRID := record.NewRID(2, 3)
	updatedRID := record.NewRID(4, 5)
	successSource := &selectUpdateScanStub{selectScanStub: &selectScanStub{}, rid: initialRID}
	successScan, err := query.NewSelectScan(successSource, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() for updates error = %v", err)
	}
	if err := successScan.SetInt("id", 8); err != nil {
		t.Errorf("SetInt() error = %v", err)
	}
	if err := successScan.SetString("name", "Ada"); err != nil {
		t.Errorf("SetString() error = %v", err)
	}
	if err := successScan.SetVal("id", query.NewIntConstant(8)); err != nil {
		t.Errorf("SetVal() error = %v", err)
	}
	if err := successScan.Insert(); err != nil {
		t.Errorf("Insert() error = %v", err)
	}
	if err := successScan.Delete(); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	if got, err := successScan.GetRID(); err != nil || !got.Equal(initialRID) {
		t.Errorf("GetRID() = (%v, %v), want (%v, nil)", got, err, initialRID)
	}
	if err := successScan.MoveToRID(updatedRID); err != nil {
		t.Errorf("MoveToRID() error = %v", err)
	}
	if got, err := successScan.GetRID(); err != nil || !got.Equal(updatedRID) {
		t.Errorf("GetRID() after MoveToRID = (%v, %v), want (%v, nil)", got, err, updatedRID)
	}
	if want := []string{"set-int", "set-string", "set-value", "insert", "delete", "get-rid", "move-to-rid", "get-rid"}; !reflect.DeepEqual(successSource.updateCalls, want) {
		t.Errorf("successful update calls = %v, want %v", successSource.updateCalls, want)
	}
}

func TestSelectScanReadOnlySourceRejectsUpdates(t *testing.T) {
	scan, err := query.NewSelectScan(&selectScanStub{}, query.Predicate{})
	if err != nil {
		t.Fatalf("NewSelectScan() error = %v", err)
	}
	checks := []struct {
		name string
		call func() error
	}{
		{"SetInt", func() error { return scan.SetInt("id", 1) }},
		{"SetString", func() error { return scan.SetString("name", "Ada") }},
		{"SetVal", func() error { return scan.SetVal("id", query.NewIntConstant(1)) }},
		{"Insert", scan.Insert},
		{"Delete", scan.Delete},
		{"GetRID", func() error { _, err := scan.GetRID(); return err }},
		{"MoveToRID", func() error { return scan.MoveToRID(record.NewRID(0, 0)) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); err == nil || !strings.Contains(err.Error(), "does not support updates") {
				t.Fatalf("%s() error = %v, want unsupported-update error", check.name, err)
			}
		})
	}
}

type selectScanStub struct {
	rows            []map[string]query.Constant
	index           int
	calls           []string
	fields          map[string]bool
	beforeFirstErr  error
	nextErrors      map[int]error
	getIntErrors    map[string]error
	getStringErrors map[string]error
	getValErrors    map[string]error
	closeErr        error
	updateErrors    map[string]error
}

func (scan *selectScanStub) BeforeFirst() error {
	scan.calls = append(scan.calls, "before-first")
	if scan.beforeFirstErr != nil {
		return scan.beforeFirstErr
	}
	scan.index = -1
	return nil
}

func (scan *selectScanStub) Next() (bool, error) {
	scan.calls = append(scan.calls, "next")
	scan.index++
	if err := scan.nextErrors[scan.index]; err != nil {
		return false, err
	}
	return scan.index < len(scan.rows), nil
}

func (scan *selectScanStub) GetInt(name string) (int32, error) {
	scan.calls = append(scan.calls, "get-int:"+name)
	if err := scan.getIntErrors[name]; err != nil {
		return 0, err
	}
	value, ok := scan.rows[scan.index][name]
	if !ok {
		return 0, fmt.Errorf("missing field %q", name)
	}
	result, ok := value.AsInt()
	if !ok {
		return 0, fmt.Errorf("field %q is not integer", name)
	}
	return result, nil
}

func (scan *selectScanStub) GetString(name string) (string, error) {
	scan.calls = append(scan.calls, "get-string:"+name)
	if err := scan.getStringErrors[name]; err != nil {
		return "", err
	}
	value, ok := scan.rows[scan.index][name]
	if !ok {
		return "", fmt.Errorf("missing field %q", name)
	}
	result, ok := value.AsString()
	if !ok {
		return "", fmt.Errorf("field %q is not string", name)
	}
	return result, nil
}

func (scan *selectScanStub) GetVal(name string) (query.Constant, error) {
	scan.calls = append(scan.calls, "get-value:"+name)
	if err := scan.getValErrors[name]; err != nil {
		return query.Constant{}, err
	}
	value, ok := scan.rows[scan.index][name]
	if !ok {
		return query.Constant{}, fmt.Errorf("missing field %q", name)
	}
	return value, nil
}

func (scan *selectScanStub) HasField(name string) bool {
	scan.calls = append(scan.calls, "has-field:"+name)
	return scan.fields[name]
}

func (scan *selectScanStub) Close() error {
	scan.calls = append(scan.calls, "close")
	return scan.closeErr
}

type selectUpdateScanStub struct {
	*selectScanStub
	updateCalls []string
	rid         record.RID
}

func (scan *selectUpdateScanStub) update(operation string) error {
	scan.updateCalls = append(scan.updateCalls, operation)
	return scan.updateErrors[operation]
}

func (scan *selectUpdateScanStub) SetInt(string, int32) error     { return scan.update("set-int") }
func (scan *selectUpdateScanStub) SetString(string, string) error { return scan.update("set-string") }
func (scan *selectUpdateScanStub) SetVal(string, query.Constant) error {
	return scan.update("set-value")
}
func (scan *selectUpdateScanStub) Insert() error { return scan.update("insert") }
func (scan *selectUpdateScanStub) Delete() error { return scan.update("delete") }
func (scan *selectUpdateScanStub) GetRID() (record.RID, error) {
	if err := scan.update("get-rid"); err != nil {
		return record.RID{}, err
	}
	return scan.rid, nil
}
func (scan *selectUpdateScanStub) MoveToRID(rid record.RID) error {
	if err := scan.update("move-to-rid"); err != nil {
		return err
	}
	scan.rid = rid
	return nil
}
