# Chapter 14: Effective Buffer Utilization

This chapter records buffer-allocation helpers and scans used to process relations in bounded chunks. Each helper and
scan contract remains a separate observable behavior so buffer estimates can be tested independently from storage
operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/multibuffer.BestRoot`, `BestFactor` | `simpledb/multibuffer/BufferNeeds.java` (`bestRoot`, `bestFactor`) | Choose a ceiling root or quotient of a relation's block count within the usable buffer budget |
| `internal/multibuffer.ChunkScan` | `simpledb/multibuffer/ChunkScan.java` (`beforeFirst`, `next`, `getInt`, `getString`, `hasField`) | Own a bounded block range, traverse it in block/slot order, read current fields, and report declared field membership |
| `internal/plan.ChunkValueScan` | `simpledb/multibuffer/ChunkScan.java` (`getVal`) | Adapt typed chunk field reads to query constants without making `multibuffer` depend on `query` |
| `internal/plan.MultibufferProductScan` | `simpledb/multibuffer/MultibufferProductScan.java` (constructor, `beforeFirst`, `next`, `useNextChunk`) | Read right-table size and buffer budget, choose bounded ranges, pin and replace chunks, reset the left scan, and enumerate left-major pairs |
| `internal/materialize.MultibufferProductPlan` | `simpledb/multibuffer/MultibufferProductPlan.java` (`schema`, `recordsOutput`, `blocksAccessed`, `open`, `copyRecordsFrom`) | Combine child schemas in left/right order, estimate product rows and chunked scan cost, materialize the right plan into a temporary table, then open an owned left-major multi-buffer product scan |


## Feature 1: Choose a safe buffer root

`multibuffer.BestRoot` reserves two buffers from the available count. For each positive exponent, it computes
`ceil(size^(1/exponent))` and returns the first root that fits the remaining budget. When at most one buffer remains
usable or the relation has at most one block, it returns one. Nonpositive sizes are handled safely with the same
one-buffer fallback. The
[buffer-root estimator diagram](../diagrams/ch14-buffer-needs-best-root.svg) shows the reservation and exponent-search
loop; the editable source is [`ch14-buffer-needs-best-root.drawio`](../diagrams/ch14-buffer-needs-best-root.drawio).

### Test-first record

Focused tests first failed to compile because `BestRoot` was undefined. Table-driven cases cover reservation
boundaries, exact and non-perfect roots, one and invalid sizes, and the largest positive Go integer. An exhaustive small
input check compares the result with Java's increasing-exponent search.

### Design decision

Java uses floating-point `Math.pow` and casts its ceiling to `int`; Go retains the same exponent-search formula so
rounding at exact-root boundaries remains compatible. Before converting the result, Go saturates at the maximum
representable `int` so a large input cannot wrap to a negative allocation. A nonpositive block count returns one as a
safe allocation fallback; the Java helper does not define that invalid-input case. This helper performs no transaction
or buffer-manager operations.

### Validation

The focused package tests cover every executable statement and function. The module coverage workflow continues to
require 100.0% statement coverage for every executable package, function, and the module total.

## Feature 2: Choose a safe buffer factor

`multibuffer.BestFactor` reserves the same two buffers and selects the first ceiling quotient `ceil(size/divisor)`
that fits the remaining budget. The
[buffer-factor estimator diagram](../diagrams/ch14-buffer-needs-best-factor.svg) shows the quotient search; the
editable source is [`ch14-buffer-needs-best-factor.drawio`](../diagrams/ch14-buffer-needs-best-factor.drawio).

### Test-first record

Focused tests first failed to compile because `BestFactor` was undefined. Table-driven tests cover buffer reservation,
exact and ceiling quotients, one and invalid sizes, and maximum Go integers. An exhaustive small-input test compares
the selected value with Java's increasing-divisor formula.

### Design decision

Java tries divisors linearly and evaluates each quotient with floating-point `Math.ceil`. Go keeps that quotient
calculation and its rounding behavior, but uses binary search over the monotone quotient sequence to find the same
first divisor without a linear walk through very large block counts. Conversion saturates at the maximum Go `int`,
preventing wraparound for large sizes. Nonpositive sizes return one as a safe fallback; Java leaves that input
undefined. This estimator performs no transaction or buffer-manager operations; chunk scanning remains separate.

### Validation

The focused package tests cover every executable statement and function. The full module coverage workflow continues
to require 100.0% statement coverage for every executable package, function, and module total.

## Feature 3: Pin and release a bounded chunk

`multibuffer.NewChunkScan` validates an inclusive, non-negative block range and pins one `record.RecordPage` for
each block in ascending order. `ChunkScan.Close` attempts every release and retains only pages whose unpin operation
failed, allowing a later close call to retry those releases without repeating successful unpins. If construction
fails after earlier pins were acquired, it releases them before returning; when cleanup itself fails, the returned
partial scan retains those failed releases so the caller can retry `Close`.

The [chunk page-lifetime diagram](../diagrams/ch14-chunk-scan-pins.svg) shows range validation, ordered pinning,
failure cleanup, and retryable release; its editable source is
[`ch14-chunk-scan-pins.drawio`](../diagrams/ch14-chunk-scan-pins.drawio). Page traversal, rewind, and field access
remain separate scan behaviors.

### Test-first record

Focused tests first failed to compile because `NewChunkScan`, `ChunkTransaction`, and `ChunkScan` were undefined.
Tests cover invalid configuration before I/O, inclusive ascending pins, cleanup after a later pin failure, joined pin
and unpin causes, retention and retry of failed releases, aggregation across multiple close failures, and repeated
close after success.

### Design decision

The Go boundary embeds `record.RecordTransaction` and adds only the `Unpin` operation required by the chunk resource
lifecycle. `ChunkScan` retains `RecordPage` handles rather than duplicating block pin logic. The inclusive loop stops
before incrementing the final block number, including when it is the maximum `int`. If a pin failure is followed by a
cleanup failure, construction returns both the original error and a partial scan that owns only the pins that still
need release. Unlike Java's constructor, Go validates the range and exposes cleanup errors rather than silently
assuming every pin succeeded. This type does not yet implement `query.Scan`; cursor movement and record values are
independent behaviors.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 4: Advance within the active chunk block

`ChunkScan.BeforeFirst` positions the cursor before slot -1 in the first pinned page. `ChunkScan.Next` uses the active
`RecordPage.NextAfter` to find one occupied slot and returns false when that block is exhausted. Rewinding clears the
exhausted state without changing page pins. Advancing into another block remains a separate behavior, so an occupied
record in a later page is not read by this operation.

### Test-first record

Focused tests first failed because the cursor fields, `BeforeFirst`, and `Next` were missing. Tests cover first-page
position, occupied and empty slots, repeated exhaustion without more I/O, rewind after scanning, unchanged pins,
zero-value/partial/closed cursor errors, block-size failures, and record-flag read failures.

### Design decision

The constructor marks a scan ready only after every page is pinned; a partial scan returned for cleanup retry cannot
be traversed. `Close` marks the scan closed before attempting unpins, so failed releases remain retryable through
`Close` but cannot be read as if every page were still pinned. `Next` deliberately stops at the active page boundary;
the next block is not selected implicitly in this feature.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 5: Advance across chunk blocks

When `ChunkScan.Next` reaches the end of its active page, it advances to each remaining pinned page, resets the slot
cursor to -1, and searches there. Empty pages are skipped; exhaustion is remembered only after the final page has no
occupied record. This follows Java `ChunkScan.next` while keeping page ownership fixed for the life of the chunk.

The [chunk rollover diagram](../diagrams/ch14-chunk-scan-rollover.svg) shows the row search, page transition, and
final exhaustion state. Its editable source is [`ch14-chunk-scan-rollover.drawio`](../diagrams/ch14-chunk-scan-rollover.drawio).

### Test-first record

Focused tests first failed because `Next` returned false at the first page boundary. Tests cover crossing to the next
page, skipping multiple empty pages, rewinding after crossing, final-range exhaustion without repeated reads, stable
pin ownership, and a wrapped record-flag error in a later page.

### Design decision

The cursor advances one page only after `RecordPage.NextAfter` completes successfully with no occupied slot. A read
failure therefore leaves the cursor on that destination page at slot -1 so a caller can retry. The scan does not pin
or unpin during traversal; its resource-lifecycle contract remains in the preceding feature. Typed value reads remain
separate.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 6: Read an integer field from the current chunk record

`ChunkScan.GetInt` reads through the active `RecordPage` at the current slot. It uses the page selected by block-order
traversal and does not move the cursor or change pin ownership. The accessor rejects scans without a current record,
including before-first and exhausted states, before reading a field.

### Test-first record

Focused tests first failed because `ChunkScan.GetInt` was undefined. Tests cover integer values before and after page
rollover, unchanged cursor and pins, missing field names, wrapped transaction read errors, and nil, zero-value, partial,
closed, before-first, and exhausted scan states.

### Design decision

The accessor delegates offset calculation and field lookup to `RecordPage.GetInt`; the chunk scan adds block context
while preserving the underlying error. A failed next call at end-of-range cannot leave the previous record readable.
String reads, query constants, and field-membership checks remain separate behaviors.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 7: Read a string field from the current chunk record

`ChunkScan.GetString` reads through the active `RecordPage` at the current slot, including after traversal advances to
a later block. It leaves cursor position and page pins unchanged. Before-first, exhausted, closed, partial, invalid,
zero-value, and nil scans return an error before field I/O.

### Test-first record

Focused tests first failed because `ChunkScan.GetString` was undefined. Tests cover string values on the first and later
pages, unchanged cursor and pins, missing field names, wrapped transaction read errors, and invalid cursor states.

### Design decision

The accessor delegates offset calculation and field lookup to `RecordPage.GetString`; it adds block context while
preserving the storage error. Query-value conversion and field-membership checks remain separate behaviors.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 8: Report declared field membership

`ChunkScan.HasField` consults the scan layout's schema and does not depend on cursor position or page lifecycle. A nil
receiver, zero-value scan, or layout without a schema reports false. The method does not pin pages or read record data.

### Test-first record

Focused compilation first failed because `ChunkScan.HasField` was undefined. Tests cover integer and varchar fields,
missing and empty names, nil and zero-value scans, closed scans, and the absence of transaction reads or new pins.

### Design decision

The chunk scan delegates membership to `record.Schema` because field existence is schema metadata rather than a current
record value. Query-value conversion and query-plan integration remain separate behaviors.

### Validation

The focused package suite covers every executable statement and function. The module coverage workflow continues to
require 100.0% statement coverage for every executable package, function, and module total.

## Feature 9: Read query values from a chunk record

`plan.ChunkValueScan` embeds a `multibuffer.ChunkScan` and converts the current record field to the matching
`query.Constant`. Integer and varchar fields use the chunk scan's typed getters, so cursor validation, active-page
selection, storage error context, and pin ownership remain centralized in the lower-level scan.

### Test-first record

Focused compilation first failed because `ChunkValueScan` and `NewChunkValueScan` were undefined. Tests cover integer
and string constants on the first and later pages, repeated reads without advancing, missing fields, wrapped storage
errors, unsupported schema types, and unavailable adapter/cursor states that must not read field data.

### Design decision

The adapter lives in `plan`, which can depend on both `multibuffer` and `query`; `multibuffer` remains independent of
the query constant type. The wrapper embeds the existing chunk scan to preserve traversal, field access, metadata, and
close behavior instead of duplicating cursor or resource ownership.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 10: Initialize a multibuffer product scan

`plan.NewMultibufferProductScan` validates the transaction, left scan, right table name, and record layout. It reads the
right relation's block count from `<table>.tbl`, obtains the available-buffer count only when the relation is
nonempty, and calls `multibuffer.BestFactor` to choose the first bounded block range. It pins that range through
`multibuffer.NewChunkScan`, adapts the chunk to query values, and rewinds the left scan before returning. An empty right
relation is initialized without pins or a left rewind. The scan owns its active right chunk and retains ownership of the
left scan with its caller. `Close` can retry failed page releases. Product-row traversal and later chunk rollover are
recorded as separate behaviors below.

The [product-scan initialization diagram](../diagrams/ch14-multibuffer-product-init.svg) shows the sizing, buffer
selection, first-range pinning, and left rewind performed before row traversal. Its editable source is
[`ch14-multibuffer-product-init.drawio`](../diagrams/ch14-multibuffer-product-init.drawio).

### Test-first record

Focused compilation first failed because `MultibufferProductScan` and its constructor were undefined. Tests cover
nonempty and empty relations, `.tbl` naming, `BestFactor` sizing, invalid inputs and negative sizes/budgets, wrapped
file-size errors, initial pin failures, left rewind failures, cleanup-error joining, partial-pin retention, and retryable
close behavior.

### Design decision

The plan package owns this orchestration because it depends on both the storage-oriented multibuffer scan and the
query-facing scan contract. `multibuffer` remains independent of `query`. The first chunk is represented by
`ChunkValueScan`; row advancement and subsequent chunk replacement are implemented by later features.

### Validation

The focused tests cover every executable statement and function. The module coverage workflow continues to require
100.0% statement coverage for every executable package, function, and module total.

## Feature 11: Advance a product scan within its current chunk

`MultibufferProductScan.Next` delegates row pairing to `query.ProductScan` while retaining the first right-hand chunk
selected during initialization. The product scan is left-major: every right record in the current chunk is paired with
one left record before the left input advances. `BeforeFirst` rewinds both inputs, and repeated exhaustion does not
read pages again. An empty right relation returns no rows without touching the left scan. The active chunk remains
pinned throughout its rows. Replacing it with the next chunk is handled by Feature 12.

The [in-chunk product traversal diagram](../diagrams/ch14-multibuffer-product-traversal.svg) shows the left-major
pair order and the right-scan rewind for each left row. Its editable source is
[`ch14-multibuffer-product-traversal.drawio`](../diagrams/ch14-multibuffer-product-traversal.drawio).

### Test-first record

Focused compilation first failed because `MultibufferProductScan.Next` and `BeforeFirst` were undefined. Tests cover
left-major pairs, repeated right rows, explicit rewind after exhaustion, stable exhaustion, empty input, left/right
read and rewind errors, product construction errors, invalid/closed scan states, and unchanged page pins.

### Design decision

The plan scan composes the existing `query.ProductScan` rather than duplicating its pairing state. The product is
created lazily so the constructor remains limited to acquiring and positioning the first chunk. Field forwarding
remains separate.

### Validation

The focused plan tests cover every executable statement and function. The module coverage workflow continues to
require 100.0% statement coverage for every executable package, function, and module total.

## Feature 12: Move a multibuffer product scan to later chunks

After the current product reaches the end of its right chunk, `MultibufferProductScan.Next` releases that range before
pinning the next one. It bounds every inclusive range by both `chunkSize` and the right relation's block count, resets
the left scan for each range, and continues the same left-major pairing order. Empty chunks are skipped; the last range
may contain fewer than `chunkSize` blocks. The final active chunk remains pinned until `Close`, and repeated exhaustion
does not read or pin pages again.

The scan does not advance its saved next-block boundary until a later range is fully pinned and the left scan rewinds.
Failed old-range releases are retried before any new pin; a clean pin or rewind failure can be retried at the same range.
If partial acquisition cleanup fails, the scan retains the remaining page handles and joined causes until `Close`
releases them. Closing the product releases the current right chunk but leaves the left scan caller-owned.

The [chunk rollover diagram](../diagrams/ch14-multibuffer-product-rollover.svg) shows the release, bounded next-range
acquisition, and left rewind order. Its editable source is
[`ch14-multibuffer-product-rollover.drawio`](../diagrams/ch14-multibuffer-product-rollover.drawio).

### Test-first record

The new tests first failed because `Next` stopped after the initially pinned chunk. Tests cover left-major output over
multiple chunks, a bounded final chunk, empty earlier chunks, stable exhaustion, unchanged pins while current rows
remain, failed-release retry ordering, later pin failure and retry, partial-pin cleanup, left-rewind failure and retry,
and final-chunk release on close.

### Design decision

Each chunk transition is a retryable resource boundary. The `nextBlock` cursor advances only after all pages are pinned
and the left scan has rewound successfully. A partially pinned range is not traversable: it is retained only so the
caller can retry cleanup with `Close`. The last chunk stays open after row exhaustion until the caller closes the scan,
matching scan resource ownership and avoiding needless release/re-pin on repeated `Next` calls.

### Validation

The focused plan tests cover every executable statement and function. The independent module coverage workflow requires
100.0% statement coverage for every executable package, function, and module total.

## Feature 13: Read query values from the current product row

`MultibufferProductScan.GetVal` delegates field lookup to the active `query.ProductScan`, so fields from both the
caller-owned left scan and the current right-hand chunk are exposed as `query.Constant` values. It is available only
after `Next` returns a row and becomes unavailable after rewind, failed or exhausted advancement, or close. Repeated
reads leave the product cursor and chunk pins unchanged. Errors preserve the underlying query cause while adding
field and right-table context.

### Test-first record

Focused tests first failed to compile because `MultibufferProductScan.GetVal` was undefined. Tests cover left and
right integer and string fields, repeat reads, unchanged pins and left cursor position, missing fields with wrapped
context, reads before the first row and after exhaustion, rewind, and close, plus nil and zero-value scans.

### Design decision

The product scan delegates to its existing `query.ProductScan` instead of reconstructing field ownership or
conversion. Explicit current-row state prevents stale values from being exposed after the scan moves away from a row.
Field-specific convenience accessors and query-plan construction remain separate behaviors.

### Validation

The focused `internal/plan` tests cover every executable statement and function. The independent coverage workflow
requires 100.0% statement coverage for every executable package, function, and module total.

## Feature 14: Report fields available from a multibuffer product

`MultibufferProductScan.HasField` reports whether the caller-owned left scan or the right record-layout schema
declares a field. It is schema metadata, so it works before traversal, after rewind or exhaustion, for an empty right
relation, and after close. Nil, zero-value, and schema-less scans report false. The check does not create the lazy
`query.ProductScan`, read a row, move either input, or change right-chunk pins.

### Test-first record

Focused compilation first failed because `MultibufferProductScan.HasField` was undefined. Tests cover left/right
field unions, absent names, empty relations, nil/zero/schema-less scans, and calls before, during, and after traversal
and close while checking row-read and page-pin state.

### Design decision

Field membership depends on the two input schemas, not on the current product row or active chunk. The plan scan
checks the left scan and the retained right layout directly, avoiding creation of the lazy row-pairing operator and
keeping metadata queries independent of cursor and resource lifecycle.

### Validation

The focused `internal/plan` tests cover every executable statement and function. The independent coverage workflow
requires 100.0% statement coverage for every executable package, function, and module total.

## Feature 15: Read an integer field from the current product row

`MultibufferProductScan.GetInt` delegates integer field selection to the active `query.ProductScan`, supporting fields
from both the caller-owned left scan and the current right-hand chunk. It requires a current product row; before-first,
rewound, exhausted, closed, nil, and zero-value scans return errors before value reads. Repeated reads preserve both
input cursors and page pins, and storage/query failures retain their causes with field and right-table context.

### Test-first record

Focused compilation first failed because `MultibufferProductScan.GetInt` was undefined. Tests cover repeated left and
right integer reads, missing fields, wrapped right-page read errors, unavailable row states, nil and zero-value scans,
and stable cursors and pins.

### Design decision

The accessor delegates to `query.ProductScan.GetInt` so its existing left-before-right field selection and storage
semantics remain authoritative. String access and planner construction remain separate behaviors.

### Validation

The focused `internal/plan` tests cover every executable statement and function. The independent coverage workflow
requires 100.0% statement coverage for every executable package, function, and module total.

## Feature 16: Read a string field from the current product row

`MultibufferProductScan.GetString` delegates varchar field selection to the active `query.ProductScan`, supporting
both the caller-owned left scan and the current right-hand chunk. It requires a current product row; before-first,
rewound, exhausted, closed, nil, and zero-value scans return errors before value reads. Repeated reads preserve both
input cursors and page pins, and query/storage failures retain their causes with field and right-table context.

### Test-first record

Focused compilation first failed because `MultibufferProductScan.GetString` was undefined. Tests cover repeated left
and right string reads, missing fields, wrapped right-page read errors, unavailable row states, nil and zero-value
scans, and stable cursors and pins.

### Design decision

The accessor delegates to `query.ProductScan.GetString` so the existing left-before-right field selection and
storage semantics remain authoritative. Integer access and planner construction remain separate behaviors.

### Validation

The focused `internal/plan` tests cover every executable statement and function. The independent coverage workflow
requires 100.0% statement coverage for every executable package, function, and module total.

## Feature 17: Open a materialized multibuffer product plan

`materialize.MultibufferProductPlan.Open` opens the left plan, copies right-plan rows and typed values to a temporary
table, and initializes `plan.MultibufferProductScan` over the temporary relation. The returned scan emits the existing
left-major product order and owns the left scan opened by the plan. `Close` releases both the active right chunk and
the owned left scan; failed chunk releases retain the product scan so callers can retry cleanup. Open failures preserve
their source cause and phase context while closing scans already acquired.

The [multibuffer product-plan open diagram](../diagrams/ch14-multibuffer-product-plan-open.svg) shows how both query
plans converge on a scan: the right rows pass through a temporary table and bounded chunk scan, while the left scan is
retained by the returned owner wrapper. The editable source is
[`ch14-multibuffer-product-plan-open.drawio`](../diagrams/ch14-multibuffer-product-plan-open.drawio).

### Test-first record

Focused compilation first failed because `MultibufferProductPlan` and its constructor were undefined. Tests cover
left-major rows from both inputs, rewind and typed field forwarding, right-side materialization, nil and partial
inputs, copy/read/write failures, cleanup errors, returned-scan ownership, and retrying a failed chunk release.

### Design decision

The plan lives in `materialize` so it can create and populate a `TempTable` while depending on the lower-level plan
contract and `plan.MultibufferProductScan`. Keeping this composition outside `plan` avoids making that package import
`materialize`, which already imports `plan` for `MaterializePlan`. Plan schema composition and estimate methods remain
separate behaviors.

### Validation

The focused `internal/materialize` suite and uncached full `go test ./... -count=1 -coverprofile=...` report
100.0% statement coverage, including every executable package and function; the interface-only `internal/index`
package has no statements. The race suite, vet, documentation validator, and independent workflow audit also pass.

## Feature 18: Combine multibuffer product schemas

`materialize.MultibufferProductPlan.Schema` returns a new schema with the left plan's fields followed by the right
plan's fields. Field types and varchar lengths are retained, and neither child schema is mutated. The accessor reads
only child metadata: it does not open scans or touch transaction state. A nil plan, child, or child schema yields a
nil result.

### Test-first record

Focused compilation first failed because `MultibufferProductPlan.Schema` was undefined. Tests cover field order,
integer/varchar definitions and lengths, non-mutation of both inputs, absence of scan/transaction calls, and nil receiver,
child-plan, and child-schema cases.

### Design decision

The plan composes schemas by using `record.Schema.AddAllFrom` in left-to-right order, matching product field lookup
order while returning a distinct schema. Row-count, distinct-value, and block-access estimates remain separate
behaviors.

### Validation

The focused tests exercise every new executable statement and function. Full-module tests, race tests, vet,
documentation validation, and the independent one-check-per-workflow audit are recorded with this Issue's PR.

## Feature 19: Estimate multibuffer product output rows

`materialize.MultibufferProductPlan.RecordsOutput` multiplies the row-count estimates of its left and right plans,
using signed 32-bit multiplication to match the Java `int` arithmetic used by the book. A nil plan or child returns
zero. The estimate does not open scans, inspect schemas, or read transaction state.

### Test-first record

Focused compilation first failed because `MultibufferProductPlan.RecordsOutput` was undefined. Tests cover a normal
product estimate, signed 32-bit overflow behavior, nil receiver and children, and the absence of scan, schema, or
transaction calls.

### Design decision

The implementation follows `plan.ProductPlan.RecordsOutput` and the book's 32-bit Java integer behavior. Block-access
and distinct-value estimates remain separate behaviors.

### Validation

The focused tests cover every new executable statement and function. Full-module tests, race tests, vet, documentation
validation, and the independent one-check-per-workflow audit are recorded with this Issue's PR.

## Feature 20: Estimate multibuffer product block access cost

`materialize.MultibufferProductPlan.BlocksAccessed` estimates reading the materialized right input once, then scanning
the left input once for each right-side chunk. It derives the right-side block count from the right plan's estimated
rows and schema, chooses the bounded chunk size with `multibuffer.BestFactor`, and counts a final partial chunk. The
estimate follows the local plan convention of excluding the one-time cost to produce a materialized relation. Missing
dependencies, an empty right input, or unavailable block sizing yields zero.

### Test-first record

Focused compilation first failed because `MultibufferProductPlan.BlocksAccessed` was undefined. Tests cover whole and
partial chunks, signed 32-bit Java-compatible cost arithmetic, unavailable metadata, and the absence of scan or storage
I/O while estimating.

### Design decision

The cost model uses the same `BestFactor` rule as the product scan so estimated chunk count matches execution. It asks
the right plan for row count and schema through `MaterializePlan.BlocksAccessed`; it does not include right-plan
materialization work, matching `MaterializePlan` and `SortPlan`'s established estimate boundary. Distinct-value
estimation remains a separate behavior.

### Validation

Focused tests cover every statement in the new method. The full-module coverage, race, vet, documentation, formatting,
and one-check-per-workflow audit results are recorded in the recovery checkpoint and stacked PR.
