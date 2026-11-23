package query_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Rwatana/simledb_in_go/internal/query"
	"github.com/Rwatana/simledb_in_go/internal/record"
	"github.com/Rwatana/simledb_in_go/internal/storage"
)

type memoryQueryRecordTransaction struct {
	blockSize int
	files     map[string][][]byte
	pins      int
	sizeErr   error
	getIntErr error
	getStrErr error
	setIntErr error
	setStrErr error
}

func newMemoryQueryRecordTransaction() *memoryQueryRecordTransaction {
	return &memoryQueryRecordTransaction{blockSize: 128, files: make(map[string][][]byte)}
}

func (tx *memoryQueryRecordTransaction) Pin(block storage.BlockID) error {
	if _, err := tx.page(block); err != nil {
		return err
	}
	tx.pins++
	return nil
}

func (tx *memoryQueryRecordTransaction) BlockSize() (int, error) {
	return tx.blockSize, nil
}

func (tx *memoryQueryRecordTransaction) GetInt(block storage.BlockID, offset int) (int32, error) {
	if tx.getIntErr != nil {
		return 0, tx.getIntErr
	}
	page, err := tx.page(block)
	if err != nil {
		return 0, err
	}
	return page.GetInt(offset)
}

func (tx *memoryQueryRecordTransaction) GetString(block storage.BlockID, offset int) (string, error) {
	if tx.getStrErr != nil {
		return "", tx.getStrErr
	}
	page, err := tx.page(block)
	if err != nil {
		return "", err
	}
	value, err := page.GetBytes(offset)
	return string(value), err
}

func (tx *memoryQueryRecordTransaction) SetInt(block storage.BlockID, offset, value int32, _ bool) error {
	if tx.setIntErr != nil {
		return tx.setIntErr
	}
	page, err := tx.page(block)
	if err != nil {
		return err
	}
	if err := page.SetInt(int(offset), value); err != nil {
		return err
	}
	tx.save(block, page)
	return nil
}

func (tx *memoryQueryRecordTransaction) SetString(block storage.BlockID, offset int32, value string, _ bool) error {
	if tx.setStrErr != nil {
		return tx.setStrErr
	}
	page, err := tx.page(block)
	if err != nil {
		return err
	}
	if err := page.SetBytes(int(offset), []byte(value)); err != nil {
		return err
	}
	tx.save(block, page)
	return nil
}

func (tx *memoryQueryRecordTransaction) Size(fileName string) (int, error) {
	if tx.sizeErr != nil {
		return 0, tx.sizeErr
	}
	return len(tx.files[fileName]), nil
}

func (tx *memoryQueryRecordTransaction) Append(fileName string) (storage.BlockID, error) {
	if tx.files[fileName] == nil {
		tx.files[fileName] = make([][]byte, 0)
	}
	block := storage.NewBlockID(fileName, len(tx.files[fileName]))
	tx.files[fileName] = append(tx.files[fileName], make([]byte, tx.blockSize))
	return block, nil
}

func (tx *memoryQueryRecordTransaction) Unpin(storage.BlockID) error {
	if tx.pins == 0 {
		return errors.New("unpin without a pin")
	}
	tx.pins--
	return nil
}

func (tx *memoryQueryRecordTransaction) page(block storage.BlockID) (*storage.Page, error) {
	blocks := tx.files[block.FileName()]
	if block.Number() < 0 || block.Number() >= len(blocks) {
		return nil, fmt.Errorf("unknown block %v", block)
	}
	return storage.NewPageFromBytes(blocks[block.Number()]), nil
}

func (tx *memoryQueryRecordTransaction) save(block storage.BlockID, page *storage.Page) {
	tx.files[block.FileName()][block.Number()] = page.Snapshot()
}

func TestRecordScanAdaptsRecordTableScanToUpdateScan(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 5)
	schema.AddField("unsupported", record.FieldType(99), 0)
	layout := record.NewLayout(schema)
	tx := newMemoryQueryRecordTransaction()

	scan, err := query.NewRecordScan(tx, "student", layout)
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	var updateScan query.UpdateScan = scan
	if !updateScan.HasField("id") || updateScan.HasField("missing") {
		t.Fatal("HasField results do not match the schema")
	}
	if err := updateScan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := updateScan.SetVal("id", query.NewIntConstant(7)); err != nil {
		t.Fatalf("SetVal(id, 7) error = %v", err)
	}
	if err := updateScan.SetVal("name", query.NewStringConstant("Ada")); err != nil {
		t.Fatalf("SetVal(name, Ada) error = %v", err)
	}
	if err := updateScan.SetInt("id", 11); err != nil {
		t.Fatalf("SetInt(id, 11) error = %v", err)
	}
	if err := updateScan.SetString("name", "Grace"); err != nil {
		t.Fatalf("SetString(name, Grace) error = %v", err)
	}
	if value, err := updateScan.GetVal("id"); err != nil || value != query.NewIntConstant(11) {
		t.Fatalf("GetVal(id) = (%v, %v), want integer 11", value, err)
	}
	if value, err := updateScan.GetVal("name"); err != nil || value != query.NewStringConstant("Grace") {
		t.Fatalf("GetVal(name) = (%v, %v), want string Grace", value, err)
	}
	if rid, err := updateScan.GetRID(); err != nil || !rid.Equal(record.NewRID(0, 0)) {
		t.Fatalf("GetRID() = (%v, %v), want [0, 0]", rid, err)
	}
	if err := updateScan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() error = %v", err)
	}
	if next, err := updateScan.Next(); err != nil || !next {
		t.Fatalf("Next() = (%t, %v), want (true, nil)", next, err)
	}
	if next, err := updateScan.Next(); err != nil || next {
		t.Fatalf("Next() at end = (%t, %v), want (false, nil)", next, err)
	}
	if err := updateScan.MoveToRID(record.NewRID(0, 0)); err != nil {
		t.Fatalf("MoveToRID([0, 0]) error = %v", err)
	}
	if rid, err := updateScan.GetRID(); err != nil || !rid.Equal(record.NewRID(0, 0)) {
		t.Fatalf("GetRID() after reposition = (%v, %v), want [0, 0]", rid, err)
	}
	if err := updateScan.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := updateScan.BeforeFirst(); err != nil {
		t.Fatalf("BeforeFirst() after delete error = %v", err)
	}
	if next, err := updateScan.Next(); err != nil || next {
		t.Fatalf("Next() after delete = (%t, %v), want (false, nil)", next, err)
	}
	if err := updateScan.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if tx.pins != 0 {
		t.Errorf("outstanding pins = %d, want 0", tx.pins)
	}
}

func TestRecordScanRejectsInvalidConstantAndFieldTypes(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 5)
	schema.AddField("unsupported", record.FieldType(99), 0)
	tx := newMemoryQueryRecordTransaction()
	scan, err := query.NewRecordScan(tx, "student", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	defer scan.Close()
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	for _, fieldName := range []string{"missing", "unsupported"} {
		if _, err := scan.GetVal(fieldName); err == nil {
			t.Errorf("GetVal(%q) error = nil, want schema error", fieldName)
		}
		if err := scan.SetVal(fieldName, query.NewIntConstant(1)); err == nil {
			t.Errorf("SetVal(%q, 1) error = nil, want schema error", fieldName)
		}
	}
	if err := scan.SetVal("id", query.NewStringConstant("wrong")); err == nil || !strings.Contains(err.Error(), "integer") {
		t.Errorf("SetVal(id, string) error = %v, want integer-kind error", err)
	}
	if err := scan.SetVal("name", query.NewIntConstant(1)); err == nil || !strings.Contains(err.Error(), "string") {
		t.Errorf("SetVal(name, integer) error = %v, want string-kind error", err)
	}
}

func TestRecordScanZeroValueReturnsContextualErrors(t *testing.T) {
	var scan *query.RecordScan
	if _, err := scan.GetVal("id"); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("GetVal(id) on nil scan error = %v, want initialization error", err)
	}
	if err := scan.SetVal("id", query.NewIntConstant(1)); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("SetVal(id, 1) on nil scan error = %v, want initialization error", err)
	}
}

func TestRecordScanPreservesTypedReadAndWriteErrors(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	schema.AddVarcharField("name", 5)
	tx := newMemoryQueryRecordTransaction()
	scan, err := query.NewRecordScan(tx, "student", record.NewLayout(schema))
	if err != nil {
		t.Fatalf("NewRecordScan() error = %v", err)
	}
	defer scan.Close()
	if err := scan.Insert(); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	readErr := errors.New("read failed")
	tx.getIntErr = readErr
	if _, err := scan.GetVal("id"); !errors.Is(err, readErr) {
		t.Errorf("GetVal(id) error = %v, want wrapped read error", err)
	}
	tx.getIntErr = nil
	tx.getStrErr = readErr
	if _, err := scan.GetVal("name"); !errors.Is(err, readErr) {
		t.Errorf("GetVal(name) error = %v, want wrapped read error", err)
	}
	tx.setStrErr = readErr
	if err := scan.SetVal("name", query.NewStringConstant("Ada")); !errors.Is(err, readErr) {
		t.Errorf("SetVal(name) error = %v, want wrapped write error", err)
	}
	tx.setStrErr = nil
	tx.setIntErr = readErr
	if err := scan.SetVal("id", query.NewIntConstant(7)); !errors.Is(err, readErr) {
		t.Errorf("SetVal(id) error = %v, want wrapped write error", err)
	}
}

func TestNewRecordScanRejectsMissingDependenciesAndPreservesConstructionError(t *testing.T) {
	schema := &record.Schema{}
	schema.AddIntegerField("id")
	layout := record.NewLayout(schema)
	if _, err := query.NewRecordScan(nil, "student", layout); err == nil || !strings.Contains(err.Error(), "transaction") {
		t.Errorf("NewRecordScan(nil, layout) error = %v, want transaction error", err)
	}
	tx := newMemoryQueryRecordTransaction()
	if _, err := query.NewRecordScan(tx, "student", nil); err == nil || !strings.Contains(err.Error(), "layout") {
		t.Errorf("NewRecordScan(tx, nil) error = %v, want layout error", err)
	}
	if _, err := query.NewRecordScan(tx, "student", record.NewLayoutFromMetadata(nil, nil, 4)); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("NewRecordScan(tx, nil schema) error = %v, want schema error", err)
	}
	cause := errors.New("size failed")
	tx.sizeErr = cause
	if _, err := query.NewRecordScan(tx, "student", layout); !errors.Is(err, cause) || !strings.Contains(err.Error(), "student") {
		t.Errorf("NewRecordScan() error = %v, want table context and wrapped cause", err)
	}
}
