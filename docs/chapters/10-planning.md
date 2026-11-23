# Chapter 10: Planning

Planning turns parsed SQL into a relational query tree. A plan describes both the scan that will execute that tree and
metadata used to compare alternatives before opening a scan. Each concrete plan, estimate, verification rule, and
planner composition remains a separate observable behavior and Issue.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/plan.Plan` | `simpledb/plan/Plan.java` | Describe opening a scan, estimating its cost and output, and exposing its schema |

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

## Intentional differences from Java

- `Plan.Open` returns `(query.Scan, error)` rather than only a scan so Go callers can handle storage failures without exceptions or panics.
