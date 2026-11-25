# Chapter 10: Planning

Planning turns parsed SQL into a relational query tree. A plan describes both the scan that will execute that tree and
metadata used to compare alternatives before opening a scan. Each concrete plan, scan, estimate, verification rule,
and planner composition remains a separate observable behavior and Issue.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/plan.Plan` | `simpledb/plan/Plan.java` | Describe opening a scan, estimating its cost and output, and exposing its schema |
| `internal/plan.TablePlan` | `simpledb/plan/TablePlan.java` | Retain one table's resolved layout and expose its logical schema |
| `internal/plan.ProductPlan` | `simpledb/plan/ProductPlan.java` | Combine child schemas, open the Cartesian-product scan, and estimate its output |
| `internal/plan.ProjectPlan` | `simpledb/plan/ProjectPlan.java` | Build the projected schema, open its `ProjectScan`, and delegate estimates |
| `internal/plan.QueryPlanner` | `simpledb/plan/QueryPlanner.java` | Convert parsed query data and a transaction into an executable plan |
| `internal/plan.BasicQueryPlanner` | `simpledb/plan/BasicQueryPlanner.java` | Resolve tables and recursively expand views before composing product, selection, and projection plans |
| `internal/plan.BetterQueryPlanner` | `simpledb/plan/BetterQueryPlanner.java` | Choose the lower estimated block-access order as each source joins the left-deep product |
| `internal/plan.UpdatePlanner` | `simpledb/plan/UpdatePlanner.java` | Define separate execution methods for parsed DML and DDL commands |
| `internal/plan.BasicUpdatePlanner` | `simpledb/plan/BasicUpdatePlanner.java` | Execute parsed DML and DDL using metadata and table scans |

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
This follows Java `SelectPlan.open()` while using Go's `(query.Scan, error)` result. Selection estimates remain a
separate behavior; schema delegation is added in Feature 8. The [plan diagram](../diagrams/ch10-plan-contract.svg) records the
`SelectPlan` to `SelectScan` path; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `SelectPlan` and `NewSelectPlan` were undefined. Tests verify matching and
nonmatching rows, the returned scan adapter, child-error wrapping, nil plans, and a child that supplies no scan.

## Feature 8: Expose the selection schema

`SelectPlan.Schema` returns the output schema of its child plan, matching Java `SelectPlan.schema()`. Selection changes
which rows are returned but does not change the fields exposed by the child. A nil selection plan or absent child
returns nil. The [plan diagram](../diagrams/ch10-plan-contract.svg) records that the predicate-filtering node retains
its child's schema; the editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `SelectPlan.Schema` was undefined. Tests verify schema identity and nil-safe results
for a nil receiver and missing child plan.

## Feature 9: Delegate selection block-access estimates

`SelectPlan.BlocksAccessed` forwards the block-access estimate to its child plan, matching Java
`SelectPlan.blocksAccessed()`. A nil selection plan or missing child returns zero. The predicate changes the estimated
row count, not the blocks read by the child scan. The [plan diagram](../diagrams/ch10-plan-contract.svg) now records
the inherited estimate; its editable source remains [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first test run failed because `SelectPlan.BlocksAccessed` was undefined. Tests verify forwarding a nonzero child
estimate and nil-safe zero results.

## Feature 10: Estimate rows returned by a selection

`SelectPlan.RecordsOutput` maps to Java `SelectPlan.recordsOutput()`. It divides the child's estimated output rows by
`Predicate.ReductionFactor(child)`, using the term-level cardinality estimates already exposed by the child plan. An
empty predicate has factor one and preserves the child's row estimate. A nil plan, missing child, or zero factor returns
zero rather than panicking. Select-plan distinct-value estimates remain a separate behavior. The updated
[plan diagram](../diagrams/ch10-plan-contract.svg) shows the row estimate passing through predicate selectivity; its
editable source remains [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `SelectPlan.RecordsOutput` was undefined. Tests verify selective and empty
predicates, nil plans and children, and a zero reduction-factor estimate.

## Feature 11: Estimate distinct values after selection

`SelectPlan.DistinctValues(fieldName)` maps to Java `SelectPlan.distinctValues(String)`. If the predicate equates the
requested field to a constant, the output has one distinct value. If it equates the field to another field, the
selection cannot expose more distinct values than either side, so the estimate is the smaller child estimate.
Otherwise, the child estimate is forwarded unchanged. Nil selections and missing children return zero. This uses the
predicate's explicit presence results so even a zero-value constant is recognized. The updated
[plan diagram](../diagrams/ch10-plan-contract.svg) shows predicate-aware distinct estimates; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `SelectPlan.DistinctValues` was undefined. Tests verify either constant
operand order, a zero-value constant, both requested fields in a field equality, delegation without a matching term,
and nil dependencies.

## Feature 12: Combine product child schemas

`ProductPlan.Schema` maps to the schema construction in Java `ProductPlan(Plan, Plan)` and `schema()`. The product
copies the left child's fields followed by the right child's fields into its own schema, retaining field types and
conceptual lengths. Missing children and nil child schemas contribute no fields; a nil receiver returns nil. Product
scan creation and product-cost estimates remain separate behaviors. The updated [plan diagram](../diagrams/ch10-plan-contract.svg)
shows the product node joining child schemas; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProductPlan` and `NewProductPlan` were undefined. Tests verify ordered field
definitions, stable schema identity, and construction with missing children.

## Feature 13: Open a product scan

`ProductPlan.Open` maps to Java `ProductPlan.open()` and composes the two child plan scans with the existing
`query.ProductScan`. It initializes the product scan before returning it, so callers can advance immediately. Missing
child plans and scans return errors; a right-child open failure closes the already opened left scan, and a failed
product-scan initialization closes both inputs while preserving the primary and cleanup errors. ProductScan itself
owns the left-major traversal and field-read behavior. The [plan diagram](../diagrams/ch10-plan-contract.svg) shows
ProductPlan opening both child scans into the product scan; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProductPlan.Open` was undefined. Tests verify the returned scan type and
first product row, nil plan/scan handling, contextual child errors, and closing opened scans after a later-stage failure.

## Feature 14: Estimate product block accesses

`ProductPlan.BlocksAccessed` maps to Java `ProductPlan.blocksAccessed()` and the nested-loop formula
`B(left) + R(left) * B(right)`: the left input is read once, then the right input is scanned for each left row. It
uses signed 32-bit arithmetic to preserve Java `int` overflow behavior. Missing children and a nil plan report zero.
The [plan diagram](../diagrams/ch10-plan-contract.svg) records this estimate beside the product scan; the editable
source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProductPlan.BlocksAccessed` was undefined. Tests verify the nested-loop
formula, both nil-child cases, a nil receiver, and independent multiplication/addition overflow behavior.

## Feature 15: Estimate product output rows

`ProductPlan.RecordsOutput` maps to Java `ProductPlan.recordsOutput()` and estimates the Cartesian-product row count as
`R(left) * R(right)`. Signed 32-bit multiplication preserves Java `int` overflow. A nil product or missing child returns
zero. The [plan diagram](../diagrams/ch10-plan-contract.svg) records the product row estimate; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProductPlan.RecordsOutput` was undefined. Tests verify the multiplication,
nil receiver and child handling, and signed 32-bit overflow.

## Feature 16: Estimate product distinct values

`ProductPlan.DistinctValues(fieldName)` maps to Java `ProductPlan.distinctValues(String)`. It finds which child schema
contains the field and forwards that child's estimate. When both schemas contain the same name, the left child wins,
matching product-scan field reads. A nil product, missing child/schema, or field absent from both schemas returns zero.
The [plan diagram](../diagrams/ch10-plan-contract.svg) records distinct-value ownership; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProductPlan.DistinctValues` was undefined. Tests verify left and right
delegation, left precedence for duplicate names, absent fields, and nil/missing schemas.

## Feature 17: Build a projection schema

`NewProjectPlan` maps to Java `ProjectPlan(Plan, List<String>)` and copies the requested field definitions from the child
schema in caller-specified order. The projection schema is independent of later child-schema changes. A nil child,
missing child schema, or unknown projected field returns an error rather than panicking; an empty projection is valid.
Opening the child as a `ProjectScan` and forwarding plan estimates remain separate behaviors. The [plan diagram](../diagrams/ch10-plan-contract.svg)
shows ProjectPlan exposing a narrower schema; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProjectPlan` and `NewProjectPlan` were undefined. Tests verify projection
order, copied definitions, schema isolation, empty fields, nil dependencies, and field-specific rejection.

## Feature 18: Open a projected scan

`ProjectPlan.Open` maps to Java `ProjectPlan.open()` and wraps the child scan with `query.ProjectScan` using the
projection schema's ordered fields. It preserves the child plan's before-first position. A nil project plan, missing
child plan, child-open error, or nil child scan returns a contextual error. The [plan diagram](../diagrams/ch10-plan-contract.svg)
shows ProjectPlan opening the projection scan; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProjectPlan.Open` was undefined. Tests verify the scan type, projected field
visibility, first child row, value delegation, close delegation, and child error/nil handling.

## Feature 19: Delegate projected-plan block estimates

`ProjectPlan.BlocksAccessed` maps to Java `ProjectPlan.blocksAccessed()` and forwards the child's estimate unchanged;
projection does not alter the number of blocks the child must read. A nil receiver or missing child returns zero, matching
the other nil-safe Go plan estimates. The [plan diagram](../diagrams/ch10-plan-contract.svg) records this delegation; its
editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProjectPlan.BlocksAccessed` was undefined. Tests verify a nonzero child estimate
and nil receiver/missing-child behavior.

## Feature 20: Delegate projected-plan output estimates

`ProjectPlan.RecordsOutput` maps to Java `ProjectPlan.recordsOutput()` and forwards the child's estimated row count
unchanged; removing columns does not remove rows. A nil receiver or missing child returns zero, matching the other
nil-safe Go plan estimates. The [plan diagram](../diagrams/ch10-plan-contract.svg) records this delegation; its editable
source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProjectPlan.RecordsOutput` was undefined. Tests verify a nonzero child row
estimate and nil receiver/missing-child behavior.

## Feature 21: Delegate projected-plan distinct-value estimates

`ProjectPlan.DistinctValues(fieldName)` maps to Java `ProjectPlan.distinctValues(String)` and forwards the requested field
to the child plan unchanged. A nil receiver or missing child returns zero, matching the other nil-safe Go estimates. The
[plan diagram](../diagrams/ch10-plan-contract.svg) records this delegation; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `ProjectPlan.DistinctValues` was undefined. Tests verify forwarding the
projected field name and a nonzero child estimate, plus nil receiver/missing-child behavior.

## Feature 22: Define the query-planner contract

`QueryPlanner` maps to Java `simpledb.plan.QueryPlanner`. Its `CreatePlan` method accepts `parse.QueryData` and the
transaction boundary needed by planning, then returns a `Plan`. Go also returns an error so metadata and scan failures can
be handled without Java exceptions. The [plan diagram](../diagrams/ch10-plan-contract.svg) shows the planner turning
parsed query data into a plan tree; its editable source is
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `QueryPlanner` was undefined. A compile-time conformance assertion verifies
that an implementation with the declared input and result types satisfies the interface.

## Feature 23: Plan SELECT queries over physical tables

`BasicQueryPlanner.CreatePlan` maps to Java `BasicQueryPlanner.createPlan` for physical table sources. It resolves each
table's layout and statistics in query order, combines multiple tables left-associatively with `ProductPlan`, then wraps
the result in `SelectPlan` followed by `ProjectPlan`. Nil dependencies, metadata failures, a query without tables, and
unknown projected fields return contextual errors. The [plan diagram](../diagrams/ch10-plan-contract.svg) shows this
operator order; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicQueryPlanner` and `NewBasicQueryPlanner` were undefined. Tests verify
two-table order, metadata lookup order, the product/selection/projection tree, predicate and output-field preservation,
and nil/error cases.

## Feature 24: Expand views in basic query plans

`BasicQueryPlanner` checks metadata for each source view before treating it as a physical table. A found definition is
parsed as `QueryData` and planned recursively with the same transaction, preserving its original position in a product.
View lookup and parse errors keep their causes and include the view name; recursive view cycles return an error instead
of recursing indefinitely. The [plan diagram](../diagrams/ch10-plan-contract.svg) shows views expanding into the same
plan tree; its editable source is [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed: stored views were sent to physical table layout lookup. Tests verify nested view
expansion, view/table ordering, multiple-view products, contextual metadata/parse errors, and cycle detection.

## Feature 25: Choose lower-cost orders for basic joins

`BetterQueryPlanner` reuses `BasicQueryPlanner` source resolution, including recursive views, then chooses the child order
with the lower `ProductPlan.BlocksAccessed` estimate as each source joins the accumulated left-deep product. It reverses
the order only when that candidate is strictly cheaper, retaining query source order on ties. Selection and projection
remain above the product tree. The [plan diagram](../diagrams/ch10-plan-contract.svg) shows the cost-aware planner next
to the basic planner; its editable source remains [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `NewBetterQueryPlanner` and `BetterQueryPlanner` were undefined. Tests verify
both cost choices, stable order on ties, left-deep products for multiple sources, final selection/projection placement,
and invalid planner inputs.

## Feature 26: Define the update-planner contract

`UpdatePlanner` maps to Java `simpledb.plan.UpdatePlanner` and defines one method for each parsed INSERT, DELETE, UPDATE,
CREATE TABLE, CREATE VIEW, and CREATE INDEX command. Each method receives the transaction boundary needed by scans and
metadata and returns an affected-row count plus an explicit error. This feature establishes the contract only; command
execution stays in separate `BasicUpdatePlanner` behaviors. The [planning diagram](../diagrams/ch10-plan-contract.svg)
shows update commands on their own contract; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `UpdatePlanner` was undefined. A compile-time probe implements all six
command methods, and focused calls verify the parsed input types and `(affected rows, error)` result shape.

## Feature 27: Execute DELETE with the basic update planner

`BasicUpdatePlanner.ExecuteDelete` resolves the target layout, opens a `TablePlan` under a `SelectPlan`, deletes each
matching row through `query.UpdateScan`, and returns the deleted count. It closes the scan on success and on iteration,
delete, or scan-type failures, joining close errors with the original cause. The [planning diagram](../diagrams/ch10-plan-contract.svg)
shows DELETE execution beneath the separate update-planner contract; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner` and `NewBasicUpdatePlanner` were undefined. Tests verify
matching-row deletion, untouched nonmatching rows, affected counts, scan cleanup, contextual metadata/open failures,
and combined iteration/delete/close errors.

## Feature 28: Execute UPDATE with the basic update planner

`BasicUpdatePlanner.ExecuteModify` resolves the target layout, opens a `TablePlan` beneath a `SelectPlan`, evaluates
the parsed replacement expression against every matching `query.UpdateScan`, writes the resulting constant to the target
field, and returns the number of successful writes. It closes the scan on success and on iteration, expression, or write
errors, joining any close failure with the original cause. An empty target field is rejected before metadata access; a
missing field is reported by the scan's schema-aware write. The [planning diagram](../diagrams/ch10-plan-contract.svg)
shows UPDATE alongside DELETE under the distinct update-planner contract; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner.ExecuteModify` was undefined. Tests cover matching, unmatched,
and empty-predicate rows, expression evaluation, target writes, nil dependencies, metadata and scan-open failures, and
preservation of iteration/evaluation/write errors when scan cleanup also fails.

## Feature 29: Execute INSERT with the basic update planner

`BasicUpdatePlanner.ExecuteInsert` opens the target `TablePlan`, inserts one row through `query.UpdateScan`, then writes
the parsed field/value pairs in their original order. It reports one affected row after all writes succeed and closes
the scan on success, insert/type/write failures, or unsupported-scan errors, joining cleanup errors with the primary
cause. Empty table names, empty field names, and mismatched or empty field/value lists are rejected before a row is
opened. The [planning diagram](../diagrams/ch10-plan-contract.svg) shows INSERT alongside the earlier DELETE and UPDATE
behaviors; its editable source remains [`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner.ExecuteInsert` was undefined. Tests verify one inserted row
and its field value, invalid command data, nil dependencies, metadata/layout and scan-open errors, unsupported and nil
scans, insert/value-write failures, and close-error preservation.

## Feature 30: Execute CREATE TABLE with the basic update planner

`BasicUpdatePlanner.ExecuteCreateTable` passes the parsed table name, an independent copy of its schema, and the same
transaction to the metadata manager. It returns zero affected rows because CREATE TABLE does not modify table records.
Nil planner/metadata/transaction dependencies, an empty table name, and metadata errors return contextual errors while
preserving the original metadata cause. The [planning diagram](../diagrams/ch10-plan-contract.svg) now shows CREATE TABLE
as a metadata-backed operation alongside the row-oriented update commands; the editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner.ExecuteCreateTable` was undefined. Tests verify metadata
delegation, field order/type/length, transaction identity, zero affected rows, nil dependencies, empty names, and wrapped
metadata errors.

## Feature 31: Execute CREATE VIEW with the basic update planner

`BasicUpdatePlanner.ExecuteCreateView` sends the parsed view name, rendered SELECT definition, and same transaction to
the metadata manager. It returns zero affected rows because creating a view does not modify table records. Nil
planner/metadata/transaction dependencies, an empty view name, and an unset query definition return contextual errors;
metadata failures keep their original cause. The [planning diagram](../diagrams/ch10-plan-contract.svg) now shows
CREATE VIEW as metadata-backed DDL alongside CREATE TABLE and the row-oriented commands; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner.ExecuteCreateView` was undefined. Tests verify the rendered
SELECT text and original transaction reach metadata, successful DDL returns zero affected rows, and nil dependencies,
empty names, unset definitions, and wrapped metadata failures are handled.

## Feature 32: Execute CREATE INDEX with the basic update planner

`BasicUpdatePlanner.ExecuteCreateIndex` passes the parsed index name, table name, field name, and same transaction to
the metadata manager. It returns zero affected rows because index creation does not change table records. Nil
planner/metadata/transaction dependencies and empty index, table, or field names return contextual errors, while
metadata failures retain their original cause. The [planning diagram](../diagrams/ch10-plan-contract.svg) now includes
CREATE INDEX as metadata-backed DDL; its editable source remains
[`ch10-plan-contract.drawio`](../diagrams/ch10-plan-contract.drawio).

### Test-first record

The first focused test run failed because `BasicUpdatePlanner.ExecuteCreateIndex` was undefined. Tests verify each parsed
name and the original transaction reach metadata, successful DDL returns zero affected rows, and nil dependencies, each
empty name, and wrapped metadata errors are handled.

## Intentional differences from Java

- `Plan.Open` returns `(query.Scan, error)` rather than only a scan so Go callers can handle storage failures without exceptions or panics.
- `NewTablePlan` receives a resolved layout rather than a metadata manager. This keeps metadata lookup outside the leaf plan and makes the table plan's schema dependency explicit.
- `TablePlan.Open` returns a query-layer `RecordScan` adapter, rather than exposing the record-layer `TableScan`, because query plans require `GetVal`.
- `NewTablePlan` receives precomputed `metadata.StatInfo` so obtaining statistics remains a separate metadata behavior; `BlocksAccessed` only forwards the estimate.
- `TablePlan.DistinctValues` forwards the current simple `StatInfo` estimate; field-specific statistics remain outside this table-plan behavior.
- `SelectPlan.Schema` returns the child schema directly because filtering does not change the set of output fields.
- `SelectPlan.BlocksAccessed` delegates directly to the child; estimates affected by selectivity remain distinct methods.
- `ProductPlan.Open` initializes both child scans before returning and closes partial scan resources when composition fails.
- `ProductPlan.BlocksAccessed` explicitly retains Java's signed 32-bit arithmetic instead of the host-sized Go `int` calculation.
- `ProductPlan.RecordsOutput` uses Java-compatible signed 32-bit multiplication rather than host-sized arithmetic.
- `ProductPlan.DistinctValues` resolves by the child's schema and gives duplicate field names to the left child.
- `NewProjectPlan` reports missing schema/field errors because Go callers must handle failures explicitly rather than receive Java exceptions.
- `ProjectPlan.Open` keeps the child's before-first position rather than resetting it a second time.
- `ProjectPlan.BlocksAccessed` delegates directly to its child because projection does not change child block access.
- `ProjectPlan.RecordsOutput` delegates directly to its child because projection does not change row count.
- `ProjectPlan.DistinctValues` delegates field estimates to the child, preserving the child's statistics model.
- `QueryPlanner.CreatePlan` returns `(Plan, error)` so Go planning failures remain explicit.
- `BasicQueryPlanner` first added physical-table planning; recursive view expansion is recorded as its own follow-up behavior.
- `BasicQueryPlanner` detects recursive view cycles and returns an error rather than allowing unbounded recursion.
- `BetterQueryPlanner` uses block-access estimates to choose each product's child order, reversing only when strictly cheaper.
- `BasicUpdatePlanner.ExecuteDelete` returns an error with its affected count and closes the scan on all operation paths.
- `BasicUpdatePlanner.ExecuteModify` evaluates replacement expressions per matching row and reports the number of successful writes.
- `BasicUpdatePlanner.ExecuteInsert` returns a successful count only after inserting one row and writing all parsed field values.
- `BasicUpdatePlanner.ExecuteCreateTable` delegates schema creation to metadata and returns zero affected rows because DDL does not change table records.
- `BasicUpdatePlanner.ExecuteCreateView` delegates the rendered SELECT definition to metadata and returns zero affected rows because DDL does not change table records.
- `BasicUpdatePlanner.ExecuteCreateIndex` delegates all parsed identifiers to metadata and returns zero affected rows because DDL does not change table records.

## Completion audit

The 32 numbered feature records map the plan contract, table/select/product/project plans and their estimates, basic
and cost-based query-planner composition, and the update-planner DML/DDL commands to their focused tests. View
expansion, query-level heuristic ordering, and multibuffer product construction are documented separately in their
dependent chapters; the Chapter 10 planners keep metadata and transaction access explicit.

The plan-contract diagram already links the plan tree, estimates, schema, and opened scan to its editable Draw.io
source. The documented Go boundaries retain explicit error results, cleanup of partial scans, cycle detection, and
Java-compatible signed 32-bit estimate arithmetic. Chapter 10 is complete on the ordered PR stack; the README status
does not imply that the stack has already been merged.
