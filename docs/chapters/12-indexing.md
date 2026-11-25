# Chapter 12: Indexing

Indexes provide a second access path from a search value to the record identifiers stored in a table. This chapter
introduces the index contract before adding any concrete hash or B-tree storage. Search positioning, matching-entry
iteration, RID retrieval, insertion, deletion, and resource cleanup remain separate observable behaviors so each
implementation can preserve the same interface.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/index.Index` | `simpledb/index/Index.java` | Position by a search key, traverse matching entries, return data RIDs, mutate value/RID entries, and close resources |
| `internal/index/hash.HashIndex` | `simpledb/index/hash/HashIndex.java` | Retain the transaction, index name, record layout, and current traversal state for a static hash index |
| `internal/index/btree.BTreeIndex`, `directoryLayoutFromLeaf`, `ensureLeafFile`, `ensureDirectoryFile` | `simpledb/index/btree/BTreeIndex.java` | Retain index state; derive the directory layout; initialize files; position and advance a search cursor; read its current RID; insert and delete entries; propagate insertion splits; close retained pages |
| `internal/index/btree.Page` | `simpledb/index/btree/BTPage.java`, `simpledb/index/btree/BTreeDir.java` | Retain and pin a B-tree block; position before a sorted search key; recursively insert directory entries |
| `internal/index/query.IndexSelectScan` | `simpledb/index/query/IndexSelectScan.java` | Adapt index matches to a query record scan and expose the selected record's fields |
| `internal/index/query.IndexJoinScan` | `simpledb/index/query/IndexJoinScan.java` | Position a right-side index lookup from the current left join value |
| `internal/metadata.IndexInfo` | `simpledb/metadata/IndexInfo.java` | Open the configured static hash index using the retained transaction and index-record layout |
| `internal/plan.IndexSelectPlan` | `simpledb/index/planner/IndexSelectPlan.java` | Retain the child plan, index metadata, and search key for indexed selection |
| `internal/query.RecordScan` | `simpledb/record/TableScan.java` | Go adapter that converts typed record fields into query constants without coupling records to query values |

## Feature 1: Define the index traversal contract

`index.Index` is the Go boundary for an index implementation. `BeforeFirst` starts a search for one `query.Constant`,
`Next` advances among entries for that key, and `GetDataRID` identifies the matching table record. `Insert` and `Delete`
maintain a `(value, RID)` pair, while `Close` releases resources. The Go methods return errors for storage operations so
callers can preserve explicit failure handling; this interface does not prescribe duplicate-key ordering or a storage
algorithm. The [index-contract diagram](../diagrams/ch12-index-contract.svg) shows the search-value and RID boundary;
the editable source is [`ch12-index-contract.drawio`](../diagrams/ch12-index-contract.drawio).

### Test-first record

The first focused test run failed because the `internal/index` package and `Index` contract were undefined. A compile-time
probe implements the six methods, and a focused contract test exercises search positioning, traversal results, RID
retrieval, insertion, deletion, close, and error propagation.

## Feature 2: Map search keys to static hash buckets

`index/hash.BucketNumber` selects one of 100 fixed buckets. Integer values use their signed 32-bit value as the hash;
string values use Java `String.hashCode` over UTF-16 code units, including surrogate pairs for supplementary Unicode
characters. The remainder is normalized into `[0, 100)` so negative keys cannot select an invalid bucket. An unset
`query.Constant` returns an error instead of silently choosing a bucket. The [hash bucket diagram](../diagrams/ch12-hash-bucket.svg)
shows how a query value becomes a bucket number; its editable source is
[`ch12-hash-bucket.drawio`](../diagrams/ch12-hash-bucket.drawio).

### Test-first record

The first focused test run failed because `BucketNumber` and `BucketCount` were undefined. Tests verify positive and
negative integers, ASCII/BMP/supplementary Unicode strings, the empty-string hash, bucket bounds, and unset-value errors.

## Feature 3: Estimate static hash search cost

`hash.SearchCost` mirrors Java `HashIndex.searchCost`: it estimates the cost of a key lookup as
`numBlocks / BucketCount`, assuming the blocks are distributed evenly among 100 buckets. It accepts
`recordsPerBlock` for API parity, but the static-hash estimate does not use it.

### Test-first record

The first focused test run failed because `SearchCost` was undefined. Tests cover an empty index, estimates below and at
one bucket, integer truncation between bucket boundaries, multiple buckets, and the intentionally ignored
records-per-block argument.

## Feature 4: Initialize static hash index state

`hash.NewHashIndex` retains the table-scan transaction, index name, and index-record layout. It starts with an unset
search key and no active table scan, and does not perform transaction I/O. Bucket positioning remains a later operation,
so constructing the index does not create or pin a bucket table.

### Test-first record

The focused constructor test first failed to compile because `NewHashIndex` was undefined. It verifies retained
configuration, the unset-key/no-scan initial state, and that construction makes no table-transaction calls.

## Feature 5: Close the active bucket scan

`HashIndex.Close` delegates resource release to the active `record.TableScan`. Closing without a scan is a no-op. When
unpinning fails, the error retains its cause and the scan reference remains available for a retry; successful release
clears the reference, making repeated closes harmless.

### Test-first record

The focused close tests first failed to compile because `HashIndex.Close` was undefined. They cover the no-scan path,
successful unpin and scan clearing, repeated close, contextual error wrapping, and retry after an unpin failure.

## Feature 6: Position before a hash search key

`HashIndex.BeforeFirst` closes the previous bucket scan, uses `BucketNumber` to choose the static bucket table, and opens
a `record.TableScan` at its first record. Repositioning closes the prior pin before opening the new table. Invalid
receivers/dependencies and unset keys return explicit errors; if closing or opening fails, the previous scan is retained
only when it could not be released, and an unopened bucket is never installed as the current scan.

### Test-first record

The first focused test run failed because `HashIndex.BeforeFirst` was undefined. Tests verify integer/string bucket
selection, close-before-open order, invalid inputs without I/O, preservation after close failure, and contextual size/pin
failures with an unset search state.

## Feature 7: Find matching entries in a hash bucket

`HashIndex.Next` advances the positioned bucket `record.TableScan` until it finds a `dataval` equal to the search key.
The index-record schema selects integer or varchar reads; nonmatching entries are hash collisions and are skipped. An
exhausted bucket returns `(false, nil)`. Missing scan/key/layout state, an invalid `dataval` definition, scan errors,
and field-read errors return contextual errors while preserving wrapped storage causes. A key and field of different
types compare as nonmatches through `query.Constant.Equal`.

The [hash-index traversal diagram](../diagrams/ch12-hash-traversal.svg) shows the collision-filter loop and its two
observable outcomes; the editable source is [`ch12-hash-traversal.drawio`](../diagrams/ch12-hash-traversal.drawio).

### Test-first record

The first focused test run failed because `HashIndex.Next` was undefined. Tests exercise integer collisions with
duplicate matches, varchar matching, mismatched key/value types, bucket exhaustion, every invalid search/layout state,
and wrapped scan, slot-flag, integer-field, and varchar-field read failures.

## Feature 8: Read the data record identifier

`HashIndex.GetDataRID` reads the current index entry's integer `block` and `id` fields and returns them as a
`record.RID`. Both fields must be present and declared as integers in the index-record schema. Calls without a scan or
current record, invalid layouts, and storage failures return contextual errors; underlying field-read errors remain
available through wrapping. The index-contract diagram already shows the `GetDataRID` output boundary, so this
straight field mapping does not require a duplicate illustration.

### Test-first record

The focused test run first failed because `HashIndex.GetDataRID` was undefined. Tests cover block/slot conversion,
missing scan and current row, nil or absent schema fields, non-integer fields, and wrapped reads of both RID components.

## Feature 9: Insert a hash-index entry

`HashIndex.Insert` validates the transaction, index-record schema, key type, and signed 32-bit RID storage before
replacing the current bucket scan. It then uses `BeforeFirst` to open the key's static bucket table, inserts one record,
and writes `block`, `id`, and `dataval`. The bucket scan remains open as the index's current scan, matching the Java
stateful contract; the caller releases it with `Close`. Errors from bucket opening, slot allocation, and each field
write retain their causes and identify the index operation. New empty bucket files are opened through `record.TableScan`,
which formats the record slots before insertion.

The [hash-index insertion diagram](../diagrams/ch12-hash-insert.svg) shows the validation, old-scan release, bucket
selection, row write, and retained scan; the editable source is [`ch12-hash-insert.drawio`](../diagrams/ch12-hash-insert.drawio).

### Test-first record

The focused test run first failed because `HashIndex.Insert` was undefined. Tests verify integer and varchar values,
existing and newly created bucket tables, RID fields, the retained search key/scan, malformed schemas and values, RID
range overflow, table size/append/pin failures, scan insertion and field-write failures, and preservation of a previous
scan when repositioning cannot release it.

## Feature 10: Delete a hash-index entry by RID

`HashIndex.Delete` positions the active bucket scan at the supplied key and walks only entries whose `dataval` matches.
It reads each matching entry's data RID, deletes the first exact RID match, and returns successfully without mutation
when the key or RID is absent. The positioned scan remains available after success or a later scan error, matching the
stateful Java index contract. Positioning, iteration, RID-read, and slot-delete errors identify the index operation and
retain their causes. The existing [hash-index traversal diagram](../diagrams/ch12-hash-traversal.svg) shows the bucket
filter loop that deletion reuses, so a second diagram would duplicate that behavior.

### Test-first record

The focused test run first failed because `HashIndex.Delete` was undefined. Tests cover skipping hash collisions,
removing only the first key/RID match, leaving entries unchanged for absent keys or RIDs, retaining the positioned
scan, rejecting invalid state, and wrapping bucket-open, iteration, RID-read, and slot-delete failures.

## Feature 11: Position an index-select scan

`indexquery.NewIndexSelectScan` retains a `query.RecordScan`, index, and search key, then positions the index before the first
matching entry as Java `IndexSelectScan` does during construction. Its `BeforeFirst` operation repeats that positioning
for the retained key. Missing scans/indexes, unset keys, and index errors return contextual errors with wrapped causes;
the index remains caller-owned and is not closed by the adapter. The existing [hash-index traversal diagram](../diagrams/ch12-hash-traversal.svg)
already documents bucket positioning, so this adapter initialization does not need another diagram.

### Test-first record

The focused test run first failed because `NewIndexSelectScan` was undefined. Tests verify constructor positioning,
repositioning with the stored key, nil dependencies/receivers, unset keys, and initial/repeated index-position errors.

## Feature 12: Advance to the next indexed data row

`IndexSelectScan.Next` advances the index once. Exhaustion returns `(false, nil)` without moving the data table scan;
for an entry, the method reads its `record.RID`, moves the table scan to that row, and returns `(true, nil)`. Index,
RID-read, and table-movement failures return contextual errors with wrapped causes while retaining both scan references.
The existing [hash-index traversal diagram](../diagrams/ch12-hash-traversal.svg) already shows the entry-to-RID step,
so no duplicate illustration is added.

### Test-first record

The focused test run first failed because `IndexSelectScan.Next` was undefined. Tests cover multiple returned RIDs,
exhaustion without movement, index-next and RID-read failures, table-page movement failure, and nil or incomplete state.

## Feature 13: Read an integer from the selected data row

`IndexSelectScan.GetInt` delegates to its retained `query.RecordScan` and returns the integer value unchanged. A nil
adapter or missing table scan produces a contextual error; table-read errors are wrapped so callers retain the original
cause. The existing index traversal diagram already establishes that the adapter reads fields from the data table, so
this accessor does not need a duplicate diagram.

### Test-first record

The focused tests first failed because `IndexSelectScan.GetInt` was undefined. They verify forwarding of the current
record's integer value, contextual errors for a nil receiver or missing table scan, and preservation of a table-read
failure through error wrapping.

## Feature 14: Read a string from the selected data row

`IndexSelectScan.GetString` delegates to its retained `query.RecordScan` and returns the string value unchanged. A nil
adapter or missing table scan produces a contextual error; table-read errors are wrapped so callers retain the original
cause. As with integer access, the index traversal illustration already explains the data-table side of this adapter.

### Test-first record

The focused tests first failed because `IndexSelectScan.GetString` was undefined. They verify forwarding of the current
record's string value, contextual errors for a nil receiver or missing table scan, and preservation of a table-read
failure through error wrapping.

## Feature 15: Read a typed value from the selected data row

`IndexSelectScan.GetVal` delegates to `query.RecordScan.GetVal`, which uses the record schema to convert integer and
varchar fields into the matching `query.Constant` kind. This adapter retains `query.RecordScan` instead of the lower-level
`record.TableScan` so query-value conversion stays outside the record package. Nil state and field-read errors are
returned with index-select context while retaining wrapped causes. The existing index-traversal diagram already shows
the data-record boundary and no duplicate illustration is needed.

### Test-first record

The focused test run first failed because `IndexSelectScan.GetVal` was undefined and exposed that the adapter retained
only a low-level record scan without schema-driven value conversion. Tests verify integer and varchar constants, nil
receiver/table-scan state, and wrapped read failures.

## Feature 16: Check a field in the selected data row

`IndexSelectScan.HasField` delegates field membership to its retained `query.RecordScan`. A nil adapter, missing record
scan, or absent underlying table scan reports false without panicking. The query adapter reads membership from the table
schema, so this check does not advance either the index or the current record.

### Test-first record

The focused tests first failed because `IndexSelectScan.HasField` was undefined. They verify integer and varchar fields,
an absent field, and false results for nil adapter or missing table-scan state.

## Feature 17: Close the index-select scan

`IndexSelectScan.Close` releases the index first and then the retained data-table scan, matching the Java close order.
Both resources are attempted even if the index close fails; contextual errors are joined so callers can inspect either
cause. A nil adapter returns an error, while absent resource references are harmless.

### Test-first record

The focused tests first failed because `IndexSelectScan.Close` was undefined. They verify index-before-table ordering,
attempting both closes after a failure, preserving both wrapped causes, nil receiver handling, and partial/absent resource
states.

## Feature 18: Open an index from metadata

`metadata.IndexInfo.Open` returns the static `hash.HashIndex` through the `index.Index` contract, wired with the exact
transaction, index name, and index-record layout retained by the metadata object. It performs no transaction I/O until
the caller positions the index; `BeforeFirst` then selects and opens the appropriate bucket. This completes the deferred
opening step from Chapter 7 without introducing B-tree behavior.

### Test-first record

The focused test first failed because `IndexInfo.Open` was undefined. It verifies construction without bucket I/O, then
positions integer and varchar indexes, inserts an entry, and reads its RID back to validate bucket and layout wiring.

## Feature 19: Preserve the child schema in an index-select plan

`plan.NewIndexSelectPlan` retains the child plan, `metadata.IndexInfo`, and search key. Its `Schema` delegates to the
child and returns the same schema pointer because indexed selection changes row access, not the table's logical fields.
A nil plan or missing child reports no schema, matching the other plan adapters. Opening the scan and cost estimates
remain separate behaviors.

### Test-first record

The focused test first failed because `IndexSelectPlan` and `NewIndexSelectPlan` were undefined. Tests verify retained
dependencies/search key, unchanged child-schema identity, and nil results for a nil plan or child.

## Feature 20: Open an index-select scan from its plan

`IndexSelectPlan.Open` opens its child and requires the query-level `RecordScan` adapter around the table scan, then
opens the retained index metadata and constructs an `IndexSelectScan` positioned on the search key. Invalid plans,
unset keys, nil or incompatible child scans, child-opening failures, and index-positioning failures return contextual
errors. If an opened child or scan cannot be used, acquired scans are closed and cleanup failures remain joined with
the original cause.

### Test-first record

The focused test first failed because `IndexSelectPlan.Open` was undefined. Tests verify successful adapter creation and
cleanup, missing state/key checks before child I/O, wrapped child errors including partial scan cleanup, nil and
incompatible scan rejection, and joined bucket-position/table-cleanup errors.

## Feature 21: Estimate index-select plan block accesses

`IndexSelectPlan.BlocksAccessed` adds the estimated static-hash index traversal cost to the estimated matching table
rows. If the plan or index metadata is unavailable, or the metadata cannot estimate index blocks, it returns zero
because the `Plan` estimate interface has no error result. The final addition follows the repository's signed 32-bit
estimate arithmetic.

### Test-first record

The focused test first failed because `IndexSelectPlan.BlocksAccessed` was undefined. Tests cover the index-plus-row
formula, nil plan and metadata, metadata block-size errors, and signed 32-bit overflow behavior.

## Feature 22: Estimate index-select plan output rows

`IndexSelectPlan.RecordsOutput` delegates the matching-row estimate to its retained `metadata.IndexInfo`. It returns
zero when the plan or its index metadata is unavailable; it does not open or consult the child plan because the index
metadata owns this estimate.

### Test-first record

The focused test first failed because `IndexSelectPlan.RecordsOutput` was undefined. Tests verify metadata delegation
without a child plan and zero results for nil plan and missing metadata.

## Feature 23: Estimate index-select plan distinct values

`IndexSelectPlan.DistinctValues` forwards the requested field to its retained `metadata.IndexInfo`. The indexed field
has one value for a single-key selection, while other fields use the metadata statistics estimate. Nil plan and missing
index metadata return zero; the child plan is not consulted for this index-defined estimate.

### Test-first record

The focused test first failed because `IndexSelectPlan.DistinctValues` was undefined. Tests verify the single value for
the indexed field, metadata delegation for another field even when the child has different estimates, and zero results
for nil plan and absent metadata. A compile-time assertion confirms that `IndexSelectPlan` now implements `Plan`.

## Feature 24: Position an index join on its first left row

`indexquery.NewIndexJoinScan` retains the left scan, index, join-field name, and right record scan, then initializes
the left scan and positions the index with the first left row's typed join value. An empty left input leaves the index
unpositioned and the scan exhausted. The constructor and `BeforeFirst` report wrapped errors without closing these
caller-owned resources; advancing the index entries and reading joined fields remain separate behaviors.

### Test-first record

The focused tests first failed because `IndexJoinScan` and `NewIndexJoinScan` were undefined. They cover initial typed
key positioning, empty left input, repeat positioning, invalid dependencies, and wrapped left-reset, left-advance,
join-value, and index-positioning errors.

## Feature 25: Advance index-join scans through matching records

`IndexJoinScan.Next` returns each matching index RID after moving the right table scan to it. When the current key has
no more index entries, it advances the left scan, reads the next join value, repositions the index, and continues; left
exhaustion returns false and marks the scan exhausted. Index iteration, RID reads, right-side moves, left advancement,
join-value reads, and index repositioning preserve their wrapped causes on failure.

### Test-first record

The focused test first failed because `IndexJoinScan.Next` was undefined. Tests verify multiple matching RIDs across
different left keys, final exhaustion, empty input, invalid scan state, and errors from index iteration/RID retrieval,
right-side movement, left advancement/value retrieval, and index repositioning.

## Feature 26: Read integer fields from index join scans

`IndexJoinScan.GetInt` reads from the right table when its schema contains the requested field; otherwise it delegates to
the left scan. Failures are contextual and retain their wrapped causes.

### Test-first record

The focused tests first failed because `IndexJoinScan.GetInt` was undefined. Tests verify right-first and left-fallback
routing, returned values, nil receiver and dependency validation, and wrapped read errors from both selected scans.

## Feature 27: Read typed fields from index join scans

`IndexJoinScan.GetVal` returns the right table's schema-typed query constant when it declares the field and otherwise
delegates to the left scan. Errors identify the selected side and retain their causes.

### Test-first record

The focused test run first failed because `IndexJoinScan.GetVal` was undefined. Tests verify right-side integer and
string constants, left-side fallback, invalid receiver/dependencies, and wrapped errors from both selected scans.

## Feature 28: Read string fields from index join scans

`IndexJoinScan.GetString` reads from the right table when it declares the requested field; otherwise it delegates to the
left scan. The selected string is returned unchanged, and errors retain their wrapped causes.

### Test-first record

The focused tests first failed because `IndexJoinScan.GetString` was undefined. Tests verify right-table and left-scan
routing, returned strings, invalid receiver/dependencies, and wrapped errors from either selected scan.

## Feature 29: Report index-join field membership

`IndexJoinScan.HasField` checks the right table schema first and otherwise delegates to the left scan. It returns false
for a nil receiver or unavailable scan dependencies; a field declared on the right short-circuits the left check.

### Test-first record

The focused tests first failed because `IndexJoinScan.HasField` was undefined. Tests verify right-side precedence,
left-side fallback, absent fields, invalid state, and short-circuit behavior.

## Feature 30: Close index-join scan resources

`IndexJoinScan.Close` releases the left scan, index, and right table scan in Java order. It attempts every available
resource and joins contextual errors so each underlying cause remains detectable; a nil receiver returns an error.

### Test-first record

The focused tests first failed because `IndexJoinScan.Close` was undefined. Tests verify close order, all three failures
being returned, partial resources, an absent embedded right table scan, and nil receiver handling.

## Feature 31: Expose the schema of index join plans

Java `IndexJoinPlan` builds its schema by appending the left plan schema and then the right plan schema. Go's
`NewIndexJoinPlan` retains both plans, the right index metadata, and the join field while copying both available child
schemas into a plan-owned schema; `Schema` exposes that combined value.

### Test-first record

The focused tests first failed because `IndexJoinPlan` was undefined. Tests verify retained inputs, left-to-right field
order and definitions, that the combined schema is independent of later child schema changes, missing child schemas,
and nil receiver behavior.

## Feature 32: Open index join plans

Java `IndexJoinPlan.open` opens the left input, casts the right input to a table scan, opens the index, and constructs an
`IndexJoinScan`. Go's `IndexJoinPlan.Open` requires the right child to provide an initialized `query.RecordScan`, rejects
missing or typed-nil scans safely, and returns contextual errors while preserving their causes. If opening or scan
initialization fails, it attempts to close every already-opened scan and index and joins cleanup errors with the primary
failure.

### Test-first record

The focused tests first failed because `IndexJoinPlan.Open` was undefined. Tests verify the initialized scan type and
resource release, nil plan dependencies and child scans, typed-nil scans, incompatible right scan types, partial child
open failures, left positioning/advancement/join-value errors, index positioning errors, and cleanup failures.

## Feature 33: Estimate index join block accesses

Java `IndexJoinPlan.blocksAccessed` estimates `B(left) + R(left) * B(index) + R(join)`. Go's
`IndexJoinPlan.BlocksAccessed` uses the retained left plan and index metadata, reproduces Java signed 32-bit overflow,
and returns zero when required state is absent or index metadata cannot compute its block estimate, matching the Go
error-handling choice made for `IndexSelectPlan`.

### Test-first record

The focused tests first failed because `IndexJoinPlan.BlocksAccessed` was undefined. Tests verify the formula, nil
receiver/children/metadata, a failed metadata estimate, and signed 32-bit overflow.

## Feature 34: Estimate index join output records

Java `IndexJoinPlan.recordsOutput` estimates `R(left) * R(index)`. Go's `IndexJoinPlan.RecordsOutput` uses signed
32-bit multiplication to preserve Java overflow and returns zero when the receiver, left plan, or index metadata is
missing. The right plan is not required because it does not participate in this estimate.

### Test-first record

The focused tests first failed because `IndexJoinPlan.RecordsOutput` was undefined. Tests verify the product, nil
receiver/left/index metadata, independence from the right plan, and signed 32-bit overflow.

## Feature 35: Estimate distinct values for index join plans

Java `IndexJoinPlan.distinctValues` uses the left estimate when the left schema declares `fieldName`, and otherwise
delegates to the right plan. Go's `IndexJoinPlan.DistinctValues` preserves that left-side precedence and fallback,
including delegation for a field not declared by either schema. It returns zero for a nil receiver, absent left plan,
or an unavailable fallback plan without panicking. `IndexJoinPlan` now satisfies the complete `Plan` interface.

### Test-first record

The focused tests first failed because `IndexJoinPlan.DistinctValues` was undefined. Tests verify left/right routing,
duplicate-name precedence, unknown-field fallback, missing schemas/plans, nil receiver, and interface conformance.

## Feature 36: Pin a B-tree page when opened

Java `BTPage` retains the transaction, block identifier, and layout, then pins the block in its constructor. Go's
`btree.NewPage` follows that ownership boundary, validates the block and layout before I/O, and retains the dependencies
only after pin succeeds. `PageTransaction` extends the record-page operations with the matching unpin capability needed
by the page lifecycle; closing the page remains a separate behavior. The [B-tree page pin diagram](../diagrams/ch12-btree-page-pin.svg)
shows the transaction pin and the retained block/layout; its editable source is
[`ch12-btree-page-pin.drawio`](../diagrams/ch12-btree-page-pin.drawio).

### Test-first record

The focused tests first failed because `NewPage` and `PageTransaction` were undefined. Tests verify a single pin of the
requested block, retained state, nil and typed-nil transaction rejection, missing layout/schema, invalid block
identifiers, and a pin failure whose original cause remains inspectable. The constructor performs no page reads or
writes.

The [B-tree page lifecycle diagram](../diagrams/ch12-btree-page-pin.svg) also shows the inverse `Close`/`Unpin`
operation and the retry-safe state transition; the editable source is
[`ch12-btree-page-pin.drawio`](../diagrams/ch12-btree-page-pin.drawio).

## Feature 37: Position before a B-tree search key

Java `BTPage.findSlotBefore` scans sorted `dataval` entries and returns the slot immediately before the first value
that is greater than or equal to the requested key. Go's `Page.FindSlotBefore` reads the page record count at byte
offset 4, calculates each field position from the eight-byte page header and record layout, and supports integer and
varchar keys; varchar ordering follows Java's UTF-16 code units. Duplicate keys return the position before the first duplicate; empty pages and keys preceding the first
entry return `-1`. Invalid types, malformed offsets, negative record counts, and storage read failures are returned as
contextual errors without changing the pinned page. The [B-tree slot search diagram](../diagrams/ch12-btree-slot-search.svg)
shows the lower-bound position and duplicate behavior; its editable source is
[`ch12-btree-slot-search.drawio`](../diagrams/ch12-btree-slot-search.drawio).

### Test-first record

The focused tests first failed because `Page.FindSlotBefore` was undefined. Tests cover integer and varchar ordering,
empty pages, keys below/at/between/above values, duplicates, invalid and typed-nil state, absent or malformed
`dataval` metadata, unset or mismatched keys, negative counts, offset overflow, and wrapped header/value read failures.

## Feature 38: Close a B-tree page

Java `BTPage.close` unpins the currently retained block and then clears its current-block reference. Go's `Page.Close`
performs the same release, uses an empty block identifier as its closed state, and is safe to call repeatedly. When
`Unpin` fails, the page retains the block so callers can retry; successful close preserves the transaction and layout
configuration but prevents further slot searches. Closing a page without a block is a no-op, while a nil page or an
active page without its transaction returns a contextual error. The B-tree page lifecycle diagram above shows both
pin and unpin directions.

### Test-first record

The focused tests first failed because `Page.Close` was undefined. Tests verify exactly-once unpin, idempotent repeated
close, closed-page search rejection, absent-block no-op, nil and typed-nil state, wrapped unpin failure, preserved state,
and successful retry.

## Feature 39: Check B-tree page capacity

Java `BTPage.isFull` checks whether the position of the next record slot is at or beyond the transaction block size.
Go's `Page.IsFull` uses the same eight-byte page header and the retained layout slot size, and reports whether
`8 + (recordCount + 1) * slotSize >= blockSize`. It returns errors for invalid or closed page state, malformed counts
or sizes, position overflow, and record-count/block-size read failures while preserving the underlying error causes.
The [B-tree page capacity diagram](../diagrams/ch12-btree-page-capacity.svg) shows the boundary comparison and its
editable source is [`ch12-btree-page-capacity.drawio`](../diagrams/ch12-btree-page-capacity.drawio).

### Test-first record

The focused tests first failed because `Page.IsFull` was undefined. Tests cover a fitting next slot, exact capacity,
an over-capacity position, invalid and closed state, negative counts, invalid block sizes, position overflow, and
wrapped count/block-size read failures.

## Feature 40: Read a B-tree page data value

Java `BTPage.getDataVal` reads the `dataval` field from the requested slot and returns its schema type. Go's
`Page.GetDataVal` computes the field position from the eight-byte page header, slot index, layout slot size, and
`dataval` offset, then returns an integer or varchar `query.Constant`. It checks the slot calculation for overflow,
rejects invalid or closed page/layout state and unsupported field types, and wraps storage read errors. The accessor
does not change page pin state. The [B-tree data value diagram](../diagrams/ch12-btree-page-data-value.svg) shows the
field-offset calculation and its editable source is [`ch12-btree-page-data-value.drawio`](../diagrams/ch12-btree-page-data-value.drawio).

### Test-first record

The focused tests first failed because `Page.GetDataVal` was undefined. Tests cover typed integer and varchar reads at
layout-derived positions, unchanged pin state, invalid and typed-nil state, missing schema/field/offset, unsupported
types, negative and overflowing slots, malformed offsets, and wrapped read failures.

## Feature 41: Read a B-tree page flag

Java `BTPage.getFlag` reads the four-byte flag stored at byte offset zero of the current page block. Go's
`Page.GetFlag` reads that same signed 32-bit value without requiring record-layout metadata or changing the page's pin
state. It returns contextual errors for nil or closed page state, a nil transaction, and a failed storage read while
preserving the underlying cause.

### Test-first record

The focused tests first failed because `Page.GetFlag` was undefined. Tests verify positive, zero, and negative flag
values at offset zero, the absence of layout and pin-state requirements, invalid and typed-nil state, and wrapped read
failures.

## Feature 42: Write a B-tree page flag

Java `BTPage.setFlag` stores the supplied flag at byte offset zero and asks the transaction to log the change. Go's
`Page.SetFlag` writes the same signed 32-bit value with logging enabled, without requiring layout metadata, reading the
old value, or changing the page pin. Nil/closed page state, a nil transaction, and storage write failures return
contextual errors while preserving the original cause.

### Test-first record

The focused tests first failed because `Page.SetFlag` was undefined. Tests verify positive, zero, and negative flag
values, the exact block/header offset and logging flag, no data read/layout/pin changes, invalid and typed-nil state,
and wrapped write failures.

## Feature 43: Read a B-tree page record count

Java `BTPage.getNumRecs` reads the signed 32-bit count at byte offset four in the current page block. Go's
`Page.GetNumRecs` returns that stored value as-is, leaving malformed-count validation to the operation that consumes
it, and does not require layout metadata or change the page pin. Nil/closed page state, a nil transaction, and a failed
header read return contextual errors while preserving the storage error cause.

### Test-first record

The focused tests first failed because `Page.GetNumRecs` was undefined. Tests verify positive, zero, and negative
counts at header offset four, no layout or pin-state changes, nil and typed-nil state, and wrapped read failures.

## Feature 44: Write a B-tree page record count

Java's private `BTPage.setNumRecs` writes the record-count header at byte offset four and requests transaction
logging. Go's package-private `Page.setNumRecs` preserves that signed value and write behavior, without requiring
layout metadata or changing the pin state. Nil/closed page state, a nil transaction, and storage write failures return
contextual errors that preserve the original cause.

### Test-first record

The focused tests first failed because `Page.setNumRecs` was undefined. Tests verify signed count writes at offset four
with logging enabled, no reads/layout/pin changes, nil and typed-nil state, and wrapped write failures.

## Feature 45: Read a B-tree directory child number

Java `BTPage.getChildNum` reads the integer `block` field from a directory page slot. Go's `Page.GetChildNum` derives
the field position from the eight-byte page header, slot size, and `block` offset, then returns the stored signed
block number. It validates the page/layout/schema and integer field metadata, checks slot arithmetic, wraps storage
read errors, and leaves the page pin unchanged.

### Test-first record

The focused tests first failed because `Page.GetChildNum` was undefined. Tests cover a signed block number at the
layout-derived offset, invalid and typed-nil page/layout/field state, negative and overflowing slots, and wrapped read
failures without changing the pin lifecycle.

## Feature 46: Read a B-tree leaf record identifier

Java `BTPage.getDataRid` reads the integer `block` and `id` fields from one leaf slot and constructs a record RID.
Go's `Page.GetDataRID` reads those fields at layout-derived positions after the eight-byte page header, preserves the
stored values, and returns no partial identifier if either read fails. It validates both field definitions and slot
metadata, wraps storage errors, and leaves the page pin unchanged.

### Test-first record

The focused tests first failed because `Page.GetDataRID` was undefined. Tests verify the block-then-id read order and
positions, invalid and typed-nil state, missing or noninteger field metadata, negative/overflowing slots, and wrapped
errors from both reads without returning a partial RID.

## Feature 47: Write a B-tree page data value

Java's `BTPage.setVal` writes the typed `dataval` field at the calculated position for one slot, with transaction
logging enabled. Go's `Page.SetDataVal` accepts the matching integer or varchar `query.Constant`, derives the field
position from the eight-byte header and layout, and checks both slot arithmetic and the transaction's signed 32-bit
write offset. It rejects unset or mismatched constants and invalid page/layout state, wraps storage failures, and
does not change the page pin.

### Test-first record

The focused tests first failed because `Page.SetDataVal` was undefined. Tests cover logged integer and varchar writes
at their layout-derived positions, invalid and typed-nil state, missing/unsupported/malformed field metadata, unset
and mismatched constants, negative/overflowing slots, int32 offset overflow, and wrapped errors from both write types.

## Feature 48: Write a B-tree directory child number

Java's `BTPage.setInt` writes a directory child number to the integer `block` field for one slot with logging enabled.
Go's `Page.SetChildNum` derives the slot position from the eight-byte page header and layout, preserves the signed
32-bit value, checks metadata and the transaction's signed 32-bit write-offset range, and wraps storage failures. It
does not read page data or change the page pin.

### Test-first record

The focused tests first failed because `Page.SetChildNum` was undefined. Tests verify negative, zero, and positive
values at the layout-derived offset with logging enabled, no reads/pin changes, nil and typed-nil state, missing or
noninteger fields, malformed offsets, negative/overflowing slots, int32 offset overflow, and wrapped write failures.

## Feature 49: Write a B-tree leaf record identifier

Java `BTPage.setDataRid` stores a leaf entry's data block number and record slot in the integer `block` and `id`
fields. Go's `Page.SetDataRID` calculates both positions from the eight-byte page header and the retained layout,
validates both locations and signed 32-bit values before writing, then performs the two logged integer writes in
block-then-id order. Invalid page/layout/slot state and storage failures return contextual errors that preserve their
causes; the operation does not read page data or change the pin lifecycle. The existing index-contract and B-tree
field-offset diagrams already show the RID and slot boundaries, so another illustration would duplicate them.

### Test-first record

The focused tests first failed because `Page.SetDataRID` was undefined. Tests verify both layout-derived logged writes,
signed values, no reads or pin changes, invalid and typed-nil state, absent/noninteger/malformed field metadata,
negative/overflowing slots and RID values, and wrapped errors from either field write.

## Feature 50: Insert a B-tree directory entry

Java `BTPage.insertDir` opens the requested slot, copies each later record one position to the right in reverse order,
increments the page record count, then writes the new `dataval` and child `block` fields. Go's `Page.InsertDir` uses
the retained schema and layout to copy complete integer/varchar records with transaction logging, then calls the
existing typed field writers for the new entry. It validates the page, metadata, value type, slot, count growth, field
bounds, and signed transaction offsets before the first mutation. Each storage write is logged individually; if an
operation fails after an earlier write, the caller's transaction rollback restores the page. The caller remains
responsible for checking capacity and choosing when to split. The [directory insertion diagram](../diagrams/ch12-btree-insert-dir.svg)
shows the right-to-left shift and the final key/child write; its editable source is
[`ch12-btree-insert-dir.drawio`](../diagrams/ch12-btree-insert-dir.drawio).

### Test-first record

The focused tests first failed because `Page.InsertDir` was undefined. Tests verify integer and varchar keys, complete
record shifting in reverse order, append at the end, logged field writes, unchanged pin state, invalid/typed-nil page
and layout metadata, mismatched value types, negative counts/slots, count and offset overflow, and wrapped count/read/
copy/count-write/new-key/new-child failures.

## Feature 51: Insert a B-tree leaf entry

Java `BTPage.insertLeaf` opens the selected slot, shifts complete later records to the right, increments the count,
then writes `dataval`, `block`, and `id`. Go's `Page.InsertLeaf` shares the checked slot-opening operation with
`Page.InsertDir`, validates both integer RID fields and both signed 32-bit RID values before shifting, and delegates the
key and RID writes to the existing typed page methods in Java order. Each write requests transaction logging; a caller
can roll back earlier writes if a later storage operation fails. Capacity checks and leaf splitting remain separate
operations. The [leaf insertion diagram](../diagrams/ch12-btree-insert-leaf.svg) shows the record shift and distinct
key/RID fields; its editable source is [`ch12-btree-insert-leaf.drawio`](../diagrams/ch12-btree-insert-leaf.drawio).

### Test-first record

The focused tests first failed because `Page.InsertLeaf` was undefined. Tests verify a middle insertion across a
varchar key and both RID fields, append to an empty page, exact logged write order, invalid page/layout/RID metadata,
RID range validation before mutation, unchanged pin state, and wrapped count, shift, count-write, key-write, and either
RID-write failures.

## Feature 52: Delete a B-tree page entry

Java `BTPage.delete` copies each later record into the preceding slot in ascending order, then decrements the record
count. Go's `Page.Delete` validates the page, schema, count, slot, and all field offsets before writing; it shifts each
schema field with transaction logging and updates the count only after all copies succeed. Deleting the last entry skips
the copy loop. Storage failures return wrapped errors, and the caller's transaction rollback restores any earlier
logged writes. The [page deletion diagram](../diagrams/ch12-btree-page-delete.svg) shows the left shift and count
change; its editable source is [`ch12-btree-page-delete.drawio`](../diagrams/ch12-btree-page-delete.drawio).

### Test-first record

The focused tests first failed because `Page.Delete` was undefined. Tests cover removal from the middle of a page with
varchar and integer fields, ascending copy order, last-slot deletion without copy reads, logged writes, invalid and
typed-nil page state, negative counts and out-of-range slots, malformed layouts, int and int32 offset overflow, and
wrapped count-read, field-copy, and count-write errors.

## Feature 53: Format a B-tree page

Java `BTPage.format` writes the page flag and empty record count, then initializes each complete slot that fits after
the eight-byte header. Go's `Page.Format` validates the target block and retained schema/layout, reads the transaction
block size, and writes integer fields as zero and varchar fields as empty strings in schema order. Every initialization
write disables transaction logging, matching Java's caller-managed page setup; the method does not change pin state.
Offsets are checked against the transaction's signed 32-bit write API before the first write, and storage errors retain
their causes. Since partial format writes are unlogged, the caller must not publish the target block if formatting
fails. The [page format diagram](../diagrams/ch12-btree-page-format.svg) shows the header, full slots, and unused tail;
its editable source is [`ch12-btree-page-format.drawio`](../diagrams/ch12-btree-page-format.drawio).

### Test-first record

The focused tests first failed because `Page.Format` was undefined. Tests verify signed flags, zero count, integer and
varchar defaults, schema write order, complete-slot boundaries, disabled logging, caller-owned pin state, invalid page,
target and layout metadata, block-size and offset errors, and wrapped failures for header and default-value writes.

## Feature 54: Append and format a new B-tree page

Java `BTPage.appendNew` appends a block to the current page's file and formats it with the supplied flag. Go's
`Page.AppendNew` checks the current page and schema before appending, requires the transaction to support block append,
and then initializes the returned block through `Page.Format`. Formatting remains unlogged and does not alter pin
ownership. Append and format failures preserve their causes; when formatting fails after allocation, the method returns
no block ID so callers cannot publish a partially initialized page and can roll back the transaction as needed. The
[append-and-format diagram](../diagrams/ch12-btree-page-append-new.svg) shows this sequence and its failure boundary;
its editable source is [`ch12-btree-page-append-new.drawio`](../diagrams/ch12-btree-page-append-new.drawio).

### Test-first record

The focused tests first failed because `Page.AppendNew` was undefined. Tests verify same-file append and format order,
the returned block on success, wrapped append and format errors, a zero block ID after partial-format failure,
pre-append dependency validation, invalid appended block IDs, no pin changes, and no format writes after append fails.

## Feature 55: Split a B-tree page

Java `BTPage.split` appends and formats a destination page, transfers records beginning at `splitpos`, sets the new
page flag, and returns its block. Go's `Page.Split` validates the record count, split boundary, destination capacity,
layout offsets, and signed write positions before allocating the destination. It copies each schema field into the
destination with logged writes, advances that page's count after a complete record, then deletes the source record at
the split slot so the remaining suffix moves left in order. The source page's flag and pin remain unchanged. The
destination is pinned for the operation and closed on every path; storage and close failures preserve their causes,
and logged mutations require transaction rollback if an error interrupts the transfer. A failure returns no block ID.
The [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) shows the prefix/suffix boundary, the new page,
and the count changes; its editable source is [`ch12-btree-page-split.drawio`](../diagrams/ch12-btree-page-split.drawio).

### Test-first record

The focused tests first failed because `Page.Split` was undefined. Tests verify integer and varchar suffixes, empty and
full split boundaries, record order and logged copies, source/destination flags and counts, capacity/offset validation
before append, append/pin/read/write/count/flag/unpin failures, cause preservation, and destination cleanup.

## Feature 56: Select a child block from a B-tree directory page

Java `BTreeDir.findChildBlock` positions before a search key, then advances to the first exact match when one exists;
otherwise it follows the greatest directory key below the search key. Go's `Page.FindChildBlock` applies this rule
to one open directory page and returns a `storage.BlockID` in that page's file. It rejects empty pages and keys below
the first directory key, which the B-tree directory invariant reserves as its minimum-key entry. Page schema, typed
key, integer child field, count, candidate slot, and nonnegative child number are checked. Read failures preserve their
causes, and selection does not change page contents or pin ownership. It selects one level only; recursive descent,
leaf traversal, and index planning remain separate behaviors. The [directory child selection diagram](../diagrams/ch12-btree-find-child-block.svg)
shows exact-match and predecessor routing; its editable source is
[`ch12-btree-find-child-block.drawio`](../diagrams/ch12-btree-find-child-block.drawio).

### Test-first record

The focused tests first failed because `Page.FindChildBlock` was undefined. Tests cover integer/varchar routing,
the first exact match among duplicates, a greatest-predecessor match, keys above the final directory key, the minimum
key boundary, empty and malformed pages, invalid typed keys, negative children, wrapped count/key/child errors, and
unchanged page contents and pin ownership.

## Feature 57: Open one child B-tree directory page

Java `BTreeDir.search` opens the selected child page while descending through directory levels. Go's
`Page.OpenChildDirectory` performs one level transition: it requires a positive page flag, selects the child through
`Page.FindChildBlock`, and pins a new `Page` with the same transaction and directory layout. The parent page remains
open and unchanged; the caller owns and closes the returned child page. A level-zero page points to leaf blocks, so this
operation rejects it rather than interpreting a leaf with the directory layout. Repeated descent, final leaf opening,
and index-search state remain separate behaviors. The [child directory-page diagram](../diagrams/ch12-btree-open-child-directory.svg)
shows the one-level pin transfer and caller ownership; its editable source is
[`ch12-btree-open-child-directory.drawio`](../diagrams/ch12-btree-open-child-directory.drawio).

### Test-first record

The focused tests first failed because `Page.OpenChildDirectory` was undefined. Tests verify same-file child selection,
retention of the parent pin, the returned page's transaction/layout, caller-controlled child close, rejection of level
zero and negative flags, and wrapped flag, selection, and child-pin failures without changing parent state.

## Feature 58: Find a leaf block by descending B-tree directories

Java `BTreeDir.search` follows the selected child until it reaches a directory whose flag is zero, then returns the
corresponding leaf block. Go's `Page.FindLeafBlock` applies `Page.FindChildBlock` at each level and mutates its receiver
to retain the last open directory page. It closes each parent before pinning the next directory page, so traversal
works with a one-frame buffer. At level zero it returns the selected leaf `storage.BlockID` without opening the leaf;
the caller still owns and must close the retained directory page. A negative level is invalid. Flag, child-selection,
unpin, and next-page pin errors retain their causes. If an unpin fails, the current page remains open for a close retry;
if a child pin fails after the parent closes, the receiver remains closed. Opening the leaf page and maintaining B-tree
index search state remain separate behaviors. The [directory descent diagram](../diagrams/ch12-btree-find-leaf-block.svg)
shows the one-pin-at-a-time transition and returned leaf block; its editable source is
[`ch12-btree-find-leaf-block.drawio`](../diagrams/ch12-btree-find-leaf-block.drawio).

### Test-first record

The focused tests first failed because `Page.FindLeafBlock` was undefined. Tests cover multi-level routing, a level-zero
directory, exact pin/unpin order, a maximum of one active directory pin, caller cleanup of the final page, invalid and
closed state, a negative level, wrapped flag and child-selection failures, retryable unpin failure, and a failed next
pin that leaves the receiver closed.

## Feature 59: Open the selected B-tree leaf page

Java `BTreeIndex.beforeFirst` closes the directory after `BTreeDir.search` and then constructs `BTreeLeaf`, which pins
the selected leaf block using a different record layout. Go's `Page.OpenLeafPage` expresses only that pin transition:
it requires an open level-zero directory page, validates the target block and leaf layout before releasing the directory,
closes the directory, then opens the selected leaf with the same transaction and supplied leaf layout. The returned page
pin belongs to the caller, while the directory receiver is closed. Closing before opening supports a one-frame buffer.
If directory unpin fails, no leaf pin is attempted and the directory remains open for retry; if leaf pinning fails after
the directory closes, the receiver remains closed. Search-key positioning remains the separate `Page.FindSlotBefore`
operation, and leaf iteration/overflow state remain separate behaviors. The [directory-to-leaf pin diagram](../diagrams/ch12-btree-open-leaf-page.svg)
shows the layout and ownership transition; its editable source is
[`ch12-btree-open-leaf-page.drawio`](../diagrams/ch12-btree-open-leaf-page.drawio).

### Test-first record

The focused tests first failed because `Page.OpenLeafPage` was undefined. Tests cover same-transaction leaf pinning
with a distinct layout, directory-close-before-leaf-pin order, one active pin maximum, caller leaf cleanup, invalid
directory and leaf inputs before unpinning, rejection of positive and negative directory levels, wrapped flag and
directory-unpin failures, retry after unpin failure, and a wrapped leaf-pin failure that leaves the receiver closed.

## Feature 60: Advance to the next matching B-tree leaf slot

Java `BTreeLeaf.next` advances the slot, reads its key, and compares it with the active search key before considering
an overflow page. Go's `Page.NextLeafSlot` examines exactly `currentSlot+1` on the open page and returns the candidate
slot together with whether its `dataval` equals the supplied key. A mismatch and page exhaustion both return `false`,
leaving the caller responsible for deciding whether to follow the leaf's overflow flag. The operation validates page,
transaction, layout, key-field type and position, record count, and slot arithmetic. Count and key read failures keep
their wrapped causes. It does not change the page contents or pin ownership. Overflow traversal, RID retrieval, and
index cursor state remain separate behaviors; no diagram is needed for this single-slot comparison.

### Test-first record

The focused tests first failed because `Page.NextLeafSlot` was undefined. Tests cover integer duplicate matches,
integer mismatches and exhaustion, varchar equality, empty pages, malformed field layouts, invalid page/transaction
state, negative and overflowing input slots, negative counts, wrapped count/key reads, and unchanged pin ownership.

## Feature 61: Select a B-tree leaf overflow block

Java `BTreeLeaf.tryOverflow` follows a leaf's flag only when the search key equals the first key in that leaf and the
flag is nonnegative. Go's `Page.FindOverflowBlock` makes that decision without changing page ownership: it reads the
first `dataval`, then the flag, and returns a block in the same index file for a matching key, including block zero. A
different first key or a negative flag reports no target; read errors preserve their causes. The operation checks the
page, transaction, key field, layout, and typed search key. Opening the returned block, advancing within it, and
maintaining B-tree index cursor state remain separate behaviors; the equality-and-flag decision does not need a diagram.

### Test-first record

The focused tests first failed because `Page.FindOverflowBlock` was undefined. Tests cover matching integer/varchar keys,
overflow block zero, a mismatching first key, a negative flag, invalid/typed-nil state, mismatched or unset search keys,
malformed layouts, wrapped first-key and flag reads, flag-read ordering after a mismatch, and unchanged pin ownership.

## Feature 62: Open the selected B-tree overflow page

After Java `BTreeLeaf.tryOverflow` selects an overflow block, it closes the current contents page and pins the next page
with the same transaction and leaf layout. Go's `Page.OpenOverflowPage` performs that resource transition for a
previously selected same-file block. It validates the current page and target before releasing anything, then closes
before pinning so a one-frame buffer suffices. The returned page belongs to the caller. An unpin failure leaves the
current page open for retry; a target pin failure after close leaves the receiver closed with its dependencies retained.
Choosing whether overflow applies, resetting the slot, RID retrieval, and cursor state remain separate behaviors; no
diagram is needed for this one-page ownership transition.

### Test-first record

The focused tests first failed because `Page.OpenOverflowPage` was undefined. Tests cover close-before-pin order,
one active pin maximum, target and dependency retention, caller cleanup, pre-close rejection of invalid state and
cross-file/negative targets, retry after unpin failure, and a failed target pin that leaves the receiver closed.

## Feature 63: Initialize a B-tree leaf search cursor

Java `BTreeLeaf` opens a leaf page and positions its current slot immediately before the first entry whose key is not
less than the search key. Go's `NewLeaf` retains the pinned page, typed search key, and result of
`Page.FindSlotBefore`; an empty page starts before slot zero. If positioning fails after opening, it attempts to close
the acquired page and joins any close failure with the original cause. Page contents are unchanged. Advancing the
cursor, handling overflow blocks, retrieving RIDs, mutating entries, and exposing a close operation remain separate
behaviors; the initial slot state does not need a diagram.

### Test-first record

The focused tests first failed because `NewLeaf` and `Leaf` were undefined. Tests cover integer duplicate-key
positioning, varchar ordering, an empty page, validation before pinning, cleanup after key/layout/count read failures,
joined positioning and unpin errors, and wrapped pin failures.

## Feature 64: Advance a B-tree leaf search cursor

Java `BTreeLeaf.next` increments the slot, returns a match when the new key equals the retained search key, and otherwise
tries an overflow page. Go's `Leaf.Next` first advances through `Page.NextLeafSlot`; when that candidate does not match
or the page is exhausted, it consults `Page.FindOverflowBlock` and uses `Page.OpenOverflowPage` to transfer the pin
before continuing at overflow slot zero. It returns false when there is no eligible overflow page and wraps read or
pin-transition failures. This keeps lower-level slot selection, overflow eligibility, and page ownership as separate
reusable behaviors while exposing one cursor-advance result. RID retrieval, deletion, insertion, and cursor close remain
separate; no diagram is needed for the next-entry transition.

### Test-first record

The focused tests first failed because `Leaf.Next` was undefined. Tests cover consecutive duplicates, a different key,
same-file overflow traversal with one active page at a time, integer/varchar keys, no-overflow termination, wrapped
count/value/flag reads, unpin/pin transition failures, and invalid cursor state.

## Feature 65: Read the current B-tree leaf entry RID

Java `BTreeLeaf.getDataRid` delegates to its current slot in `BTPage`. Go's `Leaf.GetDataRID` delegates to
`Page.GetDataRID(currentSlot)`, so the returned table RID follows the slot selected by `Next`, including after that
method transfers the cursor to an overflow page. It does not advance the cursor or change page pin ownership, and wraps
page read errors while retaining their causes. Deletion, insertion, and cursor close remain separate behaviors; no
diagram is needed for this read-only delegation.

### Test-first record

The focused tests first failed because `Leaf.GetDataRID` was undefined. Tests verify the exact current slot, overflow
page RID, unchanged cursor/page pin state, and wrapped block/id read errors.

## Feature 66: Delete a matching B-tree leaf entry

Java `BTreeLeaf.delete` advances through records for the retained search key, compares each current data RID, and removes
the first match from the current `BTPage`. Go's `Leaf.Delete` uses its existing `Next` and `GetDataRID` behaviors, then
delegates the removal to `Page.Delete`; the scan naturally continues through overflow pages and stops at the first match.
No matching RID is a successful no-op. Cursor, RID-read, and page-delete errors retain their causes, and a failed page
deletion leaves the cursor on the entry for retry. Leaf insertion and cursor close remain separate behaviors; no diagram
is needed for this scan-and-remove operation.

### Test-first record

The focused tests first failed because `Leaf.Delete` was undefined. Tests cover removal among duplicate keys, overflow
traversal, no-match behavior, wrapped advance/RID/delete errors, retained pin ownership, and cursor position on deletion
failure.

## Feature 67: Represent B-tree directory entries

Java `DirEntry` is a value pairing the first key in a child block with that block's number. Go's `DirEntry` retains a
`query.Constant` and the block number and exposes both through accessors. It performs no I/O or validation; consuming
page operations validate block identifiers at the point where they are used. Leaf insertion and split decisions remain
separate behaviors; no diagram is needed for this small value type.

### Test-first record

The focused test first failed because `NewDirEntry` was undefined. Tests verify integer and varchar keys and preserve
block number zero and negative values without side effects.

## Feature 68: Insert a B-tree leaf entry into an available slot

Java `BTreeLeaf.insert` advances the cursor and asks `BTPage` to insert the retained search key and data RID. Go's
`Leaf.Insert` checks the current page capacity, inserts at `currentSlot+1` through `Page.InsertLeaf`, and moves the
cursor only after that write succeeds. The in-page path returns no `DirEntry` and retains page ownership; a page that is
already full is rejected before mutation. Overflow relocation and page-full split outcomes remain separate behaviors.

### Test-first record

The focused tests first failed because `Leaf.Insert` was undefined. Tests verify integer/varchar insertion at the cursor
position, RID fields, nil directory result, capacity rejection, wrapped errors, invalid cursor state, and unchanged page
ownership on success and failure.

## Feature 69: Close a B-tree leaf cursor

Java `BTreeLeaf.close` releases the page pin retained by the cursor. Go's `Leaf.Close` delegates to `Page.Close`, clears
the cursor's page reference only after a successful unpin, and returns a contextual error that preserves an unpin
failure. A failed close retains the page so the caller can retry; repeated close after success is a no-op, while later
cursor operations report that no page is open. The existing [B-tree page lifecycle diagram](../diagrams/ch12-btree-page-pin.svg)
already shows the page pin/unpin boundary and is reused here instead of drawing that same transition again.

### Test-first record

The focused tests first failed because `Leaf.Close` was undefined. They verify one unpin on success, idempotent repeated
close, closed-cursor errors from the other public operations, preservation of the page after a wrapped unpin failure,
successful retry, and nil-receiver validation.

## Feature 70: Insert a new minimum key into a B-tree leaf

Java `BTreeLeaf.insert` moves a key smaller than the current first key into a new left leaf page. When the page is not an
overflow page, Go's `Leaf.Insert` splits the old contents at slot zero, keeps the old flag on the new right page, marks
the left page with flag -1, inserts the new key/RID at slot zero, and returns a `DirEntry` for the former first key and
the new right block. The cursor advances only after the insert succeeds. Key comparison preserves Java's UTF-16 order
for varchar values. If a storage failure occurs after logged writes begin, the caller must roll back the transaction;
the existing [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) already illustrates the suffix transfer
and is reused instead of adding a duplicate diagram.

### Test-first record

Focused tests first failed because `Leaf.Insert` returned no directory entry and did not split when given a new minimum
key. Tests cover integer and varchar split results, old/new page flags and records, cursor and pin ownership,
empty pages and non-minimum keys, validation/read/split/write failures, and Java-compatible UTF-16 comparison.

## Feature 71: Link a full duplicate-key B-tree leaf to an overflow page

After a successful ordinary insert makes a leaf full, Java `BTreeLeaf.insert` stores repeated values in an overflow
chain. Go checks the first and last stored keys; if they match, `Leaf.Insert` splits at slot one, copies the prior
overflow flag to the new page, and points the original page flag at the new block. The inserted entry remains at the
cursor-selected slot, and this path returns no directory entry. A full page containing mixed keys follows Feature 72's
separate separator-split behavior instead of entering this duplicate-only path. The signed 32-bit page
flag range is checked before narrowing the appended block number. Callers must roll back the transaction if a storage
error occurs after logged page mutations begin. The existing [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg)
shows this suffix transfer and is reused rather than duplicated.

### Test-first record

Focused tests first failed because ordinary `Leaf.Insert` left duplicate entries in the full source page, returned no
error for a full mixed-key page, and ignored overflow-block flag range errors. Tests cover integer and varchar duplicate
chains, preservation of the previous overflow flag and moved RIDs, cursor/page-pin ownership, mixed-key rejection,
wrapped post-insert capacity/count/key/flag/split/link errors, invalid post-insert counts, and signed 32-bit block
validation.

## Feature 72: Split a full mixed-key B-tree leaf

When a full leaf contains more than one key value, Java `BTreeLeaf.insert` chooses a midpoint but moves the boundary
to the edge of the midpoint key's duplicate group. If the midpoint key is the first key, Go scans right to the next
key group; otherwise it scans left to the first slot of the midpoint group. `Leaf.Insert` splits at that boundary with
destination flag -1 and returns a `DirEntry` for the first key and block in the new page. It keeps the source page flag
and cursor slot unchanged. Parent directory insertion and root growth remain separate behaviors. The appended block
number is checked before it can be stored in a signed 32-bit directory field, and callers must roll back logged page
mutations after a later error. The existing [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) already
shows the source/destination record transfer and is reused.

### Test-first record

The focused test-first run failed because full mixed-key pages returned the duplicate-only path's error rather than a
directory separator. Tests cover midpoint boundaries on either side of repeated key groups, integer and varchar
separators, preserved flags/cursor/pin ownership, wrapped middle/right/left key reads, split failures, and signed
32-bit directory block validation.

## Feature 73: Insert a B-tree directory entry by key

Java `BTreeDir.insertEntry` locates the insertion point and delegates the physical record shift to `BTPage.insertDir`.
Go's `Page.InsertDirectoryEntry` uses `FindSlotBefore` and `InsertDir` to keep directory keys ordered, placing a new
duplicate before the first equal key. It accepts only a valid two-field directory layout (`dataval`, integer `block`),
a nonnegative directory level, and a child block number representable as a signed 32-bit value. It checks physical slot
capacity before the first write; splitting a full directory remains a separate behavior. An error after logged page
writes requires the caller's transaction rollback. The existing [directory insertion diagram](../diagrams/ch12-btree-insert-dir.svg)
shows the right-to-left shift and final key/child write; its editable source is
[`ch12-btree-insert-dir.drawio`](../diagrams/ch12-btree-insert-dir.drawio).

### Test-first record

The focused tests first failed because `Page.InsertDirectoryEntry` was undefined. They verify integer ordering,
duplicate placement, varchar end-append, empty-page insertion, use of the last physical slot, rejection of a full page
before mutation, invalid directory layouts and levels, signed 32-bit child-block bounds, and wrapped page flag, count,
block-size, key-read, and write errors.

## Feature 74: Split a full B-tree directory page

Java `BTreeDir.split` keeps the lower directory records on the current page, moves the upper records into a newly
appended page, and returns that page's first key and block as a separator. Go's `Page.SplitDirectoryPage` requires an
exact directory layout, a nonnegative level, and a physically full page with enough records for two nonempty sides. It
splits at the midpoint, preserves the level on both pages, and returns the first moved `DirEntry` for insertion into the
parent. A nonfull page is rejected before allocation. Parent insertion, recursive split propagation, and root growth
remain separate operations. If logged transfer or cleanup fails, the returned entry is zero and the caller must roll
back the transaction. The existing [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) shows the
source/destination boundary and is reused; its editable source is
[`ch12-btree-page-split.drawio`](../diagrams/ch12-btree-page-split.drawio).

### Test-first record

The focused tests first failed because `Page.SplitDirectoryPage` was undefined. They verify integer and varchar
separators, even and odd record counts, preserved page levels and sorted records, returned destination block, pin
cleanup, nonfull and invalid-page rejection before allocation, signed 32-bit separator block validation, and wrapped
flag/count/capacity/key/child/append/pin/write/unpin errors. The new method is 100.0% covered.

## Feature 75: Insert a B-tree directory entry and split when full

Java `BTreeDir.insertEntry` inserts a key/child pair and splits the directory page when that insertion fills its last
physical slot. Go's `Page.InsertDirectoryEntryAndSplit` composes the existing ordered insertion and full-page split
operations. It returns no separator if physical capacity remains; otherwise it returns the new right page's first
entry. It does not recurse into a parent or create a new root. Errors after the logged insertion require caller rollback,
and no separator is returned on any error. The existing [directory insertion diagram](../diagrams/ch12-btree-insert-dir.svg)
and [B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) explain the two steps and are reused rather than
duplicated.

### Test-first record

The focused tests first failed because `Page.InsertDirectoryEntryAndSplit` was undefined. They verify ordinary ordered
insertion without splitting, the final-slot insertion followed by midpoint split, returned parent separator, integer and
varchar ordering, preserved page levels, refusal when the page is already full, and wrapped insertion/count/capacity/
split errors. The new method is 100.0% covered.

## Feature 76: Propagate directory splits through parent pages

Java `BTreeDir.insert` descends through the child selected by the new entry's key. It closes that child after recursion,
then inserts the child's separator into the current directory only when the child split. Go's
`Page.InsertDirectoryEntryRecursive` follows the same level-zero and internal-page behavior, returning a separator only
when the current page splits as well. It closes every opened child before changing the parent and joins recursive and
close failures so callers can retain every cause. Root growth and formatting a replacement root remain separate
operations. The existing [directory insertion diagram](../diagrams/ch12-btree-insert-dir.svg) and
[B-tree page split diagram](../diagrams/ch12-btree-page-split.svg) show the ordered parent insertion and split boundary;
their editable Draw.io sources are reused.

### Test-first record

The focused run first failed because `Page.InsertDirectoryEntryRecursive` was undefined. Tests cover direct level-zero
insertion and splitting, two-level recursive propagation, child and parent split propagation, parent immutability when a
child does not split, invalid pages and levels, child-open and recursive failures, child-close failures, joined
recursive/close causes, full-parent refusal, and parent-split append errors. The new method is 100.0% covered.

## Feature 77: Grow the fixed-block B-tree directory root

Java `BTreeDir.insert` keeps the root at directory block zero. When recursive insertion returns a split separator for
that root, Go's `Page.MakeNewRoot` appends a page at the current directory level, moves all old root entries into it,
and inserts two child entries into block zero: the moved page's first key and the returned right-page separator. The
root level is raised only after both entries are written. Before appending, the method validates that the separator
does not sort before the old root's final key, so the resulting two-child root remains ordered. It validates page,
layout, level, count, capacity, key type, and child block bounds before mutation. If a logged operation fails after
the move starts, the caller must roll back its transaction. Initial root formatting remains a separate behavior. The
[root growth diagram](../diagrams/ch12-btree-root-growth.svg) shows the fixed block-zero root and moved old entries;
its editable source is [`ch12-btree-root-growth.drawio`](../diagrams/ch12-btree-root-growth.drawio).

### Test-first record

The focused test first failed because `Page.MakeNewRoot` was undefined. An additional ordered-key case then failed
because a separator between existing root keys could be accepted and mutate the file. Tests cover integer and varchar
roots, raising the level from zero and an existing internal level, invalid root/layout/count/capacity/key/child state
before append, wrapped metadata and last-key reads, and append/pin/unpin/split/entry/level-write errors. The method is
100.0% covered.

## Feature 78: Initialize an empty B-tree directory root

Java `BTreeIndex` formats a newly appended directory block as level zero and inserts the first minimum-key entry that
points to leaf block zero. Go's `Page.InitializeDirectoryRoot` applies that initialization to an open block-zero page:
it validates the directory schema, key type, and capacity before writing, formats the page with level zero, and inserts
the caller-provided minimum key with child block zero. Java's integer index sentinel is the minimum signed integer and
its varchar sentinel is the empty string. Formatting retains the existing unlogged page-initialization behavior; the
directory entry uses the normal logged insertion path. The caller appends the block and must roll back that append if
initialization fails after a write. Empty leaf-page initialization and full B-tree index construction remain separate.
The existing [B-tree page format](../diagrams/ch12-btree-page-format.svg) and
[directory insertion](../diagrams/ch12-btree-insert-dir.svg) diagrams show the composed steps; their editable Draw.io
sources are reused.

### Test-first record

The focused compilation first failed because `Page.InitializeDirectoryRoot` was undefined. Tests verify integer and
varchar sentinel entries, level and count, fixed block zero, unlogged format writes followed by logged entry writes,
invalid pages/layouts/key types and insufficient block capacity before mutation, and wrapped block-size/format/entry
write errors. The new method is 100.0% covered.

## Feature 79: Initialize an empty B-tree leaf page

Java `BTreeIndex` appends the first leaf block when the leaf file is empty, then formats that page with flag -1. Go's
`Page.InitializeLeafPage` applies the leaf-specific layout checks to an already allocated, open page and delegates the
zero-record/default-slot writes to `Page.Format` with flag -1. Block zero is used for the initial leaf and later
nonnegative blocks may head overflow chains. It preserves the caller's page pin and does not append a block. The
caller appends first and must roll back that append if formatting fails. Directory-root initialization and full
index construction remain separate. The existing [B-tree page format diagram](../diagrams/ch12-btree-page-format.svg)
shows the page header and empty slots; its editable Draw.io source is reused.

### Test-first record

The focused compilation first failed because `Page.InitializeLeafPage` was undefined. Tests cover integer and varchar
leaf layouts, initial and overflow block numbers, flag -1, count zero, default slot values, unlogged writes, unchanged
page ownership, invalid page/layout fields, and wrapped block-size and formatting errors. The new method is 100.0%
covered.

## Feature 80: Ensure an empty B-tree leaf file has its initial page

Java `BTreeIndex` checks the leaf-file length, appends block zero when the file is empty, and formats that block as an
empty leaf page. Go's internal `ensureLeafFile` performs the same orchestration by checking the file length before
appending, validating the leaf layout, opening the new block with `NewPage`, and delegating its format to
`Page.InitializeLeafPage`. Existing leaf files remain unchanged. A newly appended block belongs to the transaction, so
the caller must roll it back when a later pin, format, or close operation fails; this helper always attempts to release
the page pin and joins initialization and cleanup failures. Directory-file initialization and the complete index
constructor remain separate. The existing [B-tree page format diagram](../diagrams/ch12-btree-page-format.svg) shows
the initialized block; its editable Draw.io source is reused.

### Test-first record

The focused test first failed because `ensureLeafFile` and its transaction boundary were undefined. Tests cover empty
integer and varchar files, existing files, append/pin/format/unpin behavior, malformed inputs and returned blocks, and
wrapped storage and cleanup errors. `leafPageFields` is shared with `Page.InitializeLeafPage` so both entry points
validate the same record shape. Every statement in the new helper and every executable function remain 100.0% covered.

## Feature 81: Ensure an empty B-tree directory file has its root

Java `BTreeIndex` appends directory block zero when the directory file is empty, formats it at level zero, and inserts
the minimum-key entry for leaf block zero. Go's internal `ensureDirectoryFile` performs that file-level behavior for a
caller-supplied directory layout: it leaves a nonempty file untouched, validates the `dataval` and integer `block`
fields before append, and chooses Java's minimum signed integer or empty-string sentinel from the key type. It opens the
new root page, delegates formatting and insertion to `Page.InitializeDirectoryRoot`, and releases the pin. The
transaction owns appended blocks, so callers must roll back when a later pin, format, entry-write, or close operation
fails; initialization and cleanup causes are both preserved. Building the directory schema and composing the full
B-tree index remain separate. The existing [B-tree page format](../diagrams/ch12-btree-page-format.svg) and
[directory insertion](../diagrams/ch12-btree-insert-dir.svg) diagrams show the root header and initial child entry; the
editable Draw.io sources are reused.

### Test-first record

The focused compilation first failed because `ensureDirectoryFile` and `directoryPageKeyType` were undefined. Tests
cover empty and existing files, integer and varchar sentinels, block-zero root level/count/child, unlogged format and
logged entry writes, invalid dependencies/layouts and append results, wrapped size/append/pin/format/entry/close errors,
and joined initialization/cleanup failures. `directoryPageKeyType` is shared with `Page.InitializeDirectoryRoot` so
both entry points apply the same directory-layout validation. Every statement in the new helper and every executable
function remain 100.0% covered.

## Feature 82: Derive the B-tree directory layout from the leaf schema

Java `BTreeIndex` declares the directory fields in order: integer `block`, followed by `dataval` with the leaf schema's
key type and conceptual length. Go's pure `directoryLayoutFromLeaf` helper validates the leaf page layout, then builds
that two-field record layout without reading or changing a file, transaction, or page. The caller's leaf schema remains
unchanged. Empty-file initialization and composition of the complete B-tree index remain separate behaviors. The
existing [directory insertion](../diagrams/ch12-btree-insert-dir.svg) diagram shows the directory key and child block;
its editable Draw.io source is reused.

### Test-first record

The focused compilation first failed because `directoryLayoutFromLeaf` was undefined. Tests cover integer and varchar
key types, varchar length preservation, directory field order and offsets, derived slot sizes, source-schema
immutability, and invalid leaf layouts (including nil metadata, missing fields, unsupported key types, and negative
varchar lengths). The helper performs no storage operations. Every statement in the helper and every executable
function remain 100.0% covered.

## Feature 83: Initialize B-tree index state and its files

Java `BTreeIndex` retains the transaction, index name, leaf layout, leaf file, directory layout, directory file, and
the block-zero directory root. Go's `NewBTreeIndex` derives the directory layout from the leaf schema, uses the Java
file suffixes `leaf` and `dir`, initializes the leaf file before the directory file, then returns that retained state
with root block zero. Existing files are preserved by the file initializers. Invalid inputs are rejected before file
access, and initialization errors return no partial index while wrapping their storage cause. Appended blocks belong
to the caller's transaction, so callers must roll back after a failed constructor. Searching, mutation, and cost
estimation remain separate behaviors. The existing [B-tree page format](../diagrams/ch12-btree-page-format.svg) and
[directory insertion](../diagrams/ch12-btree-insert-dir.svg) diagrams show the initialized files; their editable Draw.io
sources are reused.

### Test-first record

The focused compilation first failed because `NewBTreeIndex` was undefined. Tests cover integer and varchar indexes,
retained names/layouts/root, leaf-before-directory initialization, existing files, invalid inputs before file access,
wrapped initialization errors, and nil partial results. The constructor is 100.0% covered.

## Feature 84: Position a B-tree index before its search key

Java `BTreeIndex.beforeFirst` closes any old leaf cursor, descends from the block-zero directory root, and opens the
selected leaf immediately before the first matching key. Go's `BTreeIndex.BeforeFirst` validates the retained layouts,
root, and key type before storage access; it uses `Page.FindLeafBlock`, maps the directory child number to the leaf
filename, closes the level-zero directory page before pinning the leaf, and records the slot before the first key not
less than the search value. Matching-entry advancement and overflow-chain iteration remain separate. Errors preserve
their causes. If an unpin fails during cleanup, the still-owned page remains attached to the index so a later positioning
call can retry releasing it. The existing [directory insertion](../diagrams/ch12-btree-insert-dir.svg) and
[B-tree page format](../diagrams/ch12-btree-page-format.svg) diagrams show the selected child and page handoff; their
editable Draw.io sources are reused.

### Test-first record

The focused compilation first failed because `BTreeIndex.BeforeFirst` and its retained leaf state were undefined. Tests
cover integer and varchar positioning, leaf-before-root replacement order, invalid index/key state before pinning,
wrapped directory/pin/leaf-positioning errors, and retryable leaf or directory cleanup failures. The constructor and
positioning method keep the package at 100.0% statement coverage.

## Feature 85: Advance to the next matching B-tree entry

Java `BTreeIndex.next` delegates to its currently positioned `BTreeLeaf`. Go's `BTreeIndex.Next` exposes one advancement
through the retained `Leaf.Next` cursor and returns its match or exhaustion result. It rejects a nil index or missing
cursor before touching storage and wraps cursor failures without discarding their causes. Leaf-level duplicate overflow
traversal remains in the separate cursor behavior. The existing [directory insertion](../diagrams/ch12-btree-insert-dir.svg)
diagram shows the directory-to-leaf route used by the retained cursor; its editable Draw.io source is reused.

### Test-first record

The focused compilation first failed because `BTreeIndex.Next` was undefined. Tests cover one matching entry, clean
exhaustion, nil and unpositioned states without page access, closed cursors, and wrapped leaf-advance errors. The method
is 100.0% covered.

## Feature 86: Read the current B-tree index record identifier

Java `BTreeIndex.getDataRid` returns the RID stored in the current matching leaf entry. Go's
`BTreeIndex.GetDataRID` delegates to the retained `Leaf.GetDataRID` cursor and returns the same `record.RID`; it rejects
a nil index or a cursor that has not been positioned before reading a page. Wrapped leaf and page-read failures retain
their causes. Advancing matches remains the separate `Next` behavior. The existing
[index-contract diagram](../diagrams/ch12-index-contract.svg) shows the search key to RID boundary, and its editable
Draw.io source is reused.

### Test-first record

The focused compilation first failed because `BTreeIndex.GetDataRID` was undefined. Tests cover retrieval from the
current slot, unchanged cursor and page ownership, nil and unpositioned state without I/O, and wrapped block-read
failure. The method is 100.0% covered.

## Feature 87: Close the retained B-tree index pages

Java `BTreeIndex.close` releases the current leaf cursor. Go's `BTreeIndex.Close` also releases any directory page
retained after an earlier cleanup failure. It attempts the leaf first and then the pending directory page, even when
the first close fails, and joins contextual errors so callers can inspect every cause. A page reference is cleared only
after its close succeeds, allowing a later call to retry failed unpins; a repeated successful close performs no extra
I/O. The existing [index-contract diagram](../diagrams/ch12-index-contract.svg) includes resource cleanup and is
linked to its editable Draw.io source.

### Test-first record

The focused compilation first failed because `BTreeIndex.Close` was undefined. Tests cover leaf-before-directory close
order, idempotency, simultaneous unpin failures, retention and retry of both pages, and a nil receiver. The method is
100.0% covered.

## Feature 88: Insert an entry through the B-tree index

Java `BTreeIndex.insert` positions the leaf for the search value, inserts the RID, and propagates a returned separator
through the directory. Go's `BTreeIndex.Insert` delegates the ordered leaf mutation, closes that leaf before opening
the block-zero directory root, inserts split separators recursively, and grows the root in place when propagation
reaches it. A leaf insert that does not split performs no directory I/O. Errors preserve their causes, and the caller
must roll back the transaction if logged changes preceded a failure. The existing [root-growth diagram](../diagrams/ch12-btree-root-growth.svg)
shows the fixed block-zero root receiving two children; its editable Draw.io source is reused.

### Test-first record

The focused compilation first failed because `BTreeIndex.Insert` was undefined. Tests cover an in-page insert without
directory changes, leaf split propagation, root growth and block-zero retention, nil and invalid-key rejection before
pinning, joined leaf-write/close failures, root-repin failure, and a directory split error combined with retryable root
cleanup. The method is 100.0% covered.

## Feature 89: Delete an entry through the B-tree index

Java `BTreeIndex.delete` positions the leaf for a value and removes the matching RID. Go's `BTreeIndex.Delete` uses the
same cursor contract, deletes only the first value/RID pair, and does not modify directory pages. It closes the leaf
before returning and retains a failed unpin for retry through `Close`; mutation and cleanup causes are joined. If a
logged delete fails, transaction rollback remains the caller's responsibility. The existing [directory-to-leaf diagram](../diagrams/ch12-btree-find-leaf-block.svg)
and editable source `ch12-btree-find-leaf-block.drawio` explain how the search key selects the leaf. The focused test-first compilation failed because `BTreeIndex.Delete` was
undefined; tests cover a matching RID, a missing RID, nil and invalid-key inputs, positioning failure, and joined delete
and close failures with cleanup retry. The method is 100.0% covered.

## Intentional differences from Java

- Fallible operations return Go `error` values instead of relying on unchecked Java calls or exceptions.
- The contract uses the existing `query.Constant` and `record.RID` value types; concrete hash and B-tree indexes remain separate implementations.
- Static hash bucket selection normalizes Java hash remainders to a nonnegative range, avoiding Java's negative-remainder invalid-index case.
- Go insertion validates the index-record schema and signed 32-bit RID storage before changing scan state, and wraps storage failures as explicit errors.
- Go deletion reports storage failures as errors while preserving the Java behavior of removing the first matching RID and treating a missing entry as a successful no-op.
- `btree.NewBTreeIndex` returns Go errors with wrapped storage causes and no partial index; the caller rolls back transaction-owned appended blocks after construction fails.
- `btree.BTreeIndex.BeforeFirst` reports traversal and pin failures as Go errors; a page whose unpin failed remains owned by the index for a later cleanup retry.
- `btree.BTreeIndex.Next` reports an unpositioned cursor and leaf-advance failures as Go errors while preserving wrapped storage causes.
- `btree.BTreeIndex.GetDataRID` reports nil or unpositioned cursor state as Go errors and preserves wrapped leaf/page-read causes.
- `btree.BTreeIndex.Close` attempts both retained page closes, joins errors, and preserves failed handles for retry.
- `btree.BTreeIndex.Insert` closes the modified leaf before directory access, propagates splits, retains root block zero, and leaves transaction rollback to the caller after an error.
- `btree.BTreeIndex.Delete` removes only the requested leaf value/RID pair without directory mutation, and leaves transaction rollback to the caller after an error.
- `IndexSelectScan` reports index-position failures as Go errors and leaves index lifetime management to its caller.
- `IndexSelectScan.Next` reports index and table-scan failures as errors, and avoids moving the data scan after exhaustion.
- `IndexSelectScan.GetInt` reports data-scan read failures as Go errors while preserving their wrapped causes.
- `IndexSelectScan.GetString` reports data-scan read failures as Go errors while preserving their wrapped causes.
- `IndexSelectScan.GetVal` uses the query-level record adapter to return schema-typed constants without making the record package depend on query types.
- `IndexSelectScan.HasField` reports false for invalid scan state and otherwise delegates to the table schema.
- `IndexSelectScan.Close` attempts both retained resources in Java order and joins all Go close errors.
- `IndexJoinScan.Close` attempts all retained resources in Java order and joins Go close errors while preserving each cause.
- `btree.NewPage` rejects invalid transaction, layout, and block inputs before pinning and returns pin failures as Go errors with their original causes.
- `btree.Page.Close` preserves the current block after an unpin error so the caller can retry; repeated successful close calls are no-ops.
- `btree.Page.IsFull` returns contextual Go errors for invalid page state, invalid capacity metadata, arithmetic overflow, and block-size reads instead of relying on Java's unchecked transaction calls.
- `btree.Page.GetDataVal` returns typed query constants and contextual Go errors for invalid slots, layout metadata, and storage reads instead of relying on Java's unchecked field access.
- `btree.Page.GetFlag` reports transaction read failures as Go errors while preserving the signed Java flag value.
- `btree.Page.SetFlag` reports transaction write failures as Go errors while preserving the signed Java flag value and logging request.
- `btree.Page.SetDataRID` reports invalid or out-of-range identifiers and transaction write failures as Go errors, and logs both leaf RID fields separately.
- `btree.Page.InsertDir` copies later directory entries from right to left before inserting a key/child pair; storage errors remain explicit and transaction logging enables caller rollback.
- `btree.Page.InsertDirectoryEntry` validates directory level and physical capacity before finding the ordered insertion point; directory splitting remains a separate operation.
- `btree.Page.SplitDirectoryPage` splits only an already full directory page and returns a parent separator; recursive propagation and root growth remain separate operations.
- `btree.Page.InsertDirectoryEntryAndSplit` combines insertion with the page-full split decision but leaves parent recursion and root growth to separate operations.
- `btree.Page.InsertDirectoryEntryRecursive` recurses through existing directory pages and returns split separators upward; creating a new root remains the caller's responsibility.
- `btree.Page.MakeNewRoot` keeps the root in block zero, moves old entries to a new page, installs two child entries, and raises the level last.
- `btree.Page.InitializeDirectoryRoot` formats an allocated block-zero directory root and adds the initial minimum-key entry for leaf block zero.
- `btree.Page.InitializeLeafPage` formats an already allocated leaf block with overflow flag -1 and an empty record count.
- `btree.ensureLeafFile` appends and initializes the first leaf block only when the index leaf file is empty, then releases the temporary page pin.
- `btree.ensureDirectoryFile` creates the block-zero minimum-key root only when the directory file is empty and chooses Java's integer or varchar sentinel from the key schema.
- `btree.directoryLayoutFromLeaf` derives the directory `block` and `dataval` schema as a pure layout operation; file initialization and full index construction remain separate.

## Completion audit

The 89 numbered behavior records above complete the chapter's source map: the index contract, static hash key and cost
rules, hash-index lifecycle, index-select and index-join adapters/plans, and B-tree page, cursor, overflow, split,
directory propagation, root growth, index lifecycle, insertion, deletion, and cleanup behavior are all represented.
The Java behavior is adapted to the repository's `query.Constant`, `record.RID`, transaction, and buffer contracts;
the intentional error, cleanup, duplicate, and rollback differences above define the boundaries of that adaptation.

The existing index-contract, B-tree page-format, leaf search, directory insertion, and root-growth diagrams already
cover the chapter's storage transitions and link to editable Draw.io sources, so no additional diagram is needed.
Chapter 12 is complete on the ordered PR stack; the README status refers to that landing order rather than implying
that the stack has already been merged.
