package embedded

import (
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
)

func TestNewStatementRowsValidatesScanAndSchema(t *testing.T) {
	var schema record.Schema
	if rows, err := newStatementRows(nil, &schema); rows != nil || err == nil || !strings.Contains(err.Error(), "scan") {
		t.Errorf("newStatementRows(nil, schema) = (%v, %v), want scan error", rows, err)
	}
	if rows, err := newStatementRows(&statementRowsScanStub{}, nil); rows != nil || err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("newStatementRows(scan, nil) = (%v, %v), want schema error", rows, err)
	}
}

func TestStatementRowsReturnsTypedValuesAndClosesUnderlyingScan(t *testing.T) {
	var schema record.Schema
	schema.AddVarcharField("name", 20)
	schema.AddIntegerField("id")
	scan := &statementRowsScanStub{available: true, name: "Ada", id: 7}
	rows, err := newStatementRows(scan, &schema)
	if err != nil {
		t.Fatalf("newStatementRows() error = %v", err)
	}
	if nilRows := (*statementRows)(nil).Columns(); nilRows != nil {
		t.Errorf("nil Columns() = %v, want nil", nilRows)
	}
	columns := rows.Columns()
	if want := []string{"name", "id"}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("Columns() = %v, want %v", columns, want)
	}
	columns[0] = "changed"
	if got, want := rows.Columns(), []string{"name", "id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Columns() after caller mutation = %v, want defensive copy %v", got, want)
	}
	destination := make([]driver.Value, 2)
	if err := rows.Next(destination); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got, want := destination, []driver.Value{"Ada", int64(7)}; !reflect.DeepEqual(got, want) {
		t.Errorf("Next() values = %#v, want %#v", got, want)
	}
	scan.available = false
	if err := rows.Next(destination); !errors.Is(err, io.EOF) {
		t.Errorf("Next() at end error = %v, want io.EOF", err)
	}
	if err := rows.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if err := rows.Close(); err != nil || scan.closeCalls != 1 {
		t.Errorf("repeated Close() = %v with %d scan closes, want nil and one close", err, scan.closeCalls)
	}
}

func TestStatementRowsReportsDatabaseTypeNames(t *testing.T) {
	var schema record.Schema
	schema.AddVarcharField("name", 20)
	schema.AddIntegerField("id")
	rows := &statementRows{schema: &schema, columns: schema.Fields()}
	var driverRows driver.Rows = rows
	metadata, ok := driverRows.(driver.RowsColumnTypeDatabaseTypeName)
	if !ok {
		t.Fatal("statement rows do not implement driver.RowsColumnTypeDatabaseTypeName")
	}
	for index, want := range []string{"VARCHAR", "INTEGER"} {
		if got := metadata.ColumnTypeDatabaseTypeName(index); got != want {
			t.Errorf("ColumnTypeDatabaseTypeName(%d) = %q, want %q", index, got, want)
		}
	}
	for _, index := range []int{-1, 2} {
		if got := metadata.ColumnTypeDatabaseTypeName(index); got != "" {
			t.Errorf("ColumnTypeDatabaseTypeName(%d) = %q, want empty", index, got)
		}
	}
	if got := (*statementRows)(nil).ColumnTypeDatabaseTypeName(0); got != "" {
		t.Errorf("nil ColumnTypeDatabaseTypeName(0) = %q, want empty", got)
	}
	if got := (&statementRows{columns: []string{"name"}}).ColumnTypeDatabaseTypeName(0); got != "" {
		t.Errorf("missing-schema ColumnTypeDatabaseTypeName(0) = %q, want empty", got)
	}

	missingField := &statementRows{schema: &record.Schema{}, columns: []string{"missing"}}
	if got := missingField.ColumnTypeDatabaseTypeName(0); got != "" {
		t.Errorf("missing-field ColumnTypeDatabaseTypeName(0) = %q, want empty", got)
	}

	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("unknown", record.FieldType(99), 0)
	unsupported := &statementRows{schema: &unsupportedSchema, columns: unsupportedSchema.Fields()}
	if got := unsupported.ColumnTypeDatabaseTypeName(0); got != "" {
		t.Errorf("unsupported ColumnTypeDatabaseTypeName(0) = %q, want empty", got)
	}
}

func TestStatementRowsReportsScanTypes(t *testing.T) {
	var schema record.Schema
	schema.AddVarcharField("name", 20)
	schema.AddIntegerField("id")
	rows := &statementRows{schema: &schema, columns: schema.Fields()}
	var driverRows driver.Rows = rows
	metadata, ok := driverRows.(driver.RowsColumnTypeScanType)
	if !ok {
		t.Fatal("statement rows do not implement driver.RowsColumnTypeScanType")
	}
	for index, want := range []reflect.Type{reflect.TypeOf(""), reflect.TypeOf(int64(0))} {
		if got := metadata.ColumnTypeScanType(index); got != want {
			t.Errorf("ColumnTypeScanType(%d) = %v, want %v", index, got, want)
		}
	}
	for _, index := range []int{-1, 2} {
		if got := metadata.ColumnTypeScanType(index); got != nil {
			t.Errorf("ColumnTypeScanType(%d) = %v, want nil", index, got)
		}
	}
	if got := (*statementRows)(nil).ColumnTypeScanType(0); got != nil {
		t.Errorf("nil ColumnTypeScanType(0) = %v, want nil", got)
	}
	if got := (&statementRows{columns: []string{"name"}}).ColumnTypeScanType(0); got != nil {
		t.Errorf("missing-schema ColumnTypeScanType(0) = %v, want nil", got)
	}

	missingField := &statementRows{schema: &record.Schema{}, columns: []string{"missing"}}
	if got := missingField.ColumnTypeScanType(0); got != nil {
		t.Errorf("missing-field ColumnTypeScanType(0) = %v, want nil", got)
	}

	var unsupportedSchema record.Schema
	unsupportedSchema.AddField("unknown", record.FieldType(99), 0)
	unsupported := &statementRows{schema: &unsupportedSchema, columns: unsupportedSchema.Fields()}
	if got := unsupported.ColumnTypeScanType(0); got != nil {
		t.Errorf("unsupported ColumnTypeScanType(0) = %v, want nil", got)
	}
}

func TestStatementRowsReportsInvalidStateAndScanFailures(t *testing.T) {
	destination := []driver.Value{nil}
	if err := (*statementRows)(nil).Next(destination); err == nil || !strings.Contains(err.Error(), "rows") {
		t.Errorf("nil Next() error = %v, want rows error", err)
	}
	closed := &statementRows{closed: true}
	if err := closed.Next(destination); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("closed Next() error = %v, want closed error", err)
	}
	if err := (&statementRows{}).Next(nil); err == nil || !strings.Contains(err.Error(), "scan or schema") {
		t.Errorf("uninitialized Next() error = %v, want initialization error", err)
	}
	var schema record.Schema
	schema.AddIntegerField("id")
	rows, err := newStatementRows(&statementRowsScanStub{}, &schema)
	if err != nil {
		t.Fatalf("newStatementRows() error = %v", err)
	}
	if err := rows.Next(nil); err == nil || !strings.Contains(err.Error(), "destination") {
		t.Errorf("Next() with wrong destination length error = %v, want destination error", err)
	}
	nextErr := errors.New("scan next failed")
	rows.scan = &statementRowsScanStub{nextErr: nextErr}
	if err := rows.Next(destination); !errors.Is(err, nextErr) {
		t.Errorf("Next() scan error = %v, want wrapped scan error", err)
	}
	intErr := errors.New("integer read failed")
	rows.scan = &statementRowsScanStub{available: true, intErr: intErr}
	if err := rows.Next(destination); !errors.Is(err, intErr) || !strings.Contains(err.Error(), "integer column") {
		t.Errorf("Next() integer read error = %v, want wrapped integer error", err)
	}
	stringSchema := &record.Schema{}
	stringSchema.AddVarcharField("name", 20)
	rows, err = newStatementRows(&statementRowsScanStub{available: true, stringErr: errors.New("string read failed")}, stringSchema)
	if err != nil {
		t.Fatalf("newStatementRows() string schema error = %v", err)
	}
	if err := rows.Next([]driver.Value{nil}); err == nil || !strings.Contains(err.Error(), "string column") {
		t.Errorf("Next() string read error = %v, want wrapped string error", err)
	}
	unknownSchema := &record.Schema{}
	unknownSchema.AddField("unknown", record.FieldType(99), 0)
	rows, err = newStatementRows(&statementRowsScanStub{available: true}, unknownSchema)
	if err != nil {
		t.Fatalf("newStatementRows() unknown schema error = %v", err)
	}
	if err := rows.Next([]driver.Value{nil}); err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("Next() unsupported type error = %v, want type error", err)
	}
	if err := (*statementRows)(nil).Close(); err == nil || !strings.Contains(err.Error(), "rows") {
		t.Errorf("nil Close() error = %v, want rows error", err)
	}
	if err := (&statementRows{}).Close(); err == nil || !strings.Contains(err.Error(), "scan") {
		t.Errorf("Close() without scan error = %v, want scan error", err)
	}
	closeErr := errors.New("scan close failed")
	rows, err = newStatementRows(&statementRowsScanStub{closeErr: closeErr}, &schema)
	if err != nil {
		t.Fatalf("newStatementRows() close-error schema = %v", err)
	}
	if err := rows.Close(); !errors.Is(err, closeErr) {
		t.Errorf("Close() scan error = %v, want wrapped close error", err)
	}
}

type statementRowsScanStub struct {
	available  bool
	nextErr    error
	closeErr   error
	intErr     error
	stringErr  error
	id         int32
	name       string
	closeCalls int
}

func (*statementRowsScanStub) BeforeFirst() error { return nil }

func (scan *statementRowsScanStub) Next() (bool, error) { return scan.available, scan.nextErr }

func (scan *statementRowsScanStub) GetInt(string) (int32, error) { return scan.id, scan.intErr }

func (scan *statementRowsScanStub) GetString(string) (string, error) {
	return scan.name, scan.stringErr
}

func (*statementRowsScanStub) GetVal(string) (query.Constant, error) { return query.Constant{}, nil }

func (*statementRowsScanStub) HasField(string) bool { return true }

func (scan *statementRowsScanStub) Close() error {
	scan.closeCalls++
	return scan.closeErr
}
