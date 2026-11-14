package metadata

import (
	"errors"
	"fmt"
	"math"

	"github.com/Rwatana/simledb_in_go/internal/record"
)

// TableManager stores the catalog layouts used to persist and retrieve table metadata.
type TableManager struct {
	tableCatalogLayout *record.Layout
	fieldCatalogLayout *record.Layout
}

// NewTableManager initializes a table manager with the table and field catalog layouts.
func NewTableManager() *TableManager {
	return &TableManager{
		tableCatalogLayout: NewTableCatalogLayout(),
		fieldCatalogLayout: NewFieldCatalogLayout(),
	}
}

// InitializeCatalogs creates and registers the table and field catalogs for a new database.
func (m *TableManager) InitializeCatalogs(transaction record.TableScanTransaction) error {
	if err := m.CreateTable("tblcat", m.tableCatalogLayout.Schema(), transaction); err != nil {
		return fmt.Errorf("initialize table catalog: %w", err)
	}
	if err := m.CreateTable("fldcat", m.fieldCatalogLayout.Schema(), transaction); err != nil {
		return fmt.Errorf("initialize field catalog: %w", err)
	}
	return nil
}

// CreateTable stores one table's schema and physical layout in the catalogs.
func (m *TableManager) CreateTable(tableName string, schema *record.Schema, transaction record.TableScanTransaction) error {
	if schema == nil {
		return fmt.Errorf("create table metadata for %q: schema is nil", tableName)
	}
	layout := record.NewLayout(schema)
	slotSize, err := metadataInteger("slot size", layout.SlotSize())
	if err != nil {
		return fmt.Errorf("create table metadata for %q: %w", tableName, err)
	}

	if err := withCatalogScan(transaction, "tblcat", m.tableCatalogLayout, func(scan *record.TableScan) error {
		if err := scan.Insert(); err != nil {
			return fmt.Errorf("insert table catalog row: %w", err)
		}
		if err := scan.SetString("tblname", tableName); err != nil {
			return fmt.Errorf("store table name: %w", err)
		}
		if err := scan.SetInt("slotsize", slotSize); err != nil {
			return fmt.Errorf("store table slot size: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create table metadata for %q: table catalog: %w", tableName, err)
	}

	if err := withCatalogScan(transaction, "fldcat", m.fieldCatalogLayout, func(scan *record.TableScan) error {
		for _, fieldName := range schema.Fields() {
			fieldType, _ := schema.TypeOf(fieldName)
			fieldLength, _ := schema.LengthOf(fieldName)
			fieldOffset, _ := layout.Offset(fieldName)
			length, err := metadataInteger("field length", fieldLength)
			if err != nil {
				return fmt.Errorf("field %q: %w", fieldName, err)
			}
			offset, err := metadataInteger("field offset", fieldOffset)
			if err != nil {
				return fmt.Errorf("field %q: %w", fieldName, err)
			}

			if err := scan.Insert(); err != nil {
				return fmt.Errorf("insert field catalog row for %q: %w", fieldName, err)
			}
			if err := scan.SetString("tblname", tableName); err != nil {
				return fmt.Errorf("store table name for field %q: %w", fieldName, err)
			}
			if err := scan.SetString("fldname", fieldName); err != nil {
				return fmt.Errorf("store field name %q: %w", fieldName, err)
			}
			if err := scan.SetInt("type", int32(fieldType)); err != nil {
				return fmt.Errorf("store field type for %q: %w", fieldName, err)
			}
			if err := scan.SetInt("length", length); err != nil {
				return fmt.Errorf("store field length for %q: %w", fieldName, err)
			}
			if err := scan.SetInt("offset", offset); err != nil {
				return fmt.Errorf("store field offset for %q: %w", fieldName, err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create table metadata for %q: field catalog: %w", tableName, err)
	}
	return nil
}

// Layout reconstructs the stored physical layout for tableName from the catalogs.
func (m *TableManager) Layout(tableName string, transaction record.TableScanTransaction) (*record.Layout, error) {
	slotSize := -1
	if err := withCatalogScan(transaction, "tblcat", m.tableCatalogLayout, func(scan *record.TableScan) error {
		for {
			found, err := scan.Next()
			if err != nil {
				return fmt.Errorf("scan table catalog: %w", err)
			}
			if !found {
				return fmt.Errorf("table %q is not registered", tableName)
			}
			name, err := scan.GetString("tblname")
			if err != nil {
				return fmt.Errorf("read registered table name: %w", err)
			}
			if name != tableName {
				continue
			}
			storedSize, err := scan.GetInt("slotsize")
			if err != nil {
				return fmt.Errorf("read slot size for table %q: %w", tableName, err)
			}
			if storedSize < 4 {
				return fmt.Errorf("table %q has invalid slot size %d", tableName, storedSize)
			}
			slotSize = int(storedSize)
			return nil
		}
	}); err != nil {
		return nil, fmt.Errorf("load layout for table %q from table catalog: %w", tableName, err)
	}

	var schema record.Schema
	offsets := make(map[string]int)
	if err := withCatalogScan(transaction, "fldcat", m.fieldCatalogLayout, func(scan *record.TableScan) error {
		for {
			found, err := scan.Next()
			if err != nil {
				return fmt.Errorf("scan field catalog: %w", err)
			}
			if !found {
				return nil
			}
			name, err := scan.GetString("tblname")
			if err != nil {
				return fmt.Errorf("read field owner table name: %w", err)
			}
			if name != tableName {
				continue
			}
			fieldName, err := scan.GetString("fldname")
			if err != nil {
				return fmt.Errorf("read field name for table %q: %w", tableName, err)
			}
			if schema.HasField(fieldName) {
				return fmt.Errorf("table %q has duplicate field metadata for %q", tableName, fieldName)
			}
			storedType, err := scan.GetInt("type")
			if err != nil {
				return fmt.Errorf("read type for table %q field %q: %w", tableName, fieldName, err)
			}
			fieldType := record.FieldType(storedType)
			if fieldType != record.IntegerField && fieldType != record.VarcharField {
				return fmt.Errorf("table %q field %q has invalid type %d", tableName, fieldName, storedType)
			}
			storedLength, err := scan.GetInt("length")
			if err != nil {
				return fmt.Errorf("read length for table %q field %q: %w", tableName, fieldName, err)
			}
			if storedLength < 0 {
				return fmt.Errorf("table %q field %q has negative length %d", tableName, fieldName, storedLength)
			}
			storedOffset, err := scan.GetInt("offset")
			if err != nil {
				return fmt.Errorf("read offset for table %q field %q: %w", tableName, fieldName, err)
			}
			if storedOffset < 4 || int(storedOffset) >= slotSize {
				return fmt.Errorf("table %q field %q has offset %d outside slot size %d", tableName, fieldName, storedOffset, slotSize)
			}
			schema.AddField(fieldName, fieldType, int(storedLength))
			offsets[fieldName] = int(storedOffset)
		}
	}); err != nil {
		return nil, fmt.Errorf("load layout for table %q from field catalog: %w", tableName, err)
	}
	return record.NewLayoutFromMetadata(&schema, offsets, slotSize), nil
}

func withCatalogScan(transaction record.TableScanTransaction, tableName string, layout *record.Layout, operation func(*record.TableScan) error) (err error) {
	scan, err := record.NewTableScan(transaction, tableName, layout)
	if err != nil {
		return fmt.Errorf("open catalog scan: %w", err)
	}
	defer func() {
		if closeErr := scan.Close(); closeErr != nil {
			closeErr = fmt.Errorf("close catalog scan: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	return operation(scan)
}

func metadataInteger(name string, value int) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%s %d exceeds int32 range", name, value)
	}
	return int32(value), nil
}
