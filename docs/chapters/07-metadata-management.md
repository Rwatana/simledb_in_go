# Chapter 7: Metadata Management

This chapter builds the catalog descriptions and metadata services in the book's dependency order. Each observable
behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/metadata.NewTableCatalogLayout` | `simpledb/metadata/TableMgr.java` constructor (`tcatSchema`, `tcatLayout`) | Physical description of each `tblcat` record |
| `internal/metadata.NewFieldCatalogLayout` | `simpledb/metadata/TableMgr.java` constructor (`fcatSchema`, `fcatLayout`) | Physical description of each `fldcat` record |
| `internal/metadata.NewTableManager` | `simpledb/metadata/TableMgr.java` constructor | Retained catalog layouts for table metadata operations |
| `internal/metadata.NewViewCatalogSchema` | `simpledb/metadata/ViewMgr.java` constructor | Logical description of each `viewcat` record |
| `internal/metadata.NewStatInfo` | `simpledb/metadata/StatInfo.java` constructor | Immutable observed block and record counts |

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

## Feature 7: Construct the view catalog schema

`NewViewCatalogSchema` declares `viewname` followed by `viewdef`, matching Java `ViewMgr`. View names reuse the shared
16-character catalog-name limit, while SQL definitions use the book's 100-character limit.

The function returns only a logical schema and performs no transaction or catalog I/O. Registering `viewcat`, inserting
view definitions, and looking them up remain separate observable behaviors.

### Test-first record

The test first failed to compile because `NewViewCatalogSchema` and `MaxViewDefinitionLength` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewViewCatalogSchemaDescribesStoredViewDefinitions` | View catalog field order, varchar types, shared name limit, and definition limit match the book |

This extracts the deterministic schema portion of Java `ViewMgr(boolean, TableMgr, Transaction)` before adding its
new-database side effect.

## Feature 8: Initialize the view metadata manager

`NewViewManager` retains the supplied `TableManager` for later `viewcat` registration and lookup. It performs no
transaction, file, or catalog I/O, keeping dependency setup independently testable from the new-database side effect in
Java's constructor.

### Test-first record

The test first failed to compile because `NewViewManager` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewViewManagerRetainsTableManager` | A new view manager retains the exact table metadata manager supplied by its caller |

The Go constructor intentionally accepts only the dependency it can initialize without failure. Registering `viewcat`
remains the next explicit behavior.

## Feature 9: Register the view catalog

`ViewManager.InitializeCatalog` registers `viewcat` through its retained `TableManager` and the focused view catalog
schema. This reuses the existing table and field catalog encoding, scan cleanup, and error handling instead of teaching
the view manager how those catalogs are stored.

The method is an explicit new-database operation rather than Java's `isNew` constructor branch. A missing table manager
is rejected before transaction access, and registration failures gain view-bootstrap context.

### Test-first record

The test first failed to compile because `ViewManager.InitializeCatalog` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestViewManagerInitializeCatalogRegistersViewMetadata` | The existing catalogs gain one `viewcat` table row and two ordered field rows without appending unrelated files |

Creating and reading individual view definitions remain separate behaviors.

## Feature 10: Store a view definition

`ViewManager.CreateView` reconstructs the stored `viewcat` layout through `TableManager`, opens a table scan, inserts one
record, and writes `viewname` and `viewdef`. If the view table file is empty, the existing scan initialization appends,
pins, and formats its first block before insertion.

The view scan uses the shared cleanup boundary, preserving operation and close failures with view-name context. SQL
parsing, validation, and duplicate-name policy remain outside metadata storage, as in the book's focused `ViewMgr`.

### Test-first record

The test first failed to compile because `ViewManager.CreateView` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestViewManagerCreateViewStoresDefinitionInNewViewFile` | A first view creates one formatted `viewcat` block and stores its name and SQL definition in the first occupied slot |

This maps Java `ViewMgr.createView` while adding explicit layout, scan, and cleanup error propagation.

## Feature 11: Look up a view definition

`ViewManager.ViewDefinition` reconstructs the `viewcat` layout, scans occupied rows in record order, and returns the
first definition whose `viewname` matches. Exhaustion returns an empty value with `found == false`; a stored empty SQL
string remains distinguishable through `found == true`.

The scan closes after a match, exhaustion, or read failure. Layout, scan, field-read, and cleanup errors retain the
requested view name as context. Duplicate-name policy remains unchanged: the first catalog row wins, matching Java's
loop and break behavior.

### Test-first record

The test first failed to compile because `ViewManager.ViewDefinition` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestViewManagerViewDefinitionFindsMatchAndReportsAbsence` | Lookup skips earlier non-matches, returns the matching SQL definition, and reports catalog exhaustion without an error |

Go uses an explicit `(string, bool, error)` result instead of Java's nullable string, separating absence from failures.
Java `ViewMgr`'s public behavior column is now complete.

## Feature 12: Initialize table statistics

`NewStatInfo` retains the observed block and record counts that later planner estimates consume. Zero and positive counts
are preserved exactly, and the fields remain private so callers cannot mutate a shared statistics snapshot.

Initialization performs no transaction, catalog, or table I/O. The accessors and the book's deliberately simple
distinct-value guess remain separate observable behaviors.

### Test-first record

The test first failed to compile because `NewStatInfo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewStatInfoRetainsObservedCounts` | Initialization preserves supplied non-empty and empty block/record observations exactly |

This maps Java `StatInfo(int, int)` to a Go value with package-private immutable state.
