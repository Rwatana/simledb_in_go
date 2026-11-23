# Chapter 10: Planning

Planning turns parsed SQL into a relational query tree. A plan describes both the scan that will execute that tree and
metadata used to compare alternatives before opening a scan. Each concrete plan, estimate, verification rule, and
planner composition remains a separate observable behavior and Issue.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/plan.Plan` | `simpledb/plan/Plan.java` | Describe opening a scan, estimating its cost and output, and exposing its schema |
| `internal/plan.TablePlan` | `simpledb/plan/TablePlan.java` | Retain one table's resolved layout and expose its logical schema |

## Feature 1: Define the query plan contract

`plan.Plan` maps to Java `simpledb.plan.Plan`. It provides `Open` for the executable `query.Scan`, two cardinality/cost
estimates, a distinct-value estimate for one field, and the result `record.Schema`. Go returns an error from `Open` so
storage and scan-construction failures can flow through the same explicit error path used by the rest of the Go query
package. This first feature defines the shared contract only; concrete table and relational-operator plans are separate
features. The [plan-contract diagram](../diagrams/ch10-plan-contract.svg) shows the plan as the boundary between a
query tree, its estimates/schema, and the scan it opens; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `internal/plan.Plan` did not exist. A compile-time test double exercises each method
signature, including scan/error return values, the three estimates, and schema access.

## Feature 2: Add table plan schema

`TablePlan` is the concrete leaf for a single table. This feature retains the transaction, table name, and already
resolved `record.Layout`, then implements the `Plan.Schema` behavior by returning the layout's logical schema. A nil
plan or a plan without a layout safely returns nil. Scan creation and the statistics-backed estimates are deliberately
separate features so each observable behavior can be reviewed and covered independently. The updated
[plan diagram](../diagrams/ch10-plan-contract.svg) shows `TablePlan` supplying the schema from its layout; the editable
source remains [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed to compile because `NewTablePlan` and `TablePlan` were undefined. Tests then verify that the
schema exposed by the table plan is the layout's schema and that nil plans/layouts return nil.

## Feature 3: Open a table scan

`TablePlan.Open` creates the executable scan for the table. The Go implementation uses `query.NewRecordScan`, which
wraps the record-layer `TableScan` with the `GetVal` operation required by `query.Scan`; this preserves the package
dependency direction. The returned scan starts before its first row. Initialization errors retain table-plan context,
and nil plan dependencies return errors rather than panicking. Estimate methods remain separate features. The updated
[plan diagram](../diagrams/ch10-plan-contract.svg) shows the table plan opening the query-layer adapter; its editable
source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `TablePlan.Open` did not exist. After adding it, a compile-time check showed that the
record-layer `TableScan` did not satisfy `query.Scan` because it lacks `GetVal`; the tests now assert that Open returns
a `query.RecordScan`, starts at the first occupied slot, wraps initialization errors, and rejects nil dependencies.

## Feature 4: Estimate table plan block accesses

`TablePlan.BlocksAccessed` delegates to the retained `metadata.StatInfo` so the optimizer can compare the cost of
scanning this table against other plans. A missing statistic or nil receiver reports zero without panicking. The plan
constructor accepts the already-computed statistics; lookup and refresh stay in metadata management rather than being
hidden inside the estimate. The updated [plan diagram](../diagrams/ch10-plan-contract.svg) links the block estimate to
`StatInfo`; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `NewTablePlan` did not retain a statistics argument and `BlocksAccessed` was
undefined. Tests verify forwarding a nonzero block estimate and nil-safe zero estimates.

## Feature 5: Estimate table output rows

`TablePlan.RecordsOutput` delegates the expected table cardinality to its retained `metadata.StatInfo`, matching Java
`TablePlan.recordsOutput()`. This keeps the estimate tied to the same statistics snapshot used for block-access
estimation. A nil plan or missing statistics returns zero. Distinct-value estimation remains a separate behavior. The
[plan diagram](../diagrams/ch10-plan-contract.svg) now shows the table plan forwarding both block and output-row
estimates; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `TablePlan.RecordsOutput` was undefined. Tests verify a nonzero forwarded row estimate
and nil-safe zero results.

## Feature 6: Estimate table distinct values

`TablePlan.DistinctValues` forwards a field-name estimate to the retained `metadata.StatInfo`, matching Java
`TablePlan.distinctValues(fieldName)`. A nil plan or missing statistics returns zero without panicking. The current
`StatInfo` uses its documented simple estimate for any field; choosing or refreshing the statistics remains a metadata
behavior. The [plan diagram](../diagrams/ch10-plan-contract.svg) now connects this estimate to `StatInfo`; its editable
source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `TablePlan.DistinctValues` was undefined. Tests cover the current nonzero statistic
estimate and nil-safe zero results.

## Feature 7: Open a predicate-filtered scan

`SelectPlan.Open` opens its child plan and wraps that scan in the existing `query.SelectScan`, which evaluates the
stored `query.Predicate` while advancing. The constructor retains the child plan and predicate. Child-open errors keep
their cause and gain selection-plan context; missing plans and a child that returns no scan produce explicit errors.
This follows Java `SelectPlan.open()` while using Go's `(query.Scan, error)` result. Selection estimates and schema
delegation remain separate behaviors. The [plan diagram](../diagrams/ch10-plan-contract.svg) now records the
`SelectPlan` to `SelectScan` path; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `SelectPlan` and `NewSelectPlan` were undefined. Tests verify matching and
nonmatching rows, the returned scan adapter, child-error wrapping, nil plans, and a child that supplies no scan.

## Intentional differences from Java

- `Plan.Open` returns `(query.Scan, error)` rather than only a scan so Go callers can handle storage failures without exceptions or panics.
- `NewTablePlan` receives a resolved layout rather than a metadata manager. This keeps metadata lookup outside the leaf plan and makes the table plan's schema dependency explicit.
- `TablePlan.Open` returns a query-layer `RecordScan` adapter, rather than exposing the record-layer `TableScan`, because query plans require `GetVal`.
- `NewTablePlan` receives precomputed `metadata.StatInfo` so obtaining statistics remains a separate metadata behavior; `BlocksAccessed` only forwards the estimate.
- `TablePlan.DistinctValues` forwards the current simple `StatInfo` estimate; field-specific statistics remain outside this table-plan behavior.
