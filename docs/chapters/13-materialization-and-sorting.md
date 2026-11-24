# Chapter 13: Materialization and Sorting

Materialization turns an intermediate query result into a temporary relation that later operators can scan. This chapter
keeps descriptor creation, opening the relation, and sorting as separate behaviors: the descriptor alone must not create
a file or issue transaction operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/materialize.TempTable`, `NewTempTable` | `simpledb/materialize/TempTable.java` | Allocate a unique temporary relation name and retain its transaction and record layout before opening a scan |

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
