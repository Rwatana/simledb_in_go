# Chapter 15: Query Optimization

This chapter records the product-order choice for the `OptimizedProductPlan` source. Its estimate inputs and selected
plan are independently tested in the Chapter 15 feature below, which is represented in the ordered PR stack.

## Java source map

| Go behavior | Java source | Responsibility |
| --- | --- | --- |
| `internal/plan.NewOptimizedProductPlan` | `simpledb/plan/OptimizedProductPlan.java` (constructor) | Compare both two-input product orientations and select the cheaper block-access order |

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
