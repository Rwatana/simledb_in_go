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

## Intentional differences from Java

- Fallible operations return Go `error` values instead of relying on unchecked Java calls or exceptions.
- The contract uses the existing `query.Constant` and `record.RID` value types; concrete hash and B-tree indexes remain separate implementations.
- Static hash bucket selection normalizes Java hash remainders to a nonnegative range, avoiding Java's negative-remainder invalid-index case.
