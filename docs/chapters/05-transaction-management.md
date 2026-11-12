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

## Feature 6: Flush the appended COMMIT LSN

The [editable Draw.io diagram](../diagrams/ch05-commit-durability.drawio) and
[rendered SVG](../diagrams/ch05-commit-durability.svg) show the durability boundary. `AppendAndFlushCommitRecord` first
uses the existing COMMIT append behavior and only then calls `Flush` with exactly that returned LSN.

An append error means there is no LSN to flush, so flush is not called. A flush error includes both transaction ID and LSN,
preserves the underlying error, and is not retried. Successful completion returns the same LSN, now covered by the log
manager's durable prefix guarantee.

### Test-first record

Success ordering, append failure, flush failure, and concrete `log.Manager` port conformance were tested first. The initial
build failed because `DurableLog` and `AppendAndFlushCommitRecord` were undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendAndFlushCommitRecordFlushesReturnedLSN` | Append once, flush the exact LSN once, return it |
| `TestAppendAndFlushCommitRecordDoesNotFlushAfterAppendFailure` | Append failure cuts off flush |
| `TestAppendAndFlushCommitRecordReturnsFlushFailureWithContext` | Flush failure carries transaction and LSN context, no retry |
| `TestLogManagerSatisfiesDurableRecoveryLog` | Existing log manager satisfies append and flush port |

### Java source map and Go decision

| Java recovery path | Go | Decision |
| --- | --- | --- |
| `CommitRecord.writeToLog` returns LSN | `AppendCommitRecord` | Keep append independently testable |
| `RecoveryMgr.commit` calls `lm.flush(lsn)` | `AppendAndFlushCommitRecord` | Isolate the log durability primitive from buffer flush |
| implicit concrete `LogMgr` | `DurableLog` consumer port | Require only append and flush |
| unchecked flush failure | contextual wrapped error | Expose the exact failed durability cut point |

## Feature 7: Transaction ROLLBACK log record codec

ROLLBACK is the third fixed-width transaction marker. Its operation code is `3`, followed by the same signed four-byte
transaction ID representation used by START and COMMIT. The public type remains distinct because the operation has a
different recovery meaning even though the bytes share a private codec.

```text
offset 0                  offset 4                 offset 8
+-------------------------+------------------------+
| ROLLBACK operation = 3  | transaction ID (int32) |
+-------------------------+------------------------+
```

### Test-first record

Exact positive, zero, and negative-bit-pattern encodings plus malformed inputs were tested first. The initial build failed
because `OperationRollback`, `EncodeRollbackRecord`, and `DecodeRollbackRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackRecordRoundTrip` | Exact ROLLBACK bytes and signed transaction ID round-trip |
| `TestDecodeRollbackRecordRejectsInvalidEncoding` | Truncation, trailing bytes, and a COMMIT operation are rejected |

### Java source map and Go decision

| Java `RollbackRecord` | Go | Decision |
| --- | --- | --- |
| `ROLLBACK = 3` | `OperationRollback Operation = 3` | Preserve the durable operator value |
| operator plus transaction integer | shared private fixed-width codec | Reuse layout without merging semantic types |
| constructor extracts `txnum` | strict `DecodeRollbackRecord` | Validate size and operation before returning the ID |
| `writeToLog` | excluded | Append and flush remain separate behaviors |
| empty `undo` | no undo method yet | Rollback scanning belongs to later orchestration |

## Feature 8: Append a ROLLBACK record

`AppendRollbackRecord` sends the exact Feature 7 encoding through the existing recovery-owned `LogAppender`, once. It
returns the append LSN unchanged; failure carries the ROLLBACK transaction ID, preserves the cause, and is not retried.

### Test-first record

Exact bytes, unchanged LSN, and the no-retry failure path were tested first. The initial build failed because
`AppendRollbackRecord` was undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendRollbackRecordAppendsExactEncodingAndReturnsLSN` | One exact ROLLBACK append and unchanged LSN |
| `TestAppendRollbackRecordReturnsAppendFailureWithContext` | Wrapped ROLLBACK context and no retry |

Java `RollbackRecord.writeToLog` combines encoding and append. Go reuses the separately tested codec and the private
one-call append helper, keeping rollback scanning and durability out of this behavior.

## Feature 9: Flush the appended ROLLBACK LSN

`AppendAndFlushRollbackRecord` appends first, then flushes exactly the returned LSN. Append failure prevents flush; flush
failure carries transaction and LSN context and is not retried. Success returns the same now-durable LSN.

The private append-and-flush sequence is shared with COMMIT, while each operation supplies its own append behavior and
error name. This preserves distinct public recovery meanings without duplicating the durability cut points.

### Test-first record

Success, append failure, and flush failure were fixed before implementation. The initial build failed because
`AppendAndFlushRollbackRecord` was undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendAndFlushRollbackRecordFlushesReturnedLSN` | Flush and return exactly the appended LSN |
| `TestAppendAndFlushRollbackRecordDoesNotFlushAfterAppendFailure` | Append failure cuts off flush |
| `TestAppendAndFlushRollbackRecordReturnsFlushFailureWithContext` | Flush failure has transaction/LSN context and no retry |

Java `RecoveryMgr.rollback` flushes the rollback marker after undo and buffer flush. This feature implements only the final
log durability primitive; undo and buffer coordination remain later transaction behavior.

## Feature 10: SETINT update record codec

The [editable Draw.io diagram](../diagrams/ch05-setint-record.drawio) and
[rendered SVG](../diagrams/ch05-setint-record.svg) show the variable-length undo record. SETINT stores operation `4`, the
transaction, a byte-length-prefixed filename, block number, page offset, and the integer value that existed before update.
Its exact size is `24 + len(filename bytes)`.

`EncodeSetIntRecord` rejects a block number that cannot fit the four-byte durable format. `DecodeSetIntRecord` checks the
operation, signed filename length, complete filename/fixed suffix, and exact final boundary, so truncated or concatenated
records are not accepted accidentally. UTF-8 filenames and signed int32 field bit patterns round-trip unchanged.

### Test-first record

Exact bytes, UTF-8 and signed fields, block overflow, and malformed record boundaries were tested first. The initial build
failed because the SETINT operation, encoder, and decoder did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSetIntRecordRoundTrip` | Exact complete layout and decoded fields |
| `TestSetIntRecordRoundTripsUTF8FilenameAndSignedFields` | Filename bytes and signed values survive round-trip |
| `TestEncodeSetIntRecordRejectsBlockOutsideInt32` | Host-width block cannot silently truncate on disk |
| `TestDecodeSetIntRecordRejectsInvalidEncoding` | Wrong op, length corruption, truncation, and trailing bytes fail |

Java `SetIntRecord` uses `Page` offsets to write the same six values. Go owns the recovery wire layout directly with
big-endian primitives and returns errors for format violations; append, old-value capture from Buffer, and undo remain
separate observable behaviors.

## Feature 11: Append a SETINT record

`AppendSetIntRecord` encodes first, then performs exactly one append and returns its LSN unchanged. Encoding failure makes
no I/O call. Append failure includes transaction and block context, preserves the cause, and is not retried.

### Test-first record

Exact append bytes/LSN, encoding cutoff, and append failure were tested first. The initial build failed because
`AppendSetIntRecord` was undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendSetIntRecordAppendsExactEncodingAndReturnsLSN` | One exact append and unchanged LSN |
| `TestAppendSetIntRecordDoesNotAppendAfterEncodingFailure` | Invalid block width prevents I/O |
| `TestAppendSetIntRecordReturnsAppendFailureWithContext` | Transaction/block context, wrapped cause, no retry |

Java `SetIntRecord.writeToLog` calculates the layout and appends in one method. Go composes the separately tested codec
with the existing append port while keeping old-value capture and undo out of this boundary.

## Feature 12: SETSTRING update record codec

The [editable Draw.io](../diagrams/ch05-setstring-record.drawio) and [SVG](../diagrams/ch05-setstring-record.svg) show the
two variable regions. SETSTRING stores operation `5`, transaction, length-prefixed filename, block, offset, and a separately
length-prefixed old string. Exact size is `24 + filename bytes + old-value bytes`.

The decoder validates both signed lengths before slicing, then requires the exact final boundary. The encoder rejects
block-number narrowing and combined-size overflow. UTF-8 content and signed fixed fields round-trip without coercion.

### Test-first record

Exact bytes, UTF-8/signed fields, block overflow, and corruption of either length were tested first. The initial build
failed because the SETSTRING operation, encoder, and decoder did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSetStringRecordRoundTrip` | Exact two-string layout and decoded fields |
| `TestSetStringRecordRoundTripsUTF8AndSignedFields` | Both strings and signed fields round-trip |
| `TestEncodeSetStringRecordRejectsBlockOutsideInt32` | Block number cannot truncate |
| `TestDecodeSetStringRecordRejectsInvalidEncoding` | Both lengths and exact boundary are strict |

## Feature 13: Append a SETSTRING record

`AppendSetStringRecord` encodes before I/O, appends exactly once, and returns the LSN unchanged. Encoding failure prevents
append; append failure carries transaction/block context, preserves the cause, and is not retried.

### Test-first record

Exact bytes/LSN, encoding cutoff, and contextual append failure were tested first. The initial build failed because
`AppendSetStringRecord` was undefined.

| Test | Observable behavior |
| --- | --- |
| `TestAppendSetStringRecordAppendsExactEncodingAndReturnsLSN` | One exact append and unchanged LSN |
| `TestAppendSetStringRecordDoesNotAppendAfterEncodingFailure` | Invalid block width prevents I/O |
| `TestAppendSetStringRecordReturnsAppendFailureWithContext` | Wrapped transaction/block context and no retry |

## Feature 14: Decode records by operation

The [editable Draw.io](../diagrams/ch05-record-dispatch.drawio) and [SVG](../diagrams/ch05-record-dispatch.svg) show the
single common dispatch point. `DecodeRecord` reads the four-byte operation prefix, selects one of the six strict decoders,
and returns the minimal `Record` interface exposing `Operation`.

Short prefixes and unknown operations fail before dispatch. Once selected, each existing decoder retains authority over
its exact layout, so its boundary and corruption errors pass through unchanged.

### Test-first record

All six dispatch paths, short prefix, unknown operation, and selected-decoder error propagation were tested first. The
initial build failed because `DecodeRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestDecodeRecordDispatchesEveryOperation` | Each operation returns its concrete typed record |
| `TestDecodeRecordRejectsShortAndUnknownOperation` | Prefix/operation errors and strict decoder errors remain explicit |

## Feature 15: Log the previous integer before update

`LogPreviousInteger` consumes a minimal `UpdateView`: assigned block plus Page contents. It reads the current int32
at the requested offset and composes the existing SETINT append behavior before any caller mutation occurs. Missing block,
nil contents, or page range error prevents append.

### Test-first record

Old-value capture, missing assignment, range failure, and concrete Buffer conformance were tested first. The initial build
failed because `UpdateView` and `LogPreviousInteger` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLogPreviousIntegerAppendsValueBeforeUpdate` | Captures block/offset/old value and returns append LSN |
| `TestLogPreviousIntegerRejectsMissingAssignmentBeforeAppend` | Unassigned Buffer causes no log write |
| `TestLogPreviousIntegerRejectsPageReadFailureBeforeAppend` | Invalid page range causes no log write |
| `TestBufferSatisfiesUpdateView` | Existing Buffer directly satisfies the consumer port |

This maps to Java `RecoveryMgr.setInt`'s old-value read and `SetIntRecord.writeToLog` call only. Applying the new value and
marking the buffer dirty remain transaction-layer behavior.

## Feature 16: Log the previous string before update

`LogPreviousString` reuses `UpdateView`, reads the Page's length-prefixed bytes at the requested offset, converts those
bytes to their string value, and composes the existing SETSTRING append behavior. Missing assignment, nil contents, or an
invalid byte region fails before append, so the log cannot describe a value that was not actually observed.

### Test-first record

UTF-8 old-value capture, missing assignment, range failure, and shared Buffer conformance were tested first. The initial
build failed because `UpdateView` and `LogPreviousString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLogPreviousStringAppendsValueBeforeUpdate` | Captures block/offset/UTF-8 old value and returns append LSN |
| `TestLogPreviousStringRejectsMissingAssignmentBeforeAppend` | Unassigned Buffer causes no log write |
| `TestLogPreviousStringRejectsPageReadFailureBeforeAppend` | Invalid length-prefixed region causes no log write |
| `TestBufferSatisfiesUpdateView` | Integer and string logging share the same minimal Buffer port |

This maps to Java `RecoveryMgr.setString`'s old-value read and `SetStringRecord.writeToLog` call only. Applying the new
value, dirty metadata, and undo remain outside this behavior.

## Feature 17: Undo a SETINT record

`SetIntRecord.Undo` consumes an `IntegerUndoTarget` rather than a concrete transaction. It pins the recorded block,
restores the saved integer with logging explicitly disabled, and always unpins after a restore attempt. A pin failure
stops before mutation; restore and unpin failures retain their causes and operation context.

### Test-first record

The exact pin/restore/unpin sequence, no-log flag, and each failure boundary were tested first. The initial build failed
because `SetIntRecord.Undo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSetIntRecordUndoRestoresOldValueWithoutLogging` | Pin, restore with `false`, then unpin |
| `TestSetIntRecordUndoStopsAfterPinFailure` | Failed pin prevents restore and unpin |
| `TestSetIntRecordUndoUnpinsAfterRestoreFailure` | Failed restore still releases the pin |
| `TestSetIntRecordUndoReturnsUnpinFailure` | Release failure remains visible to the caller |

This maps to Java `SetIntRecord.undo`. Go makes failure and cleanup explicit while retaining the book's rule that undo
must not generate another SETINT record.

## Feature 18: Undo a SETSTRING record

`SetStringRecord.Undo` follows the same recovery boundary through a string-specific `StringUndoTarget`: pin the recorded
block, restore the saved UTF-8 value with logging disabled, and release the pin after every restore attempt. Each failure
keeps its cause and identifies the failed phase.

### Test-first record

The exact pin/restore/unpin sequence, UTF-8 value, no-log flag, and each failure boundary were tested first. The initial
build failed because `SetStringRecord.Undo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSetStringRecordUndoRestoresOldValueWithoutLogging` | Pin, restore UTF-8 with `false`, then unpin |
| `TestSetStringRecordUndoStopsAfterPinFailure` | Failed pin prevents restore and unpin |
| `TestSetStringRecordUndoUnpinsAfterRestoreFailure` | Failed restore still releases the pin |
| `TestSetStringRecordUndoReturnsUnpinFailure` | Release failure remains visible to the caller |

This maps to Java `SetStringRecord.undo` and deliberately remains a separate observable behavior from SETINT undo.

## Feature 19: Identify transaction-owned records

`RecordTransaction` uses the typed records' existing `TransactionID` methods to distinguish transaction history from
global records. START, COMMIT, ROLLBACK, SETINT, and SETSTRING return their owner; CHECKPOINT and a nil record report no
owner. The minimal `Record` dispatch interface remains unchanged.

### Test-first record

All transaction record kinds, CHECKPOINT, and nil were tested first. The initial build failed because
`RecordTransaction` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordTransactionReturnsEveryOwnedRecordTransaction` | All five transaction record kinds expose their owner |
| `TestRecordTransactionReportsUnownedCheckpointAndNil` | Global and absent records have no transaction owner |

This supplies Java `LogRecord.txNumber`'s selection role without assigning a sentinel transaction to CHECKPOINT. It does
not iterate, filter, stop, or undo records yet.

## Next behavior

Continue with Sect. 5.3.9.2 backward rollback scanning, next selecting only the requested transaction before adding the
matching START stopping rule and update undo.

## Sources

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 5, Sect. 5.3.9.1
- SimpleDB 3.4 Java source: `simpledb.tx.recovery.LogRecord`, `CheckpointRecord`
