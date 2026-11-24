# Chapter 13: Materialization and Sorting

Materialization turns an intermediate query result into a temporary relation that later operators can scan. This chapter
keeps descriptor creation, opening the relation, and sorting as separate behaviors: the descriptor alone must not create
a file or issue transaction operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/materialize.TempTable`, `NewTempTable` | `simpledb/materialize/TempTable.java` | Allocate a unique temporary relation name and retain its transaction and record layout before opening a scan |
| `internal/materialize.MaterializePlan` | `simpledb/materialize/MaterializePlan.java` | Copy an underlying query scan into a temporary table and expose its output scan and cost estimates |
| `internal/materialize.RecordComparator` | `simpledb/materialize/RecordComparator.java` | Compare the current values of two scans lexicographically in sort-field order |
| `internal/materialize.SortScan` | `simpledb/materialize/SortScan.java` | Merge one or two sorted temporary-table scans and expose the selected row as a query scan |

## Feature 1: Describe a temporary table

`materialize.NewTempTable` validates its transaction and schema, assigns a process-unique `tempN` name, and derives a
`record.Layout` from the supplied schema. `TableName` and `Layout` expose that descriptor. The retained transaction is
used by a later scan-opening behavior; construction performs no calls through it and creates no physical table. The
[temporary-table descriptor diagram](../diagrams/ch13-temp-table.svg) shows this boundary; the editable source is
[`ch13-temp-table.drawio`](../diagrams/ch13-temp-table.drawio).

### Test-first record

The focused test run first failed because `internal/materialize` had no non-test Go files. Tests cover the generated
name shape, retained transaction and schema identity, layout offsets/size, concurrent name uniqueness, nil dependencies,
and zero transaction calls during construction.

### Design decision

Java synchronizes an integer counter and `TempTable.open` immediately creates a `TableScan`. Go uses an atomic counter
for concurrent name allocation and introduces descriptor construction independently. That keeps this behavior
observable without touching storage; scan creation remains a distinct behavior. No catalog registration is added,
matching the temporary-table contract.

### Validation

The constructor, metadata accessors, and generated-name behavior are covered together with the current 100% statement
coverage target. The test transaction records every operation so descriptor construction cannot silently perform file
or buffer access.

## Feature 2: Open the temporary table scan

`TempTable.Open` passes the retained transaction, generated `tempN` name, and schema-derived layout to
`query.NewRecordScan`, which adapts the record-level scan to the `query.UpdateScan` contract. Its underlying
`record.NewTableScan` reads the temporary file size, appends and formats the first block when needed, and pins the
initial record page. Errors retain their causes and gain temporary-table context. The returned update scan owns its page
pin; the caller releases it through `Close`. The descriptor-boundary diagram shows query adaptation between the
metadata-only descriptor and physical access.

### Test-first record

The first focused build failed because `TempTable.Open` was undefined. Tests cover new and existing files, forwarding of
the descriptor name and schema layout, nil or incomplete descriptors without I/O, contextual failures at size/append/
pin/format steps, cleanup after format errors, and caller-owned scan close.

### Design decision

Java `TempTable.open` returns an `UpdateScan`. Go returns the narrow `query.UpdateScan` interface and uses the existing
`query.NewRecordScan` adapter to expose query constants while keeping `record` independent of `query`. The temporary
table remains outside the metadata catalog, and descriptor construction still performs no storage I/O.

## Feature 3: Compare records by ordered sort fields

`RecordComparator` compares each requested field in order through the query `ValueScan` contract. The first unequal
pair determines the ascending order; all equal fields, including an empty field list, compare equal. The constructor
copies the caller's field list. Read errors are contextual and preserve their causes, while unset or differently typed
constants return an error instead of relying on an unchecked comparison. The
[lexicographic comparator diagram](../diagrams/ch13-record-comparator.svg) shows the short-circuit path; the editable
source is [`ch13-record-comparator.drawio`](../diagrams/ch13-record-comparator.drawio).

### Test-first record

The first focused build failed because `NewRecordComparator` and `RecordComparator` were undefined. Tests cover integer
and string order, field-order copying, first-difference short-circuiting, all-equal and empty-key cases, nil inputs, and
wrapped scan-read errors plus unset or incompatible constants.

### Design decision

Java implements `Comparator<Scan>` and assumes its `Constant` comparison cannot fail. Go accepts `query.ValueScan`, the
smallest interface needed for value comparison, and returns an error because scan reads and typed constant comparison
can fail in this implementation. This keeps the comparator independent of storage traversal and update methods.

## Feature 4: Materialize query plan results

`MaterializePlan.Open` opens the source plan, creates a temporary-table descriptor from its output schema, copies each
row in schema field order through the `query.Scan` and `query.UpdateScan` contracts, closes the source, and returns the
temporary scan rewound before its first row. Empty input produces an empty scan. Any source read, destination write, or
rewind failure closes scans already acquired and retains the original error along with cleanup failures. The
[materialization flow diagram](../diagrams/ch13-materialize-plan.svg) shows the row-copy and ownership boundary; the
editable source is [`ch13-materialize-plan.drawio`](../diagrams/ch13-materialize-plan.drawio).

`Schema`, `RecordsOutput`, and `DistinctValues` preserve the source estimates. `BlocksAccessed` estimates only later reads
of the materialized file by dividing output records by records per block and rounding upward; it excludes the one-time
copy cost. Invalid or unusable block-size estimates return zero because the `plan.Plan` contract does not return errors
from cost methods.

### Test-first record

The focused test-first build failed because `MaterializePlan` and `NewMaterializePlan` were undefined. Tests cover
ordered multi-field copies, empty input, returned scan position and ownership, metadata delegation, exact and partial
block estimates, invalid dependencies, source-open/iteration/value/close errors, destination-open/insert/write errors,
rewind errors, and joined cleanup failures. Every statement in the new feature is exercised.

### Design decision

Java materialization performs the copy imperatively and returns an `UpdateScan`. Go expresses it as a `plan.Plan` and
returns the narrow `query.Scan` contract, with the source and temporary destination retaining their existing ownership
rules. The copy is synchronous and row-at-a-time; it does not introduce a second record representation or include the
materialization pass in the read-cost estimate.

## Feature 5: Merge sorted runs

`NewSortScan` opens one or two temporary tables that already contain rows in comparator order. `Next` advances the
previously selected run and compares the remaining heads, exposing the lower record through the `query.Scan` contract.
When keys tie, the second run is selected first, matching Java `SortScan`. A single run passes through unchanged;
exhaustion of both runs returns `false`. `BeforeFirst` rewinds both inputs and clears the current record. The
[sorted-run merge diagram](../diagrams/ch13-sort-scan.svg) shows how the comparator chooses a head and how the selected
run advances; the editable source is [`ch13-sort-scan.drawio`](../diagrams/ch13-sort-scan.drawio).

### Test-first record

The focused test-first compilation failed because `SortScan` and `NewSortScan` were undefined. Tests cover interleaving
and tie order, single and empty runs, rewind and exhaustion, delegated field reads, invalid inputs before storage use,
open and positioning failures, advancement and comparison failures, and joined cleanup errors. Every executable
statement in `internal/materialize` is covered.

### Design decision

Java's `SortScan` exposes a broad `Scan` API and compares its two current run heads. Go implements the same merge over
the narrower `query.Scan` contract, opens the runs from their `TempTable` descriptors, and returns errors for failed
storage operations rather than relying on Java's unchecked exceptions. The returned scan owns and closes both input
scans; the sort plan that creates runs remains a separate behavior.
