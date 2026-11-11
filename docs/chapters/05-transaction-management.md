# Chapter 5: Transaction Management

Chapter 5 adds recovery and concurrency control above the file, log, and buffer layers. The Go implementation follows the
book's order: recovery records and undo-only recovery first, then locking and the transaction use case. Each observable
behavior remains a separate stacked pull request.

## Feature 1: Quiescent checkpoint log record

The editable [Draw.io diagram](../diagrams/ch05-checkpoint-record.drawio) and rendered
[SVG](../diagrams/ch05-checkpoint-record.svg) show the entire record layout. A quiescent checkpoint has one value only: a
four-byte operation code whose value is zero. It has no transaction ID and no undo payload.

```text
offset 0                                      offset 4
+---------------------------------------------+
| CHECKPOINT operation = 0 (big-endian int32) |
+---------------------------------------------+
```

### Public API

| API | Responsibility |
| --- | --- |
| `OperationCheckpoint` | Identify the book-compatible checkpoint operation code `0` |
| `EncodeCheckpointRecord` | Produce the exact four-byte disk representation |
| `DecodeCheckpointRecord` | Validate and interpret one exact checkpoint representation |
| `CheckpointRecord.Operation` | Report that the decoded record is a checkpoint |

The decoder requires exactly four bytes. Truncation and trailing bytes are errors rather than silently accepting an
ambiguous record boundary; an operation other than checkpoint is also rejected with its numeric value in the error.

### Test-first record

The round-trip and invalid-encoding tests were written first. Before the implementation existed, the package failed to
build with undefined `EncodeCheckpointRecord`, `DecodeCheckpointRecord`, and `OperationCheckpoint` symbols. The smallest
implementation then added only the checkpoint operation, exact encoding, and strict decoder.

| Test | Observable behavior |
| --- | --- |
| `TestCheckpointRecordRoundTrip` | Exact bytes and decoded checkpoint operation |
| `TestDecodeCheckpointRecordRejectsInvalidEncoding` | Truncation, trailing bytes, and another operation are rejected |

## Java source reading

The relevant references are `simpledb.tx.recovery.LogRecord` and `CheckpointRecord`.

| Java | Go | Decision |
| --- | --- | --- |
| `CHECKPOINT = 0` | `OperationCheckpoint Operation = 0` | Preserve the on-disk code with a typed value |
| `writeToLog` builds a four-byte `Page` | `EncodeCheckpointRecord` returns four bytes | Keep encoding independent of log append for this behavior |
| `txNumber()` returns dummy `-1` | No transaction accessor | Absence is clearer than manufacturing a transaction ID |
| empty `undo` method | No undo API yet | Do not introduce transaction orchestration before an update record requires it |
| factory accepts a byte array | strict checkpoint decoder | Record dispatch belongs to a later PR when another record kind exists |

## Clean Architecture decision

The recovery package owns the durable meaning of recovery record bytes. It does not import the log manager, buffer
manager, filesystem adapter, or transaction layer. Appending the bytes and deciding when to flush them are later use-case
behaviors, not part of the checkpoint value itself.

## Intentionally excluded

- Appending or flushing through `log.Manager`
- START, COMMIT, ROLLBACK, SETINT, and SETSTRING records
- Transaction IDs and undo behavior
- Generic record dispatch
- Rollback and restart-recovery scans

## Feature 2: Transaction START log record

The same [editable diagram](../diagrams/ch05-checkpoint-record.drawio) and
[rendered SVG](../diagrams/ch05-checkpoint-record.svg) now compare the two record prefixes. START extends the four-byte
operation with one four-byte transaction ID:

```text
offset 0               offset 4                 offset 8
+----------------------+------------------------+
| START operation = 1  | transaction ID (int32) |
+----------------------+------------------------+
```

`EncodeStartRecord` preserves the complete signed int32 bit pattern. `DecodeStartRecord` requires exactly eight bytes,
checks the operation before exposing the ID, and reports malformed length or a different operation as contextual errors.
The codec deliberately does not decide whether a transaction ID is valid; allocation and lifecycle policy belong to the
future transaction use case.

### Test-first record

Positive, zero, negative-bit-pattern, and malformed encodings were tested before implementation. The first run failed to
build because `TransactionID`, `OperationStart`, `EncodeStartRecord`, and `DecodeStartRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestStartRecordRoundTrip` | Exact eight-byte encoding and signed int32 round-trip |
| `TestDecodeStartRecordRejectsInvalidEncoding` | Truncation, trailing bytes, and another operation are rejected |

### Java source map and Go decision

| Java `StartRecord` | Go | Decision |
| --- | --- | --- |
| `START = 1` | `OperationStart Operation = 1` | Preserve the durable operator value |
| two `Integer.BYTES` fields | exact eight-byte slice | Make the record boundary explicit |
| constructor extracts `txnum` | strict decoder returns `StartRecord` | Validate size and operation before the ID |
| `writeToLog(lm, txnum)` | codec only | Append is the next independent behavior |
| empty `undo` | no undo method yet | Avoid an interface before update records require it |

## Feature 3: Append a START record

`AppendStartRecord` composes Feature 2's exact codec with the existing log append contract. Its `LogAppender` port contains
only `Append([]byte) (log.LSN, error)`, so `log.Manager` satisfies it directly and recovery tests can use an in-memory
recorder without filesystem I/O.

```text
TransactionID
    -> EncodeStartRecord                 (exact 8 bytes)
    -> LogAppender.Append                (exactly once)
    -> return the same LSN or wrapped error
```

The function does not flush the LSN. A START record only establishes rollback history; the later commit and rollback
behaviors are responsible for their own durability boundaries. On append failure, the error includes the transaction ID
and preserves the underlying error for `errors.Is`; it does not retry because the caller cannot know whether a failed I/O
partially entered an external implementation.

### Test-first record

The exact appended bytes, unchanged LSN, single-call failure path, and concrete `log.Manager` conformance were tested
first. The initial build failed because `AppendStartRecord` and `LogAppender` were undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendStartRecordAppendsExactEncodingAndReturnsLSN` | One exact append and unchanged LSN |
| `TestAppendStartRecordReturnsAppendFailureWithContext` | Wrapped transaction context and no retry |
| `TestLogManagerSatisfiesStartRecordAppender` | Existing concrete manager satisfies the consumer-owned port |

### Java source map and Go decision

| Java `StartRecord.writeToLog` | Go | Decision |
| --- | --- | --- |
| accepts concrete `LogMgr` | consumes `LogAppender` | Declare the minimum port at the recovery consumer |
| builds two integers inline | reuses `EncodeStartRecord` | Keep byte layout independently testable |
| returns `lm.append(rec)` | returns the same `log.LSN` | Preserve the log manager's ordering token |
| unchecked append failure | wrapped Go error | Expose failure and transaction context explicitly |

## Feature 4: Transaction COMMIT log record codec

COMMIT uses the same fixed-width shape as START, but its durable operation code is `2`. `EncodeCommitRecord` writes the
operation followed by the signed four-byte transaction bit pattern; `DecodeCommitRecord` requires exactly eight bytes and
checks the operation before returning a `CommitRecord`.

```text
offset 0                offset 4                 offset 8
+-----------------------+------------------------+
| COMMIT operation = 2  | transaction ID (int32) |
+-----------------------+------------------------+
```

This PR factors the already-tested START representation into private `encodeTransactionRecord` and
`decodeTransactionRecord` helpers. The public START API and its error text remain unchanged. The shared code expresses only
the common two-integer layout; it does not introduce a generic public record abstraction before record dispatch is needed.

### Test-first record

Exact positive, zero, and negative-bit-pattern encodings plus malformed inputs were tested first. The initial build failed
because `OperationCommit`, `EncodeCommitRecord`, and `DecodeCommitRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestCommitRecordRoundTrip` | Exact COMMIT bytes and signed transaction ID round-trip |
| `TestDecodeCommitRecordRejectsInvalidEncoding` | Truncation, trailing bytes, and a START operation are rejected |

### Java source map and Go decision

| Java `CommitRecord` | Go | Decision |
| --- | --- | --- |
| `COMMIT = 2` | `OperationCommit Operation = 2` | Preserve the durable operator value |
| two `Integer.BYTES` values | shared private fixed-width codec | Reuse the representation without merging record types |
| constructor extracts `txnum` | strict `DecodeCommitRecord` | Validate boundary and operation first |
| `writeToLog` | excluded | Append and flush are later observable behaviors |
| empty `undo` | no undo method yet | Do not add orchestration to a value codec |

## Feature 5: Append a COMMIT record

`AppendCommitRecord` sends Feature 4's exact bytes through the same recovery-owned `LogAppender` port as START. It returns
the append LSN unchanged. Failure is wrapped with the COMMIT transaction ID, preserves the underlying error, and is not
retried.

The shared private append helper contains only the one-call/error-context sequence. START still selects its own codec and
name, while COMMIT selects its own; this avoids a public generic record writer and keeps each operation independently
reviewable.

### Test-first record

Exact bytes, unchanged LSN, and the no-retry error path were tested before implementation. The first run failed because
`AppendCommitRecord` was undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendCommitRecordAppendsExactEncodingAndReturnsLSN` | One exact COMMIT append and unchanged LSN |
| `TestAppendCommitRecordReturnsAppendFailureWithContext` | Wrapped COMMIT transaction context and no retry |

### Java source map and Go decision

| Java `CommitRecord.writeToLog` | Go | Decision |
| --- | --- | --- |
| concrete `LogMgr` parameter | existing `LogAppender` port | Reuse the consumer-owned append boundary |
| builds bytes and appends together | codec then append | Keep layout tests separate from I/O composition |
| returns append LSN | unchanged `log.LSN` | Preserve ordering identity for the next flush behavior |
| append has no checked error | wrapped Go error | Make partial I/O uncertainty explicit and avoid retry |

## Next behavior

Continue Sect. 5.3.9.1 by flushing an appended COMMIT LSN in a separate PR because append and durability are distinct
observable behaviors.

## Sources

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 5, Sect. 5.3.9.1
- SimpleDB 3.4 Java source: `simpledb.tx.recovery.LogRecord`, `CheckpointRecord`
