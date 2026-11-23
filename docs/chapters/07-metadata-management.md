# Chapter 7: Metadata Management

This chapter builds the catalog descriptions and metadata services in the book's dependency order. Each observable
behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/metadata.NewTableCatalogLayout` | `simpledb/metadata/TableMgr.java` constructor (`tcatSchema`, `tcatLayout`) | Physical description of each `tblcat` record |
| `internal/metadata.NewFieldCatalogLayout` | `simpledb/metadata/TableMgr.java` constructor (`fcatSchema`, `fcatLayout`) | Physical description of each `fldcat` record |
| `internal/metadata.NewTableManager` | `simpledb/metadata/TableMgr.java` constructor | Retained catalog layouts for table metadata operations |

## Feature 1: Construct the table catalog layout

`NewTableCatalogLayout` declares `tblname` followed by `slotsize`, matching the order used by Java `TableMgr`.
`tblname` is a varchar limited to `MaxNameLength` (16 conceptual characters), and `slotsize` is an integer. The
existing record package derives the physical offsets and total slot width, so metadata does not duplicate UTF-8 or
occupancy-flag sizing rules.

The resulting layout places `tblname` after the four-byte record occupancy flag, places `slotsize` after the
length-prefixed maximum UTF-8 representation of the name, and occupies 76 bytes in total.

### Test-first record

The test first failed to compile because `NewTableCatalogLayout` and `MaxNameLength` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewTableCatalogLayoutDescribesTableMetadata` | Table-catalog field order, logical types, name limit, offsets, and slot size agree with the record layout rules |

## Intentional differences from Java

- Layout construction is a focused package function rather than an incidental side effect of constructing the whole
  table manager. Field-catalog construction and persistent catalog initialization remain separate behaviors.
- Field types use the Go record package's named `FieldType` values instead of `java.sql.Types` integers.
- The name limit is exported because it is shared catalog vocabulary; the layout itself exposes no mutable manager
  state.

## Feature 2: Construct the field catalog layout

`NewFieldCatalogLayout` declares `tblname`, `fldname`, `type`, `length`, and `offset` in the order stored by Java
`TableMgr`. Both names use the shared 16-character limit. The remaining fields are integers that preserve enough logical
and physical metadata to reconstruct a record layout later.

The existing record sizing rules place the two maximum-width names before the three integers, yielding offsets 4, 72,
140, 144, and 148 and a total slot width of 152 bytes. This PR only describes `fldcat`; persistent catalog creation and
layout lookup remain later behaviors.

### Test-first record

The test first failed to compile because `NewFieldCatalogLayout` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewFieldCatalogLayoutDescribesFieldMetadata` | Field-catalog order, logical types, shared name limits, physical offsets, and slot size follow the book's catalog description |

This maps the `fcatSchema` and `fcatLayout` portion of Java `TableMgr` without introducing the manager's transaction or
catalog-population side effects prematurely.

## Feature 3: Initialize the table metadata manager

`NewTableManager` constructs and retains both catalog layouts. Later table creation and lookup behaviors can therefore
share one stable physical description for each catalog instead of rebuilding layouts around individual scans.

Initialization performs no transaction, file, or record I/O. Java combines layout setup with conditional creation of
catalog tables in one constructor; the Go design separates the deterministic state from persistent side effects so
failure and cleanup behavior can be introduced and tested independently.

### Test-first record

The test first failed to compile because `NewTableManager` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewTableManagerRetainsCatalogLayouts` | A new manager owns the exact table- and field-catalog schemas, offsets, and slot sizes produced by their focused constructors |

Persistent creation of `tblcat` and `fldcat`, user table metadata insertion, and catalog lookup remain later behaviors.

## Feature 4: Store table metadata in the catalogs

`TableManager.CreateTable` derives a physical layout from the caller's schema, inserts one `tblcat` row containing the
table name and slot size, then inserts one `fldcat` row for each field in declaration order. Each field row stores the
table name, field name, logical type, conceptual length, and derived physical offset, matching Java
`TableMgr.createTable`.

The Go implementation opens and closes the two catalog scans independently. Every write and close failure receives
catalog, table, and field context; if an operation and cleanup both fail, `errors.Join` preserves both causes. Integer
metadata is range-checked before conversion to the catalogs' four-byte representation. A nil schema is rejected before
any catalog I/O.

This behavior assumes the catalog tables already exist. New-database catalog creation, duplicate-name policy, and DDL
rollback remain separate concerns.

### Test-first record

After the in-memory catalog transaction was in place, the test failed to compile because `TableManager.CreateTable` did
not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableManagerCreateTableStoresCatalogRows` | One table row and declaration-ordered field rows retain the schema's logical and derived physical metadata, and both scans close |
| `TestTableManagerCreateTablePreservesOperationAndCloseFailures` | A catalog write failure still closes its scan, preserves both causes when cleanup also fails, and stops before the next catalog |
| `TestTableManagerCreateTableRejectsNilSchemaBeforeTransactionAccess` | A nil schema returns table-specific context before a nil transaction can be accessed |
| `TestMetadataIntegerPreservesInt32Bounds` | Metadata integer conversion accepts both signed 32-bit endpoints |
| `TestMetadataIntegerRejectsValuesOutsideInt32` | Values immediately outside the signed 32-bit range return a contextual error |
| `TestTableManagerCreateTableReportsCloseFailureAfterSuccessfulTableWrite` | A successful table write followed by scan cleanup failure returns the cleanup cause |
| `TestTableManagerCreateTablePreservesCatalogScanOpenFailure` | Catalog size failure keeps its cause and stops before pinning or writing |
| `TestTableManagerCreateTableRejectsUnencodableSlotSizeBeforeTransactionAccess` | An oversized derived slot is rejected before transaction access |
| `TestTableManagerCreateTablePreservesTableSlotSizeWriteFailure` | A table catalog slot-size write failure retains context and releases the scan |
| `TestTableManagerCreateTablePreservesTableCatalogInsertFailure` | A table row occupancy failure stops before field catalog access |
| `TestTableManagerCreateTablePreservesFieldCatalogInsertFailure` | A field row occupancy failure retains context and leaves the row unused |
| `TestTableManagerCreateTablePreservesFieldCatalogTableNameWriteFailure` | A field row table-name write failure stops later writes and closes both scans |
| `TestTableManagerCreateTablePreservesFieldCatalogFieldNameWriteFailure` | A field-name write failure retains field context and closes both scans |
| `TestTableManagerCreateTablePreservesFieldCatalogTypeWriteFailure` | A field type write failure retains context and stops subsequent metadata writes |
| `TestTableManagerCreateTablePreservesFieldCatalogLengthWriteFailure` | A field length write failure retains context and stops before offset storage |
| `TestTableManagerCreateTablePreservesFieldCatalogOffsetWriteFailure` | A field offset write failure retains context after earlier metadata is stored |

## Feature 5: Initialize new database catalogs

`TableManager.InitializeCatalogs` registers `tblcat` and `fldcat` using the same `CreateTable` path as every later user
table. On an empty database, the existing table-scan initialization appends, pins, and formats each first catalog block;
the catalog layouts then describe themselves through two table rows and seven field rows.

The operation stops after the first failed catalog registration and adds bootstrap context without hiding the underlying
scan, write, or cleanup error. Detecting whether the database is new remains composition-root policy rather than state
inside the metadata manager.

### Test-first record

The test first failed to compile because `TableManager.InitializeCatalogs` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableManagerInitializeCatalogsStoresTheirOwnMetadata` | Empty catalog files are appended and formatted once, then store complete self-describing table and field rows in declaration order |
| `TestTableManagerInitializeCatalogsPreservesTableCatalogFailure` | Failure opening `tblcat` retains bootstrap context and has no catalog side effects |
| `TestTableManagerInitializeCatalogsPreservesFieldCatalogFailure` | Failure opening `fldcat` retains bootstrap context after the table catalog row is written |

This maps the `isNew` branch of Java `TableMgr` while making bootstrap an explicit, separately testable Go operation.

## Feature 6: Reconstruct a table layout from the catalogs

`TableManager.Layout` locates the requested table's stored slot size in `tblcat`, then walks `fldcat` in record order to
rebuild the schema and offset map. It returns `record.NewLayoutFromMetadata`, preserving catalog offsets and slot width
instead of recalculating them from today's encoding rules. This maps Java `TableMgr.getLayout`.

The Go API reports an absent table explicitly instead of returning a layout with Java's `-1` sentinel size. It also
rejects slot sizes smaller than the occupancy flag, unknown field types, negative lengths, duplicate fields, and offsets
outside the stored slot. Both catalog scans use the shared cleanup boundary, so close failures remain visible.

An empty-schema table is valid: it has a four-byte slot and no matching `fldcat` rows. Layout caching remains outside
this behavior.

### Test-first record

The tests first failed to compile because `TableManager.Layout` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableManagerLayoutReconstructsStoredPhysicalMetadata` | Stored catalog order, types, lengths, offsets, and slot size reconstruct the field-catalog layout and both scans close |
| `TestTableManagerLayoutReportsMissingTableAndClosesTableCatalog` | An absent table returns contextual failure after closing `tblcat` without opening `fldcat` |
| `TestTableManagerLayoutPreservesTableCatalogScanFailure` | An occupancy-read failure in `tblcat` retains its cause and closes the scan |
| `TestTableManagerLayoutPreservesRegisteredTableNameReadFailure` | A table-name read failure retains catalog context and closes `tblcat` |
| `TestTableManagerLayoutPreservesTableCatalogSlotSizeReadFailure` | A slot-size read failure retains the requested-table context and closes `tblcat` |
| `TestTableManagerLayoutRejectsInvalidTableCatalogSlotSize` | A stored slot size smaller than the occupancy flag is rejected |
| `TestTableManagerLayoutPreservesFieldCatalogScanFailure` | An occupancy-read failure in `fldcat` retains its cause and closes both scans |
| `TestTableManagerLayoutPreservesFieldOwnerTableNameReadFailure` | A field owner-name read failure retains table context and closes both scans |
| `TestTableManagerLayoutPreservesFieldNameReadFailure` | A field-name read failure retains field-catalog context and closes both scans |
| `TestTableManagerLayoutRejectsDuplicateFieldMetadata` | Duplicate stored field names are rejected while both catalog scans close |
| `TestTableManagerLayoutPreservesFieldTypeReadFailure` | A field-type read failure retains its cause and closes both scans |
| `TestTableManagerLayoutRejectsInvalidFieldType` | An unknown stored field type is rejected |
| `TestTableManagerLayoutPreservesFieldLengthReadFailure` | A field-length read failure retains its cause and closes both scans |
| `TestTableManagerLayoutRejectsNegativeFieldLength` | A negative stored field length is rejected |
| `TestTableManagerLayoutPreservesFieldOffsetReadFailure` | A field-offset read failure retains its cause and closes both scans |
| `TestTableManagerLayoutRejectsFieldOffsetsOutsideStoredSlot` | A field offset beyond the stored slot is rejected |
