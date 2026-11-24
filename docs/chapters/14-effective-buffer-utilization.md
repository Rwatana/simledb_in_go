# Chapter 14: Effective Buffer Utilization

This chapter records buffer-allocation helpers and scans used to process relations in bounded chunks. Each helper and
scan contract remains a separate observable behavior so buffer estimates can be tested independently from storage
operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/multibuffer.BestRoot` | `simpledb/multibuffer/BufferNeeds.java` (`bestRoot`) | Choose a ceiling root of a relation's block count within the usable buffer budget |

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
or buffer-manager operations. `BestFactor` and chunk scanning remain separate behaviors.

### Validation

The focused package tests cover every executable statement and function. The module coverage workflow continues to
require 100.0% statement coverage for every executable package, function, and the module total.
