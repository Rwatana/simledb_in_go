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

## Next behavior

Continue Sect. 5.3.9.1 with the START record's transaction ID encoding and decoding as a separate issue and stacked PR.

## Sources

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 5, Sect. 5.3.9.1
- SimpleDB 3.4 Java source: `simpledb.tx.recovery.LogRecord`, `CheckpointRecord`
