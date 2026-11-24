# Chapter 14: Effective Buffer Utilization

This chapter records buffer-allocation helpers and scans used to process relations in bounded chunks. Each helper and
scan contract remains a separate observable behavior so buffer estimates can be tested independently from storage
operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/multibuffer.BestRoot`, `BestFactor` | `simpledb/multibuffer/BufferNeeds.java` (`bestRoot`, `bestFactor`) | Choose a ceiling root or quotient of a relation's block count within the usable buffer budget |

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
