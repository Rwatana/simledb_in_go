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
| `internal/metadata.NewStatManager` | `simpledb/metadata/StatMgr.java` constructor | Statistics cache state and table metadata dependency |

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

## Feature 13: Report estimated table blocks accessed

`StatInfo.BlocksAccessed` returns the stored observed block count unchanged. It preserves zero for an empty observation
and performs no mutation or I/O.

### Test-first record

The test first failed to compile because `StatInfo.BlocksAccessed` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatInfoBlocksAccessedReturnsObservedBlocks` | Positive and zero observed block counts are returned exactly |

This maps Java `StatInfo.blocksAccessed` directly.

## Feature 14: Report estimated table records output

`StatInfo.RecordsOutput` returns the stored observed record count unchanged. Positive and zero cardinalities require no
recalculation, mutation, or I/O.

### Test-first record

The test first failed to compile because `StatInfo.RecordsOutput` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatInfoRecordsOutputReturnsObservedRecords` | Positive and zero observed record counts are returned exactly |

This maps Java `StatInfo.recordsOutput` directly.

## Feature 15: Estimate distinct field values

`StatInfo.DistinctValues` implements the book's intentionally rough `1 + records / 3` estimate with integer division.
It therefore returns at least one for zero through two records and increases once per three observed records. The field
name does not affect the guess.

### Test-first record

The test first failed to compile because `StatInfo.DistinctValues` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatInfoDistinctValuesUsesBookEstimate` | Integer-division boundaries and field-name independence produce the book's lower-bounded estimate |

This maps Java `StatInfo.distinctValues`. Real per-field cardinality collection remains intentionally outside this
educational manager. Java `StatInfo`'s public behavior column is now complete.

## Feature 16: Initialize the statistics manager

`NewStatManager` retains the table metadata manager and starts with an allocated empty statistics cache and zero lookup
calls. A mutex is part of the state from the beginning so later cache and refresh operations can preserve Java's
synchronized behavior without changing ownership.

Unlike Java `StatMgr(TableMgr, Transaction)`, the Go constructor performs no full-database scan. Initial refresh is a
fallible I/O behavior and remains separate from deterministic dependency setup.

### Test-first record

The test first failed to compile because `NewStatManager` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewStatManagerStartsWithEmptyCache` | Initialization retains the exact table manager and allocates empty cache state with zero calls |

## Feature 17: Calculate statistics for one table

`StatManager.calculateTableStats` scans every occupied record in a supplied table layout. It increments the record count
for each row and uses the current RID's zero-based block number plus one as the observed block count. A table with no
occupied rows therefore reports zero blocks, matching Java `StatMgr.calcTableStats` even though table-scan initialization
may have formatted an empty physical block.

The scan closes on exhaustion or failure, and errors retain table and record context. This focused calculation does not
change the manager cache or lookup counter; refresh and lookup policy remain later behaviors.

### Test-first record

After the multi-block fixture was prepared, the test first failed to compile because `calculateTableStats` did not
exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatManagerCalculateTableStatsCountsRecordsAndBlocks` | Nine occupied records spanning two blocks produce `(2 blocks, 9 records)` without mutating cache state |

## Feature 18: Refresh all table statistics

`StatManager.Refresh` reconstructs the `tblcat` layout, walks every registered table name, reconstructs each table's
layout, and calculates a fresh `StatInfo`. Only after the entire scan succeeds does it replace the cache and reset the
lookup counter to zero, so a partial refresh cannot discard the previous usable snapshot.

The manager mutex covers the refresh and state commit, preserving Java's synchronized semantics. The outer table-catalog
scan and every nested layout/table scan close through the existing cleanup boundary with table context.

### Test-first record

The test first failed to compile because `StatManager.Refresh` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatManagerRefreshRebuildsEveryRegisteredTable` | Refresh replaces stale cache state with `tblcat`, `fldcat`, and user-table snapshots and resets the call counter |

This maps Java `StatMgr.refreshStatistics` while exposing the fallible refresh explicitly instead of hiding it in a
constructor.

## Feature 19: Cache statistics on first lookup

`StatManager.Statistics` increments the lookup counter and returns an existing table snapshot when present. On a cache
miss it scans exactly that table with the caller-supplied layout, installs the completed `StatInfo`, and returns the same
pointer that future lookups reuse. A failed scan retains table context and never installs a partial result.

The manager mutex covers both the cache decision and first calculation, so concurrent first lookups cannot publish
competing snapshots. Automatic periodic refresh remains a separate policy behavior.

### Test-first record

The test first failed to compile because `StatManager.Statistics` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStatManagerStatisticsCachesFirstLookup` | The first lookup calculates one-row statistics, while a later lookup after another insert returns the same cached snapshot and increments the call counter |

This maps the cache-hit and cache-miss portion of Java `StatMgr.getStatInfo`; the Go method returns explicit I/O errors.

## Feature 20: Refresh statistics after one hundred lookups

`StatManager.Statistics` now triggers a full refresh when the incremented lookup counter exceeds one hundred. The 101st
lookup therefore rebuilds every registered table snapshot before selecting the requested entry, and the successful
refresh resets the counter to zero. The first one hundred lookups continue to use the existing cache.

The public `Refresh` method and threshold path share `refreshLocked`: callers acquire the manager mutex exactly once,
avoiding recursive locking while preserving the same atomic cache replacement and failure behavior.

### Test-first record

The test first returned the deliberately stale 99-record snapshot on lookup 101 instead of refreshing it.

| Test | Observable behavior |
| --- | --- |
| `TestStatManagerStatisticsRefreshesOnHundredFirstLookup` | Lookup 101 replaces the stale cache with all registered table snapshots, returns the fresh two-row table estimate, and resets the counter |

This completes the refresh-threshold portion of Java `StatMgr.getStatInfo`.

## Feature 21: Initialize index metadata

`NewIndexInfo` retains an index's name, indexed field, table schema, transaction, and table statistics. It also constructs
the physical index-record layout in book order: integer `block`, integer `id`, then `dataval` with the indexed table
field's exact logical type and varchar length.

Initialization validates the dependencies and field name instead of allowing a later nil dereference. The concrete
`HashIndex`/`BTreeIndex` opening behavior remains deferred to Chapters 12–13; Chapter 7 needs only metadata and cost
descriptions, so no future index interface or implementation is introduced here.

### Test-first record

The test first failed to compile because `NewIndexInfo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewIndexInfoBuildsIndexRecordLayout` | Integer and varchar indexed fields produce `[block, id, dataval]` layouts while retaining the supplied metadata dependencies |
| `TestNewIndexInfoRejectsInvalidMetadata` | Nil dependencies and a missing indexed field return contextual initialization errors |

This maps Java `IndexInfo` construction and `createIdxLayout` while making invalid metadata an explicit Go error.

## Feature 22: Estimate index block accesses

`IndexInfo.BlocksAccessed` divides the transaction block size by the index-record slot size to estimate records per
block, divides the table cardinality by that value to estimate total index blocks, and finally divides by the book's 100
static-hash buckets. Each division deliberately uses integer truncation, matching Java `IndexInfo.blocksAccessed` and
`HashIndex.searchCost` exactly.

Only the cost formula lives in metadata. Opening or traversing a concrete hash index remains deferred to Chapter 12.
Unlike Java's unchecked division, the Go method returns contextual errors when block size is unavailable or one index
record cannot fit in a block.

### Test-first record

The tests first failed to compile because `IndexInfo.BlocksAccessed` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexInfoBlocksAccessedEstimatesStaticHashBucketCost` | 25,000 integer index records in 400-byte blocks estimate ten block accesses after records-per-block and 100-bucket division |
| `TestIndexInfoBlocksAccessedRejectsUnavailableOrTooSmallBlocks` | Block-size failures and undersized blocks return contextual errors instead of panicking |

## Feature 23: Estimate indexed records output

`IndexInfo.RecordsOutput` divides the underlying table cardinality by `StatInfo`'s distinct-value estimate for the
indexed field. Both operations use the same immutable statistics snapshot retained at initialization, and integer
division deliberately truncates like Java `IndexInfo.recordsOutput`.

The `StatInfo.DistinctValues` estimate is always at least one, so this calculation requires no exceptional zero-divisor
path and performs no transaction or index I/O.

### Test-first record

The test first failed to compile because `IndexInfo.RecordsOutput` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexInfoRecordsOutputEstimatesRowsPerSearchKey` | Twelve table rows and five estimated distinct indexed values produce two matching rows after integer division |

## Feature 24: Estimate distinct values through an index

`IndexInfo.DistinctValues` returns one for the indexed field because an index scan positioned on a search key fixes that
field's output value. For every other requested field it delegates to the retained table statistics estimate, matching
Java `IndexInfo.distinctValues` and its intentionally coarse `StatInfo` model.

This estimate is deterministic and performs no transaction, catalog, or index I/O.

### Test-first record

The test first failed to compile because `IndexInfo.DistinctValues` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexInfoDistinctValuesFixesIndexedSearchKey` | The indexed field reports one distinct output value while another field retains the table estimate of five |

## Feature 25: Construct the index catalog schema

`NewIndexCatalogSchema` declares the three strings persisted for each index registration in book order: `indexname`,
`tablename`, then `fieldname`. Every field uses the table catalog's shared `MaxNameLength`, preserving the Java
`IndexMgr` constructor's logical schema without coupling schema construction to database startup.

This behavior performs no catalog, transaction, or concrete index I/O. Registering `idxcat`, retaining manager
dependencies, and storing index rows remain separate observable behaviors so the stacked changes stay independently
testable.

### Test-first record

The test first failed to compile because `NewIndexCatalogSchema` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewIndexCatalogSchemaDescribesStoredIndexRegistrations` | The index catalog exposes three ordered varchar fields, each with `MaxNameLength` |

## Feature 26: Initialize the index metadata manager

`NewIndexManager` retains the table and statistics managers that later index-catalog operations need. Construction is
deterministic and performs no catalog, transaction, or concrete index I/O. This separates the Java `IndexMgr`
constructor's dependency wiring from its new-database registration and existing-catalog layout lookup.

The manager keeps the concrete Chapter 7 metadata dependencies because they already provide the exact catalog and
statistics behaviors needed; introducing interfaces before a second implementation exists would add no useful boundary.

### Test-first record

The test first failed to compile because `NewIndexManager` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewIndexManagerRetainsMetadataDependencies` | Construction preserves the supplied table and statistics manager identities without requiring a transaction |

## Feature 27: Register the index catalog

`IndexManager.InitializeCatalog` registers `idxcat` through the retained table manager and the independently tested
`NewIndexCatalogSchema`. `TableManager.CreateTable` persists one table-catalog row plus the `indexname`, `tablename`,
and `fieldname` field rows in schema order, without opening or modifying any concrete index.

This maps the new-database branch of Java `IndexMgr` while keeping startup policy outside the manager constructor. A
missing table manager and catalog failures return contextual errors rather than producing a later nil dereference.

### Test-first record

The tests first failed to compile because `IndexManager.InitializeCatalog` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexManagerInitializeCatalogRegistersIndexMetadata` | Registration stores the `idxcat` table row and its three ordered field rows without appending an index data block |
| `TestIndexManagerInitializeCatalogRejectsMissingTableManager` | Registration reports a missing table-manager dependency instead of panicking |

## Feature 28: Store an index registration

`IndexManager.CreateIndex` reconstructs the `idxcat` layout through `TableManager`, inserts one catalog row, and writes
the supplied index, table, and field names. The shared catalog-scan boundary closes the scan on both success and failure,
and each read/write stage adds index-specific context to returned errors.

Like Java `IndexMgr.createIndex`, this operation records metadata only. It does not build, open, or traverse a concrete
hash or B-tree index; those physical structures remain Chapter 12 and Chapter 13 concerns.

### Test-first record

The test first failed to compile because `IndexManager.CreateIndex` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexManagerCreateIndexStoresRegistrationInNewIndexCatalogFile` | Creating an index appends the first `idxcat` block and stores all three names in one occupied row |

## Feature 29: Look up index metadata by table

`IndexManager.IndexInfo` scans `idxcat`, filters registrations by their stored table name, and returns `IndexInfo`
values keyed by indexed field name. Each match reconstructs the target table layout, obtains its cached or freshly
calculated statistics, and builds the Chapter 7 cost metadata without opening a concrete index.

The result is an initialized empty map when no registration matches. Missing dependencies and every catalog, layout,
statistics, metadata-construction, and cleanup failure receive table or field context. Duplicate field registrations
retain Java `IndexMgr.getIndexInfo` map semantics: the later catalog row replaces the earlier value for that key.

### Test-first record

The test first failed to compile because `IndexManager.IndexInfo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIndexManagerIndexInfoReturnsRegistrationsForRequestedTable` | Lookup returns only the requested table's two indexes, keyed by field, and returns an empty map for an unindexed table |

## Feature 30: Initialize the metadata facade

`NewMetadataManager` constructs the Chapter 7 facade's table, view, statistics, and index managers. The view and
statistics managers share the same table manager, while the index manager shares both that table manager and the same
statistics cache. Construction performs no catalog or transaction I/O.

This retains Java `MetadataMgr`'s dependency graph while moving its new-database startup work into a separate behavior.
The Go managers are instance fields rather than Java's static fields, so independent database instances cannot overwrite
each other's metadata state.

### Test-first record

The test first failed to compile because `NewMetadataManager` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewMetadataManagerWiresSharedDependencies` | The facade creates all four managers and shares the table and statistics dependencies by identity |

## Feature 31: Initialize all metadata catalogs

`MetadataManager.InitializeCatalogs` runs the new-database startup sequence through the facade. It first initializes
the self-describing `tblcat` and `fldcat` catalogs, then registers `viewcat`, and finally registers `idxcat`. The order
ensures every later manager can persist its logical schema through an already usable table manager.

After the sequence, all four catalog layouts can be reconstructed from `tblcat` and `fldcat`; no view or index data file
is created until its first stored definition or registration. Each startup stage adds facade context to its error.

### Test-first record

The test first failed to compile because `MetadataManager.InitializeCatalogs` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestMetadataManagerInitializeCatalogsRegistersCompleteCatalogSet` | Startup makes the table, field, view, and index catalog layouts reconstructable while appending only the two physical self-describing catalog files |

## Feature 32: Create a table through the metadata facade

`MetadataManager.CreateTable` delegates the supplied name, logical schema, and transaction to the facade's shared table
manager. The resulting table and field catalog rows remain reconstructable by the same manager, while any catalog error
is wrapped with the requested table name and facade context.

This is the Go facade counterpart of Java `MetadataMgr.createTable`; it adds no second catalog-write path or duplicate
schema calculation.

### Test-first record

The test first failed to compile because `MetadataManager.CreateTable` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestMetadataManagerCreateTableStoresSchemaThroughFacade` | Creating `student` through the facade persists an ordered integer/varchar schema that the table manager can reconstruct |

## Feature 33: Look up a table layout through the metadata facade

`MetadataManager.Layout` delegates a table name and transaction to the shared table manager, returning the reconstructed
physical layout unchanged. Catalog failures keep their detailed lower-level cause and gain the requested table name plus
facade context.

This maps Java `MetadataMgr.getLayout` without caching a second copy of catalog-derived layouts in the facade.

### Test-first record

The test first failed to compile because `MetadataManager.Layout` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestMetadataManagerLayoutReconstructsTableThroughFacade` | Lookup returns the stored field order and exact slot size for a table created through the facade |

## Feature 34: Create a view through the metadata facade

`MetadataManager.CreateView` delegates the supplied view name, SQL definition, and transaction to the shared view
manager. The existing `ViewManager` remains the single owner of `viewcat` layout lookup, row insertion, field writes,
and scan cleanup; the facade adds the requested view name to any returned error.

This maps Java `MetadataMgr.createView` without duplicating view-catalog behavior in the facade.

### Test-first record

The test first failed to compile because `MetadataManager.CreateView` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestMetadataManagerCreateViewStoresDefinitionThroughFacade` | Creating `seniors` through the facade stores the exact SQL definition that the shared view manager reads back |
