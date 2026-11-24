# Chapter 11: Indexing

Indexes provide a second access path from a search value to the record identifiers stored in a table. This chapter
introduces the index contract before adding any concrete hash or B-tree storage. Search positioning, matching-entry
iteration, RID retrieval, insertion, deletion, and resource cleanup remain separate observable behaviors so each
implementation can preserve the same interface.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/index.Index` | `simpledb/index/Index.java` | Position by a search key, traverse matching entries, return data RIDs, mutate value/RID entries, and close resources |
| `internal/index/hash.HashIndex` | `simpledb/index/hash/HashIndex.java` | Retain the transaction, index name, record layout, and current traversal state for a static hash index |
| `internal/index/btree.Page` | `simpledb/index/btree/BTPage.java` | Retain and pin a B-tree block, then position before a sorted search key |
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
algorithm. The [index-contract diagram](../diagrams/ch11-index-contract.svg) shows the search-value and RID boundary;
the editable source is [`ch11-index-contract.drawio`](../diagrams/ch11-index-contract.drawio).

### Test-first record

The first focused test run failed because the `internal/index` package and `Index` contract were undefined. A compile-time
probe implements the six methods, and a focused contract test exercises search positioning, traversal results, RID
retrieval, insertion, deletion, close, and error propagation.

## Feature 2: Map search keys to static hash buckets

`index/hash.BucketNumber` selects one of 100 fixed buckets. Integer values use their signed 32-bit value as the hash;
string values use Java `String.hashCode` over UTF-16 code units, including surrogate pairs for supplementary Unicode
characters. The remainder is normalized into `[0, 100)` so negative keys cannot select an invalid bucket. An unset
`query.Constant` returns an error instead of silently choosing a bucket. The [hash bucket diagram](../diagrams/ch11-hash-bucket.svg)
shows how a query value becomes a bucket number; its editable source is
[`ch11-hash-bucket.drawio`](../diagrams/ch11-hash-bucket.drawio).

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

The [hash-index traversal diagram](../diagrams/ch11-hash-traversal.svg) shows the collision-filter loop and its two
observable outcomes; the editable source is [`ch11-hash-traversal.drawio`](../diagrams/ch11-hash-traversal.drawio).

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

The [hash-index insertion diagram](../diagrams/ch11-hash-insert.svg) shows the validation, old-scan release, bucket
selection, row write, and retained scan; the editable source is [`ch11-hash-insert.drawio`](../diagrams/ch11-hash-insert.drawio).

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
retain their causes. The existing [hash-index traversal diagram](../diagrams/ch11-hash-traversal.svg) shows the bucket
filter loop that deletion reuses, so a second diagram would duplicate that behavior.

### Test-first record

The focused test run first failed because `HashIndex.Delete` was undefined. Tests cover skipping hash collisions,
removing only the first key/RID match, leaving entries unchanged for absent keys or RIDs, retaining the positioned
scan, rejecting invalid state, and wrapping bucket-open, iteration, RID-read, and slot-delete failures.

## Feature 11: Position an index-select scan

`indexquery.NewIndexSelectScan` retains a `query.RecordScan`, index, and search key, then positions the index before the first
matching entry as Java `IndexSelectScan` does during construction. Its `BeforeFirst` operation repeats that positioning
for the retained key. Missing scans/indexes, unset keys, and index errors return contextual errors with wrapped causes;
the index remains caller-owned and is not closed by the adapter. The existing [hash-index traversal diagram](../diagrams/ch11-hash-traversal.svg)
already documents bucket positioning, so this adapter initialization does not need another diagram.

### Test-first record

The focused test run first failed because `NewIndexSelectScan` was undefined. Tests verify constructor positioning,
repositioning with the stored key, nil dependencies/receivers, unset keys, and initial/repeated index-position errors.

## Feature 12: Advance to the next indexed data row

`IndexSelectScan.Next` advances the index once. Exhaustion returns `(false, nil)` without moving the data table scan;
for an entry, the method reads its `record.RID`, moves the table scan to that row, and returns `(true, nil)`. Index,
RID-read, and table-movement failures return contextual errors with wrapped causes while retaining both scan references.
The existing [hash-index traversal diagram](../diagrams/ch11-hash-traversal.svg) already shows the entry-to-RID step,
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
by the page lifecycle; closing the page remains a separate behavior. The [B-tree page pin diagram](../diagrams/ch11-btree-page-pin.svg)
shows the transaction pin and the retained block/layout; its editable source is
[`ch11-btree-page-pin.drawio`](../diagrams/ch11-btree-page-pin.drawio).

### Test-first record

The focused tests first failed because `NewPage` and `PageTransaction` were undefined. Tests verify a single pin of the
requested block, retained state, nil and typed-nil transaction rejection, missing layout/schema, invalid block
identifiers, and a pin failure whose original cause remains inspectable. The constructor performs no page reads or
writes.

The [B-tree page lifecycle diagram](../diagrams/ch11-btree-page-pin.svg) also shows the inverse `Close`/`Unpin`
operation and the retry-safe state transition; the editable source is
[`ch11-btree-page-pin.drawio`](../diagrams/ch11-btree-page-pin.drawio).

## Feature 37: Position before a B-tree search key

Java `BTPage.findSlotBefore` scans sorted `dataval` entries and returns the slot immediately before the first value
that is greater than or equal to the requested key. Go's `Page.FindSlotBefore` reads the page record count at byte
offset 4, calculates each field position from the eight-byte page header and record layout, and supports integer and
varchar keys; varchar ordering follows Java's UTF-16 code units. Duplicate keys return the position before the first duplicate; empty pages and keys preceding the first
entry return `-1`. Invalid types, malformed offsets, negative record counts, and storage read failures are returned as
contextual errors without changing the pinned page. The [B-tree slot search diagram](../diagrams/ch11-btree-slot-search.svg)
shows the lower-bound position and duplicate behavior; its editable source is
[`ch11-btree-slot-search.drawio`](../diagrams/ch11-btree-slot-search.drawio).

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
The [B-tree page capacity diagram](../diagrams/ch11-btree-page-capacity.svg) shows the boundary comparison and its
editable source is [`ch11-btree-page-capacity.drawio`](../diagrams/ch11-btree-page-capacity.drawio).

### Test-first record

The focused tests first failed because `Page.IsFull` was undefined. Tests cover a fitting next slot, exact capacity,
an over-capacity position, invalid and closed state, negative counts, invalid block sizes, position overflow, and
wrapped count/block-size read failures.

## Feature 40: Read a B-tree page data value

Java `BTPage.getDataVal` reads the `dataval` field from the requested slot and returns its schema type. Go's
`Page.GetDataVal` computes the field position from the eight-byte page header, slot index, layout slot size, and
`dataval` offset, then returns an integer or varchar `query.Constant`. It checks the slot calculation for overflow,
rejects invalid or closed page/layout state and unsupported field types, and wraps storage read errors. The accessor
does not change page pin state. The [B-tree data value diagram](../diagrams/ch11-btree-page-data-value.svg) shows the
field-offset calculation and its editable source is [`ch11-btree-page-data-value.drawio`](../diagrams/ch11-btree-page-data-value.drawio).

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

## Intentional differences from Java

- Fallible operations return Go `error` values instead of relying on unchecked Java calls or exceptions.
- The contract uses the existing `query.Constant` and `record.RID` value types; concrete hash and B-tree indexes remain separate implementations.
- Static hash bucket selection normalizes Java hash remainders to a nonnegative range, avoiding Java's negative-remainder invalid-index case.
- Go insertion validates the index-record schema and signed 32-bit RID storage before changing scan state, and wraps storage failures as explicit errors.
- Go deletion reports storage failures as errors while preserving the Java behavior of removing the first matching RID and treating a missing entry as a successful no-op.
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
