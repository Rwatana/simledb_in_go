# Chapter 14: Effective Buffer Utilization

This chapter records buffer-allocation helpers and scans used to process relations in bounded chunks. Each helper and
scan contract remains a separate observable behavior so buffer estimates can be tested independently from storage
operations.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/multibuffer.BestRoot`, `BestFactor` | `simpledb/multibuffer/BufferNeeds.java` (`bestRoot`, `bestFactor`) | Choose a ceiling root or quotient of a relation's block count within the usable buffer budget |
| `internal/multibuffer.ChunkScan` | `simpledb/multibuffer/ChunkScan.java` | Own the pinned record pages for one bounded block range; traversal is documented separately |

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
