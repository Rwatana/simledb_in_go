# Chapter 15: Query Optimization

Chapter 15 is still in progress. The current features record the product-order choice for `OptimizedProductPlan` and
the metadata state needed by the book's per-table heuristic planner. Its later selection, join, product, and full
query-planner behaviors remain separate features.

## Java source map

| Go behavior | Java source | Responsibility |
| --- | --- | --- |
| `internal/plan.NewOptimizedProductPlan` | `simpledb/plan/OptimizedProductPlan.java` (constructor) | Compare both two-input product orientations and select the cheaper block-access order |
| `internal/plan.NewTablePlanner` | `simpledb/opt/TablePlanner.java` (constructor) | Resolve and retain a table's plan, schema, predicate, transaction, and indexes for later heuristic planning |
| `internal/plan.TablePlanner.MakeSelectPlan` | `simpledb/opt/TablePlanner.java` (`makeSelectPlan`, `makeIndexSelect`) | Use a matching field index for a constant equality, or return the table plan |
| `internal/plan.TablePlanner.MakeSelectPlan` | `simpledb/opt/TablePlanner.java` (`addSelectPred`) | Apply the query terms that can be evaluated against one table schema |

## Feature 1: Choose the cheaper product plan order

`plan.NewOptimizedProductPlan(left, right)` constructs both `ProductPlan(left, right)` and
`ProductPlan(right, left)`, then returns the orientation with the lower `BlocksAccessed` estimate. The original
orientation is retained only when it is strictly cheaper; equal estimates choose the reversed orientation to match
Java's constructor. The factory returns the selected `ProductPlan` directly, avoiding a wrapper that only forwards
the existing `Plan` contract. It does not open either plan or modify either input.

### Test-first record

Focused compilation first failed because `NewOptimizedProductPlan` was undefined. Tests cover a cheaper original order,
a cheaper reversed order, equal-cost reversed tie-breaking, the selected block estimate, and the absence of scan opens.

### Design decision

The implementation reuses `ProductPlan` and its Java-compatible block-access estimate rather than copying the plan
interface into a wrapper. Query-planner integration is a separate behavior and remains unchanged here.

### Validation

Focused tests cover every statement in the factory. Full-module coverage, race, vet, documentation, formatting, and
independent workflow-audit results are recorded in the recovery checkpoint and stacked PR.

## Feature 2: Initialize one table planner's metadata

`plan.NewTablePlanner` resolves one physical table's layout, validates its schema, resolves statistics, then loads
the table's index metadata. It retains a `TablePlan`, schema, predicate, transaction, and index map so later planner
operations can make independent selection, join, and product choices. Each metadata request happens once and in this
order. Missing metadata or transaction dependencies and missing layout/schema/statistics fail before a planner is
returned; lookup failures preserve their causes with table and phase context. A missing index map is treated as an
empty set of indexes.

The Go `TablePlannerMetadata` interface exposes only `Layout`, `Statistics`, and `IndexInfo`, the operations this
constructor needs. It deliberately does not require view resolution, scan creation, or query-level orchestration.
This constructor does not open a table scan or inspect rows. Selection-plan choice and index use remain separate
observable behaviors.

### Test-first record

Focused compilation first failed because `NewTablePlanner` was undefined. Tests verify dependency validation,
metadata request order and arguments, state retention, empty index metadata, contextual wrapped failures, and the
absence of storage access.

### Validation

Focused planner tests cover every statement in the constructor and its dependency check. Full-module coverage, race,
vet, documentation, formatting, and the independent workflow-audit results are recorded in the recovery checkpoint
and stacked PR.

## Feature 3: Use an index for a table equality selection

`TablePlanner.MakeSelectPlan` checks the table's registered indexes for a field that the query predicate equates with
a constant. A match produces an `IndexSelectPlan` over the retained `TablePlan`, carrying the matching index and
search key. If no registered index has such an equality, the method returns the table plan. This plan choice does not
open scans or access storage. Applying the remaining table-local predicate terms is a separate behavior.

### Test-first record

Focused compilation first failed because `MakeSelectPlan` was undefined. Tests cover matching indexes, a constant on
either side of the equality, unmatched and field-to-field predicates, nil index metadata, and no storage access.

### Design decision

The method returns the existing `Plan` contract, using `IndexSelectPlan` only for an indexed equality and the retained
`TablePlan` as its fallback. It leaves residual filtering and query-level join/product choices to later planner
features.

### Validation

Focused planner tests cover every statement in the method. Full-module coverage, race, vet, documentation, formatting,
and independent workflow-audit results are recorded in the recovery checkpoint and stacked PR.

## Feature 4: Apply table-local selection predicates

After choosing its table or index-selection candidate, `TablePlanner.MakeSelectPlan` wraps that candidate in a
`SelectPlan` when the query predicate contains terms applicable to the table schema. The selection contains every
table-local term, including an indexed equality, matching the Java planner's `addSelectPred` behavior. Terms that need
fields from another table remain outside this plan for join planning. With no applicable terms, the candidate is returned
unchanged. Plan construction performs no scan or storage I/O.

### Test-first record

Focused compilation first failed because a table-local predicate was not applied. Tests cover filtering the index
candidate, retaining all local terms while excluding a cross-table term, filtering the table-scan fallback, returning an
unfiltered candidate when no terms apply, and avoiding storage access.

### Design decision

The implementation reuses `Predicate.SelectSubPredicate` and the existing `SelectPlan`; it does not duplicate predicate
partitioning or add another scan wrapper type. Index lookup selection remains covered by the preceding feature.

### Validation

Focused planner tests cover every statement in the selection method. Full-module coverage, race, vet, documentation,
formatting, and independent workflow-audit results are recorded in the recovery checkpoint and stacked PR.
