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

## Feature 20: Select one transaction's records

`NextTransactionRecord` consumes the minimal `EncodedRecordIterator`, decodes newest-to-oldest candidates, and returns
only the next record whose owner matches the requested transaction. CHECKPOINT and other transactions are skipped;
iterator exhaustion remains an explicit no-record result. Iterator and decoding failures stop selection with transaction
context.

### Test-first record

Mixed ownership, CHECKPOINT, repeated calls through exhaustion, and both failure sources were tested first. The initial
build failed because `NextTransactionRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNextTransactionRecordSkipsUnownedAndOtherTransactions` | Preserve reverse order while filtering one transaction |
| `TestNextTransactionRecordReturnsIteratorFailure` | Storage traversal failure stops selection with context |
| `TestNextTransactionRecordReturnsDecodeFailure` | Malformed candidate is not silently skipped |

This maps to the `rec.txNumber() == txnum` predicate in Java `RecoveryMgr.doRollback`. Matching START termination and
undo dispatch remain separate behaviors.

## Feature 21: Stop rollback traversal at START

`RollbackRecordIterator` wraps transaction selection with the history boundary required by rollback. It yields matching
records in reverse-log order, consumes but does not expose the matching START, and then remains exhausted without reading
older history. Reaching the beginning of the log before START is a corruption/incomplete-history error rather than a
successful rollback.

### Test-first record

Permanent stopping, protection of older history, missing START, and selection failure were tested first. The initial
build failed because `NewRollbackRecordIterator` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackRecordIteratorStopsPermanentlyAtMatchingStart` | START terminates this transaction without reading older records |
| `TestRollbackRecordIteratorRejectsExhaustionBeforeStart` | Missing history boundary is explicit failure |
| `TestRollbackRecordIteratorReturnsSelectionFailure` | Iterator/decode failures keep rollback context |

This maps to the START branch and return in Java `RecoveryMgr.doRollback`. The iterator still does not invoke undo.

## Feature 22: Undo rollback updates in reverse order

`UndoRollbackRecords` consumes decoded records from the START-bounded source and a `RollbackUndoTarget` supporting both
value types. It invokes SETINT or SETSTRING undo in source order, ignores marker records whose undo is a no-op, and stops
at the first source or restore failure. Because the source is newest-to-oldest, repeated updates to the same location are
restored in the required reverse history order.

### Test-first record

Mixed update types, a no-op marker, source failure, and first-undo failure cutoff were tested first. The initial build
failed because `UndoRollbackRecords` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestUndoRollbackRecordsRestoresUpdatesInSourceOrder` | SETSTRING then SETINT restore in reverse-log order without logging |
| `TestUndoRollbackRecordsReturnsSourceFailure` | Bounded traversal failure stops rollback |
| `TestUndoRollbackRecordsStopsAfterUndoFailure` | Failed newest undo prevents older mutations |

This maps to `rec.undo(tx)` in Java `RecoveryMgr.doRollback`, while iterator construction and rollback durability remain
outside this behavior.

## Feature 23: Compose transaction history rollback

`RollbackTransactionHistory` creates one newest-to-oldest encoded iterator, bounds it with `RollbackRecordIterator`, and
passes that decoded stream to `UndoRollbackRecords`. This is the complete internal history-restoration phase: iterator
creation, transaction selection, START termination, and reverse update undo. It deliberately performs no buffer flush or
ROLLBACK marker I/O.

### Test-first record

End-to-end mixed-transaction history, iterator creation failure, and undo failure propagation were tested first. The
initial build failed because `RollbackTransactionHistory` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackTransactionHistoryCreatesBoundedIteratorAndUndoesUpdates` | One iterator restores only the target history to START |
| `TestRollbackTransactionHistoryStopsAfterIteratorCreationFailure` | Iterator failure prevents every undo operation |
| `TestRollbackTransactionHistoryReturnsBoundedUndoFailure` | Restore failure retains transaction and underlying context |

This composes Java `RecoveryMgr.doRollback` without reproducing its class structure. Public rollback durability remains
the next set of behaviors.

## Feature 24: Flush restored rollback buffers

`RollbackTransactionChanges` first completes `RollbackTransactionHistory`, then invokes the minimal
`TransactionBufferFlusher` exactly once for the same transaction. A history failure prevents flush; a flush failure keeps
its cause and transaction context. No ROLLBACK record is appended at this boundary.

### Test-first record

Successful ordering and identity, history-failure cutoff, and flush failure were tested first. The initial build failed
because `RollbackTransactionChanges` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackTransactionChangesFlushesAfterHistoryUndo` | Flush the same transaction only after all undo operations |
| `TestRollbackTransactionChangesDoesNotFlushAfterHistoryFailure` | Incomplete rollback never persists a partial phase here |
| `TestRollbackTransactionChangesReturnsFlushFailure` | Buffer persistence failure remains explicit |

This maps to `bm.flushAll(txnum)` immediately after `doRollback()` in Java `RecoveryMgr.rollback`. Marker append and LSN
flush remain separate.

## Feature 25: Append ROLLBACK after restored-buffer flush

`RollbackTransactionAndAppend` completes history undo and restored-buffer persistence before composing the existing
`AppendRollbackRecord` behavior. It appends exactly one marker and returns its LSN unchanged. Any earlier failure prevents
append, while append failure retains the existing transaction context. The marker is not flushed here.

### Test-first record

Successful phase order/LSN, history cutoff, buffer-flush cutoff, and append failure were tested first. The initial build
failed because `RollbackTransactionAndAppend` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackTransactionAndAppendWritesMarkerAfterBufferFlush` | Append once after restored pages persist and return LSN |
| `TestRollbackTransactionAndAppendDoesNotAppendAfterHistoryFailure` | Incomplete undo cannot claim rollback completion |
| `TestRollbackTransactionAndAppendDoesNotAppendAfterBufferFlushFailure` | Unpersisted undo cannot claim rollback completion |
| `TestRollbackTransactionAndAppendReturnsAppendFailure` | Marker write failure remains explicit |

This maps to `RollbackRecord.writeToLog` after `bm.flushAll(txnum)` in Java `RecoveryMgr.rollback`. Marker durability is
the next behavior.

## Feature 26: Make transaction rollback durable

`RollbackTransaction` completes history undo, restored-buffer flush, and ROLLBACK append before flushing exactly the
returned marker LSN. It returns that LSN unchanged on success. Any earlier failure prevents marker flush; durability
failure includes both transaction and LSN while preserving its cause.

### Test-first record

Successful LSN identity, early failure cutoff, append failure cutoff, and marker flush failure were tested first. The
initial build failed because `RollbackTransaction` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRollbackTransactionFlushesReturnedMarkerLSN` | Flush exactly the appended marker and return its LSN |
| `TestRollbackTransactionDoesNotFlushAfterEarlierFailure` | Failed history phase performs no marker I/O |
| `TestRollbackTransactionDoesNotFlushAfterAppendFailure` | Failed marker append cannot be flushed |
| `TestRollbackTransactionReturnsMarkerFlushFailure` | Durability failure identifies transaction and LSN |

This completes Java `RecoveryMgr.rollback`'s recovery/durability responsibility. Lock release and buffer unpin ownership
belong to the later public transaction layer.

## Feature 27: Track finished restart transactions

`FinishedTransactions` is a zero-value-ready set updated only by COMMIT and ROLLBACK records. START, update records, and
CHECKPOINT do not change it. Repeated completion markers are idempotent, and unseen transactions remain unfinished.

### Test-first record

Both completion kinds, duplicate observation, all non-completion kinds, and unseen lookup were tested first. The initial
build failed because `FinishedTransactions` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestFinishedTransactionsTracksCommitAndRollbackIdempotently` | Both terminal markers finish their transaction once |
| `TestFinishedTransactionsIgnoresNonCompletionRecords` | START, updates, and CHECKPOINT do not finish transactions |
| `TestFinishedTransactionsReportsUnseenTransactionUnfinished` | Missing completion history stays eligible for undo |

This maps to Java `RecoveryMgr.doRecover`'s `finishedTxs` collection without exposing its mutable representation.

## Feature 28: Select unfinished restart updates

`FinishedTransactions.ShouldUndo` observes each record in newest-to-oldest scan order, records COMMIT/ROLLBACK completion,
and selects only SETINT or SETSTRING from transactions not yet finished. Thus a terminal marker suppresses every older
update for that transaction, while updates from unfinished transactions remain candidates. START, CHECKPOINT, and nil are
never selected.

### Test-first record

Interleaved completion markers, older finished updates, both unfinished update types, boundaries, and nil were tested
first. The initial build failed because `FinishedTransactions.ShouldUndo` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestFinishedTransactionsSelectsOnlyUnfinishedUpdatesInScanOrder` | Scan order filters completed histories and selects unfinished updates |

This maps to the completion-marker and `!finishedTxs.contains(rec.txNumber())` branches in Java
`RecoveryMgr.doRecover`, without adding iteration or undo yet.

## Feature 29: Stop restart scan at CHECKPOINT

`RestartRecordIterator` decodes the newest-to-oldest source and yields records until the first quiescent CHECKPOINT. It
consumes but does not expose that boundary, then remains exhausted without reading older history. Reaching the physical
beginning of the log without a checkpoint is also successful exhaustion. Iterator and decoder failures retain restart
context.

### Test-first record

Permanent checkpoint stopping, older-history protection, end-of-log exhaustion, and both failure sources were tested
first. The initial build failed because `NewRestartRecordIterator` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRestartRecordIteratorStopsPermanentlyAtCheckpoint` | CHECKPOINT protects all older history from restart undo |
| `TestRestartRecordIteratorAllowsEndOfLogBeforeCheckpoint` | A database without an earlier checkpoint can scan to origin |
| `TestRestartRecordIteratorReturnsIteratorAndDecodeFailures` | Traversal and corruption failures remain explicit |

This maps to the CHECKPOINT return and iterator exhaustion in Java `RecoveryMgr.doRecover`. Completion filtering and undo
remain consumers of this bounded stream.

## Feature 30: Undo unfinished restart updates

`UndoRestartRecords` consumes decoded records from the checkpoint-bounded source, maintains `FinishedTransactions`, and
invokes SETINT or SETSTRING undo only for unfinished transactions. The source's newest-to-oldest order is preserved;
COMMIT/ROLLBACK suppress older updates, while START and completed updates cause no mutation. Source or undo failure stops
before processing an older record.

### Test-first record

Interleaved finished/unfinished histories, both update types, source failure, and first-undo failure cutoff were tested
first. The initial build failed because `UndoRestartRecords` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestUndoRestartRecordsRestoresOnlyUnfinishedUpdatesInSourceOrder` | Restore only unfinished SETSTRING/SETINT in reverse-log order |
| `TestUndoRestartRecordsReturnsSourceFailure` | Checkpoint-bounded traversal failure stops restart recovery |
| `TestUndoRestartRecordsStopsAfterUnfinishedUndoFailure` | Failed newest undo prevents older mutations |

This maps to the completion tracking and `rec.undo(tx)` branch in Java `RecoveryMgr.doRecover`. Iterator construction and
post-undo durability remain separate.

## Feature 31: Compose restart history recovery

`RecoverRestartHistory` creates one newest-to-oldest encoded iterator, bounds it with `RestartRecordIterator`, and passes
the decoded stream to `UndoRestartRecords`. This composes iterator creation, CHECKPOINT termination, completion tracking,
and unfinished update undo without performing any post-recovery flush or checkpoint I/O.

### Test-first record

End-to-end mixed finished/unfinished history, protected pre-checkpoint history, iterator creation failure, and undo
failure propagation were tested first. The initial build failed because `RecoverRestartHistory` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecoverRestartHistoryCreatesBoundedIteratorAndUndoesUnfinishedUpdates` | One iterator restores only unfinished post-checkpoint history |
| `TestRecoverRestartHistoryStopsAfterIteratorCreationFailure` | Iterator failure prevents every undo operation |
| `TestRecoverRestartHistoryReturnsUndoFailure` | Restore failure retains restart and underlying context |

This composes Java `RecoveryMgr.doRecover` without reproducing its class structure. Buffer flush and checkpoint
durability remain separate.

## Feature 32: Flush restart-recovered buffers

`RecoverRestartChanges` completes the checkpoint-bounded history phase before invoking the existing
`TransactionBufferFlusher` once for the recovery transaction. History failure prevents flush; persistence failure keeps
its cause and recovery transaction context. No CHECKPOINT record is written here.

### Test-first record

Successful ordering and transaction identity, history-failure cutoff, and flush failure were tested first. The initial
build failed because `RecoverRestartChanges` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecoverRestartChangesFlushesAfterHistoryUndo` | Flush the recovery transaction only after every undo |
| `TestRecoverRestartChangesDoesNotFlushAfterHistoryFailure` | Incomplete recovery cannot persist a partial phase here |
| `TestRecoverRestartChangesReturnsFlushFailure` | Recovery buffer persistence failure remains explicit |

This maps to `bm.flushAll(txnum)` immediately after `doRecover()` in Java `RecoveryMgr.recover`. CHECKPOINT append and
LSN flush remain separate.

## Feature 33: Append a CHECKPOINT record

`AppendCheckpointRecord` composes the existing four-byte quiescent checkpoint encoding with `LogAppender`. It performs
exactly one append, returns the LSN unchanged, and preserves an append failure with checkpoint context and no retry.

### Test-first record

Exact bytes/LSN and contextual append failure were tested first. The initial build failed because
`AppendCheckpointRecord` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestAppendCheckpointRecordAppendsExactEncodingAndReturnsLSN` | One exact append and unchanged LSN |
| `TestAppendCheckpointRecordReturnsAppendFailureWithContext` | Wrapped checkpoint context and no retry |

This maps to Java `CheckpointRecord.writeToLog`; restart-recovery ordering and durability remain separate.

## Feature 34: Append CHECKPOINT after restart flush

`RecoverRestartAndAppend` completes restart history undo and recovered-buffer persistence before composing
`AppendCheckpointRecord`. It appends exactly one checkpoint and returns its LSN unchanged. History or buffer-flush failure
prevents append; append failure retains checkpoint context. The checkpoint LSN is not flushed here.

### Test-first record

Successful phase order/LSN, history cutoff, buffer-flush cutoff, and checkpoint append failure were tested first. The
initial build failed because `RecoverRestartAndAppend` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecoverRestartAndAppendWritesCheckpointAfterBufferFlush` | Append once after restored pages persist and return LSN |
| `TestRecoverRestartAndAppendDoesNotAppendAfterHistoryFailure` | Incomplete recovery cannot establish a new boundary |
| `TestRecoverRestartAndAppendDoesNotAppendAfterBufferFlushFailure` | Unpersisted recovery cannot establish a new boundary |
| `TestRecoverRestartAndAppendReturnsCheckpointFailure` | Checkpoint write failure remains explicit |

This maps to `CheckpointRecord.writeToLog` after `bm.flushAll(txnum)` in Java `RecoveryMgr.recover`. Checkpoint durability
is the next behavior.

## Feature 35: Make restart CHECKPOINT durable

`RecoverRestart` completes history undo, recovered-buffer flush, and CHECKPOINT append before flushing exactly the
returned checkpoint LSN. It returns that LSN unchanged on success. Any earlier failure prevents checkpoint flush;
durability failure includes the recovery transaction and LSN while preserving its cause.

### Test-first record

Successful LSN identity, early failure cutoff, append failure cutoff, and checkpoint flush failure were tested first. The
initial build failed because `RecoverRestart` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecoverRestartFlushesReturnedCheckpointLSN` | Flush exactly the appended checkpoint and return its LSN |
| `TestRecoverRestartDoesNotFlushCheckpointAfterEarlierFailure` | Failed recovery performs no checkpoint I/O |
| `TestRecoverRestartDoesNotFlushCheckpointAfterAppendFailure` | Failed checkpoint append cannot be flushed |
| `TestRecoverRestartReturnsCheckpointFlushFailure` | Durability failure identifies recovery transaction and LSN |

This completes Java `RecoveryMgr.recover`'s recovery/durability responsibility. Lock release and buffer unpin belong to
the later public transaction layer.

## Feature 36: Acquire the first shared block lock

The zero-value-ready `concurrency.LockTable` owns synchronized lock state keyed by `BlockID`. `LockShared` records one
shared holder for an unlocked block, and distinct blocks remain independent. This first behavior has no conflict path;
additional holders, exclusive state, release, and waiting remain separate.

### Test-first record

Zero-value first acquisition and independent block state were tested first. The initial build failed because `LockTable`
did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestZeroValueLockTableAcquiresFirstSharedLock` | An unlocked block becomes shared by one holder |
| `TestLockTableKeepsUncontendedBlocksIndependent` | Locks on different blocks do not share state |

This maps to the non-waiting path of Java `LockTable.sLock`. Go uses a zero-value-ready synchronized table and postpones
conflict policy until the corresponding observable behaviors require it.

## Feature 37: Add compatible shared lock holders

`LockShared` increments the shared-holder count when the same block is already shared. The table mutex serializes
concurrent acquisitions so no increment is lost, while state for other blocks remains unchanged. Shared acquisition
still has no conflict path because exclusive state has not been introduced.

### Test-first record

Sequential and concurrent compatible acquisitions were tested first. The initial test run failed because every
acquisition replaced the holder count with one.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableAddsCompatibleSharedHolder` | A second compatible request raises the holder count to two |
| `TestLockTableSerializesConcurrentSharedHolders` | Concurrent requests preserve every holder and other block state |

This completes the compatible path of Java `LockTable.sLock`: the integer value represents the number of shared
holders. Exclusive state and its conflict policy remain separate behaviors.

## Feature 38: Upgrade a sole shared holder to exclusive

`LockExclusive` changes a block held by one shared holder to the exclusive sentinel `-1`. The transition is serialized
by the table mutex and leaves other blocks untouched. This behavior covers only the uncontended success path; callers
must already hold the sole shared lock.

### Test-first record

The sole-holder transition and independent block state were tested first. The initial build failed because
`LockExclusive` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableUpgradesSoleSharedHolderToExclusive` | One shared holder becomes exclusive while other blocks remain unchanged |

This maps to the non-waiting assignment in Java `LockTable.xLock`, where `-1` denotes exclusive ownership. Detection of
other shared holders and its wait policy remain separate behaviors.

## Feature 39: Reject exclusive upgrade with competing shared holders

`LockExclusive` checks the shared-holder count while holding the table mutex. A count greater than one returns the
distinguishable `ErrLockConflict` sentinel and preserves both that count and every other block's state. It does not wait
or apply a timeout yet.

### Test-first record

The competing-holder rejection and state preservation were tested first. The initial build failed because
`ErrLockConflict` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableRejectsExclusiveUpgradeWithCompetingSharedHolder` | Competing shared holders prevent the exclusive transition without changing state |

The `count > 1` predicate maps directly to Java `LockTable.hasOtherSLocks`. Go exposes the temporary non-waiting result
as a sentinel so the later waiting behavior can distinguish contention from other failures.

## Feature 40: Release one of multiple shared holders

`Unlock` decrements a shared-holder count greater than one under the table mutex. The block remains represented because
at least one compatible holder still exists, and other block state is unchanged. Counts of one and the exclusive
sentinel are intentionally left for the separate final-release behavior.

### Test-first record

One release from three shared holders was tested first. The initial build failed because `Unlock` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableReleasesOneOfMultipleSharedHolders` | Releasing one shared hold changes three holders to two without affecting another block |

This maps to the `val > 1` decrement branch of Java `LockTable.unlock`. Removing the final lock and waking waiters are
kept out of this behavior.

## Feature 41: Remove the final shared block lock

`Unlock` removes a block entry when its shared-holder count is exactly one. The removal happens under the table mutex,
leaves other blocks unchanged, and performs no waiter notification because waiting infrastructure has not been
introduced. Exclusive release remains separate.

### Test-first record

Final shared removal and independent block state were tested first. The initial test failed because `Unlock` left the
count-one entry unchanged.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableRemovesFinalSharedLock` | Releasing the sole shared holder removes only that block's entry |

This implements the shared-lock case of Java `LockTable.unlock`'s removal branch. Java's accompanying `notifyAll` and
the exclusive-lock case remain later behaviors.

## Feature 42: Remove an exclusive block lock

`Unlock` removes a block whose state is the exclusive sentinel `-1`. The removal is serialized by the table mutex,
preserves other blocks, and deliberately performs no waiter notification.

### Test-first record

Exclusive removal and independent block state were tested first. The initial test failed because `Unlock` retained the
`-1` entry.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableRemovesExclusiveLock` | Releasing an exclusive lock removes only that block's entry |

This completes the state-removal cases in Java `LockTable.unlock`. Notification remains separate so its observable
wakeup behavior can be tested independently.

## Feature 43: Reject shared acquisition while exclusive

`LockShared` checks for the negative exclusive sentinel while holding the table mutex. It returns `ErrLockConflict`
without changing the exclusive state or any other block. Waiting and timeout remain outside this non-waiting conflict
path.

### Test-first record

Shared acquisition against an exclusive holder and state preservation were tested first. The initial test failed
because `LockShared` incremented `-1` and returned success.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableRejectsSharedAcquisitionWhileExclusiveLockExists` | An exclusive holder rejects a shared request without changing lock state |

The negative-state predicate maps to Java `LockTable.hasXlock`. The sentinel result gives the later waiting wrapper a
precise contention signal.

## Feature 44: Notify lock waiters after final release

`LockTable` lazily creates a condition variable bound to the same mutex that protects lock state. Removing a final
shared or exclusive entry broadcasts after the state mutation, allowing every registered waiter to reacquire the mutex
and recheck its predicate. An ordinary partial shared release does not broadcast; a later exclusive-wait behavior adds
a targeted exception when the count reaches the requester's sole shared hold.

The [editable Draw.io diagram](../diagrams/ch05-lock-release-notification.drawio) and
[rendered SVG](../diagrams/ch05-lock-release-notification.svg) show why state mutation and notification share one mutex,
and why a partial shared release stays on the no-notification path.

### Test-first record

Final shared/exclusive wakeup and partial-release silence were tested first. The initial build failed because
`waitForChangeLocked` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableNotifiesWaitersAfterFinalRelease` | Final shared and exclusive release wake a registered waiter |
| `TestLockTableDoesNotNotifyWaitersAfterPartialSharedRelease` | A remaining shared holder prevents notification |

This maps to Java `LockTable.unlock`'s `notifyAll` after entry removal. Actual shared/exclusive wait loops and bounded
timeouts remain separate behaviors.

## Feature 45: Wait for exclusive release before shared acquisition

`WaitShared` holds the table mutex while checking the negative exclusive predicate. On conflict it waits on the same
condition variable, then reacquires the mutex and loops to recheck state. After final release broadcasts, it increments
the now-compatible state exactly once. The existing `LockShared` remains the non-waiting primitive.

The wait/release/recheck path extends the state transition shown in the
[editable Draw.io diagram](../diagrams/ch05-lock-release-notification.drawio) and
[rendered SVG](../diagrams/ch05-lock-release-notification.svg).

### Test-first record

Blocking before release, state preservation while blocked, and acquisition after notification were tested first. The
initial build failed because `WaitShared` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableWaitsForExclusiveReleaseBeforeAcquiringShared` | An exclusive holder blocks a shared request until final release, then one shared hold is granted |

This is the unbounded wait/recheck portion of Java `LockTable.sLock`. The ten-second abort policy is intentionally a
separate behavior.

## Feature 46: Time out a blocked shared request

`WaitSharedFor` applies a caller-provided deadline to the same shared wait/recheck loop. A timer reacquires the table
mutex, marks the wait expired, and broadcasts the condition; the request then rechecks the exclusive predicate before
returning `ErrLockTimeout`. If final release wins before the deadline, the timer is stopped and one shared hold is
granted. Timeout changes no lock state.

### Test-first record

Deadline abort/state preservation and successful acquisition before the deadline were tested first. The initial build
failed because `WaitSharedFor` and `ErrLockTimeout` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableTimesOutBlockedSharedRequest` | A persistent exclusive holder causes a distinguishable timeout without state mutation |
| `TestLockTableBoundedSharedRequestAcquiresBeforeDeadline` | Final release before the deadline grants one shared hold |

This maps to Java `LockTable.sLock`'s elapsed-time check and `LockAbortException`. Go accepts the duration explicitly so
tests and future transaction policy do not depend on a package-global ten-second delay.

## Feature 47: Wait for competing shared holders before exclusive acquisition

`WaitExclusive` assumes the requester already owns one shared hold. While the count is greater than one it registers an
exclusive waiter and sleeps on the table condition. Releasing a competing holder from count two to one broadcasts only
when such a waiter is registered. The awakened request rechecks the predicate and changes the sole shared state to the
exclusive sentinel `-1`. Ordinary partial release still does not notify unrelated waiters.

The [editable Draw.io diagram](../diagrams/ch05-lock-release-notification.drawio) and
[rendered SVG](../diagrams/ch05-lock-release-notification.svg) now include the count-two to count-one exclusive-upgrade
wakeup path.

### Test-first record

Blocked state and upgrade after the competing holder releases were tested first. The initial build failed because
`WaitExclusive` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableWaitsForCompetingSharedHolderBeforeAcquiringExclusive` | Other shared holders block upgrade; their final competing release wakes and grants exclusive state |

This implements the unbounded wait/recheck portion of Java `LockTable.xLock`. Bounded abort remains separate.

## Feature 48: Time out a blocked exclusive upgrade

`WaitExclusiveFor` applies a caller-provided deadline to exclusive upgrade waiting. While competing shared holders
remain, the request registers itself as an exclusive waiter and sleeps on the table condition. A deadline timer
reacquires the table mutex and wakes that condition. The awakened request always removes its waiter registration before
either returning `ErrLockTimeout` with the shared count unchanged or upgrading the sole remaining shared hold to `-1`.

The existing count-two to count-one notification path in the
[editable Draw.io diagram](../diagrams/ch05-lock-release-notification.drawio) and
[rendered SVG](../diagrams/ch05-lock-release-notification.svg) also serves bounded exclusive waiters.

### Test-first record

Deadline abort/state preservation, waiter cleanup, and successful upgrade before the deadline were tested first. The
initial build failed because `WaitExclusiveFor` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLockTableTimesOutBlockedExclusiveUpgrade` | Persistent competing shared holders cause a distinguishable timeout without lock-state mutation or a stale waiter registration |
| `TestLockTableBoundedExclusiveUpgradeAcquiresBeforeDeadline` | The final competing release before the deadline grants exclusive state and removes the waiter registration |

This maps to Java `LockTable.xLock`'s elapsed-time check and `LockAbortException`. As with bounded shared acquisition,
Go accepts the duration explicitly so tests and future transaction policy do not depend on a package-global delay.

## Feature 49: Reuse a transaction-local shared lock

`Manager` represents the lock state of one transaction and receives the shared `LockTable` as a constructor dependency.
Its local map records blocks already held by that transaction. `LockShared` asks the table to acquire only when the
block is absent locally, so a repeated request remains one global shared hold. Different blocks are acquired and
recorded independently.

### Test-first record

Repeated acquisition, independent block state, and local shared-mode recording were tested first. The initial build
failed because `Manager`, `NewManager`, and the local shared mode did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestManagerReusesTransactionSharedLock` | One transaction's repeated shared request reuses its existing global hold while a different block is acquired independently |

This maps to Java `ConcurrencyMgr.sLock` and its per-instance `locks` map. Go injects the table rather than hiding it in
a static field, which keeps the global coordination boundary explicit and independently testable. Exclusive upgrade and
release remain separate behaviors.

## Feature 50: Upgrade a transaction-local lock to exclusive

`Manager.LockExclusive` first returns immediately when the transaction already records exclusive ownership. Otherwise
it reuses `LockShared` to establish the transaction's one shared hold, asks the shared table to wait for and perform the
exclusive upgrade, and records the exclusive local mode only after that upgrade succeeds. Locks tracked for other blocks
remain unchanged.

### Test-first record

Acquisition from no local lock, repeated exclusive reuse, and preservation of another local shared lock were tested
first. The initial build failed because `LockExclusive` and the local exclusive mode did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestManagerUpgradesTransactionLockToExclusiveOnce` | A transaction establishes and upgrades one block exactly once while retaining a different shared block |

This maps to Java `ConcurrencyMgr.xLock`: obtain shared ownership if necessary, upgrade through the global table, then
replace the local `S` marker with `X`. Go keeps error propagation explicit and does not update local ownership after a
failed table operation. Releasing the transaction's tracked locks remains a separate behavior.

## Feature 51: Release all transaction-local locks

`Manager.Release` visits each block recorded by one transaction, asks the shared `LockTable` to release that block once,
then clears the local ownership map. Shared and exclusive modes both correspond to one table entry per manager, so the
same release operation handles either mode. An empty map makes a repeated release a no-op, while another manager's local
and global locks are outside the iteration and remain unchanged.

### Test-first record

Shared and exclusive release, local cleanup, repeated-release idempotence, and isolation from another manager were
tested first. The initial build failed because `Release` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestManagerReleasesAllTransactionLocks` | Releasing one manager removes each of its global holds and local records exactly once without affecting another manager |

This maps directly to Java `ConcurrencyMgr.release`, including one global unlock per locally tracked block followed by
clearing the transaction's map. Connecting this lifecycle to commit and rollback remains a separate transaction behavior.

## Feature 52: Track a transaction-pinned buffer

`transaction.BufferList` receives the shared buffer manager through the narrow `BufferPinner` port. `Pin` delegates the
block request first and records both the returned frame and one pin occurrence only after that request succeeds. `Buffer`
then exposes the frame currently owned for a block, while an unpinned block is reported as absent. A failed shared-manager
pin leaves both local collections unchanged.

### Test-first record

Successful delegation and lookup, one local pin occurrence, missing-block lookup, and failure atomicity were tested first.
The initial build failed because `NewBufferList` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestBufferListPinsAndTracksBlock` | A successful shared-manager pin records its frame and one local occurrence for later transaction use |
| `TestBufferListDoesNotTrackFailedPin` | A failed pin propagates its cause without creating transaction-local ownership |

This maps to Java `BufferList.pin` and `getBuffer`. Go introduces a small port so transaction-local ownership depends only
on the shared manager operation it currently needs. Repeated pins and both unpin paths remain separate behaviors.

## Feature 53: Release one of repeated transaction pins

`BufferList.Unpin` resolves the transaction's tracked frame, delegates one release to the shared buffer manager, and then
removes exactly one matching block occurrence from the local pin sequence. When another occurrence remains, block lookup
continues to return the same frame. An untracked block returns `ErrBufferNotPinned` without calling the shared manager.
Local state is mutated only after shared-manager unpin succeeds.

### Test-first record

Repeated pin tracking, one delegated release, remaining lookup, one-occurrence removal, and missing-block rejection were
tested first. The initial build failed because `Unpin` and `ErrBufferNotPinned` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestBufferListReleasesOneOfRepeatedPins` | One unpin after two pins releases the shared frame once and retains one local occurrence and lookup |
| `TestBufferListRejectsUnpinForMissingBlock` | An untracked block returns a distinguishable error without delegating an invalid release |

This maps to the repeated-pin path of Java `BufferList.unpin`, including removing one list occurrence while retaining the
block-to-buffer mapping. Removing that mapping after the final occurrence remains the next separate behavior.

## Feature 54: Remove transaction buffer lookup after final unpin

After a successful shared-manager release and removal of one pin occurrence, `BufferList.Unpin` scans the remaining local
occurrences for that block. It deletes the block-to-buffer mapping only when none remain. If the shared manager rejects the
unpin, the method returns that cause before changing either the occurrence sequence or lookup map.

### Test-first record

Final lookup removal, empty occurrence state, and failure atomicity were tested first. The initial test failed because the
lookup mapping remained after the final occurrence was removed.

| Test | Observable behavior |
| --- | --- |
| `TestBufferListRemovesBufferAfterFinalUnpin` | Releasing the only local pin removes both its occurrence and block lookup |
| `TestBufferListPreservesOwnershipWhenUnpinFails` | A shared-manager unpin failure leaves local occurrence and lookup unchanged |

This completes the final-pin path of Java `BufferList.unpin` while preserving the repeated-pin behavior from Feature 53.
Bulk unpin remains separate because it has different iteration and partial-failure behavior.

## Feature 55: Release all transaction buffer pins

`BufferList.UnpinAll` repeatedly releases the first local occurrence until the acquisition-ordered sequence is empty.
Delegating through `Unpin` gives repeated pins one shared-manager release each and reuses the existing rule that removes a
block lookup only after its final occurrence. Once successful, both local collections are empty and a later call is a
no-op.

### Test-first record

Multiple blocks, repeated occurrences, acquisition-order release, complete cleanup, and repeated-call idempotence were
tested first. The initial build failed because `UnpinAll` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestBufferListReleasesAllPinOccurrences` | Bulk release delegates once per occurrence in acquisition order and clears every local buffer and pin record |

This maps to Java `BufferList.unpinAll`. Go returns an error so a shared-manager failure is visible; defining an explicit
partial-failure retry contract remains separate from the successful bulk-release behavior.

## Feature 56: Allocate transaction identifiers

The zero value of `transaction.IDAllocator` starts at zero internally and atomically increments before returning an ID,
so a fresh allocator yields positive identifiers `1, 2, 3, ...`. `Next` returns `recovery.TransactionID` directly,
making the four-byte recovery-record identifier the canonical transaction lifecycle type. Atomic increment prevents
duplicate IDs when transaction construction is concurrent.

### Test-first record

Sequential allocation, canonical return type, concurrent uniqueness, positivity, and a gap-free completed allocation set
were tested first. The initial build failed because `IDAllocator` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestIDAllocatorReturnsSequentialRecoveryTransactionIDs` | A fresh allocator returns canonical recovery transaction IDs starting at one in sequence |
| `TestIDAllocatorReturnsUniqueConcurrentIDs` | Concurrent calls produce one unique ID for every completed allocation without gaps |

This maps to Java `Transaction.nextTxNumber`, replacing its synchronized static integer with an injectable, zero-value-ready
atomic allocator. Persisting or restoring the sequence across process restarts is not part of the Java behavior here.

## Feature 57: Initialize transaction-local state

`transaction.New` consumes exactly one canonical ID from an injected `IDAllocator`, creates a fresh concurrency manager
over the shared lock table, and creates fresh buffer ownership over the shared buffer port. The resulting `Transaction`
exposes its recovery ID while keeping its local resource managers private. Separate constructor calls therefore share
global coordination dependencies but never share local ownership objects.

### Test-first record

ID consumption, canonical ID exposure, distinct local concurrency and buffer managers, and empty initial pin state were
tested first. The initial build failed because `New` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewInitializesIsolatedTransactionLocalState` | Two constructions consume successive IDs and create isolated local ownership over shared dependencies |

This maps to the initialization portion of Java `Transaction`, but dependency assembly is explicit instead of reading
static server state. START logging remains separate so constructor state and recovery side effects can be tested independently.

## Feature 58: Append START while creating a transaction

`transaction.New` now accepts the recovery log's append port. It allocates the transaction ID, appends exactly one START
record for that ID, and returns initialized local state only after append succeeds. An append failure is wrapped with the
transaction context and returns no partially initialized transaction. The returned LSN is intentionally not flushed,
matching the existing START-record durability policy.

### Test-first record

One START per transaction, encoded ID correspondence, successful construction ordering, and append-failure propagation
were tested first. The initial build failed because `New` did not accept a log or return an error.

| Test | Observable behavior |
| --- | --- |
| `TestNewInitializesIsolatedTransactionLocalState` | Each successful construction appends one START carrying the same ID exposed by the transaction |
| `TestNewReturnsNoTransactionWhenStartAppendFails` | START append failure returns its cause and no transaction instance |

This maps to Java `RecoveryMgr` construction invoked from the `Transaction` constructor. Go makes the fallible side effect
part of the constructor result instead of relying on an unchecked exception. START flush remains intentionally absent.

## Feature 59: Flush transaction buffers before durable COMMIT

`recovery.CommitTransaction` first invokes the transaction-scoped buffer flusher. Only after that succeeds does it reuse
`AppendAndFlushCommitRecord` to append COMMIT and flush through the returned LSN. The result is the durable COMMIT LSN.
A buffer-flush failure is wrapped with transaction context and stops before any log append or flush.

### Test-first record

Buffer-before-log ordering, durable LSN return, COMMIT transaction identity, and early stop after buffer-flush failure
were tested first. The initial build failed because `CommitTransaction` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestCommitTransactionFlushesBuffersBeforeDurableMarker` | Modified buffers are flushed before COMMIT append and COMMIT LSN flush |
| `TestCommitTransactionStopsAfterBufferFlushFailure` | Buffer flush failure prevents both COMMIT append and log flush |

This maps to Java `RecoveryMgr.commit`: `BufferMgr.flushAll(txnum)` precedes `CommitRecord.writeToLog` and log flush.
Transaction lock release and buffer unpin remain outer lifecycle behaviors and are intentionally not included here.

## Feature 60: Invoke durable commit from a transaction

`Transaction` now retains the durable log and a `BufferManager` port that combines pin ownership with transaction-scoped
flush. `Commit` adapts its canonical recovery ID to the buffer package's transaction ID and delegates to
`recovery.CommitTransaction`. It therefore returns only after its dirty buffers are flushed, COMMIT is appended, and the
COMMIT LSN is durable. START remains append-only during construction.

### Test-first record

The transaction's own ID, buffer-before-log ordering, START/COMMIT record sequence, returned COMMIT LSN, and COMMIT flush
were tested first. The initial build failed because `Transaction.Commit` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionCommitInvokesDurabilityForOwnID` | A transaction commits its own ID through buffer flush, COMMIT append, and COMMIT LSN flush in order |
| `TestTransactionCommitFailureKeepsPinnedBuffers` | A durability failure leaves transaction-local buffer ownership intact and writes no COMMIT |

This implements the recovery portion of Java `Transaction.commit` while preserving explicit Go errors. Lock release and
buffer unpin are deliberately still absent so each cleanup transition can be observed and tested independently.

## Feature 61: Release locks after durable COMMIT

After `recovery.CommitTransaction` returns a durable COMMIT LSN, `Transaction.Commit` calls its local concurrency
manager's `Release`. Shared and exclusive ownership are therefore removed only after the transaction's updates and COMMIT
marker are durable. Any durability error returns before release, leaving the transaction's locks intact for retry or
failure handling. The successful method still returns the COMMIT LSN.

### Test-first record

Exclusive conflict before commit, shared availability after durable commit, and lock retention after durability failure
were tested first. The initial success test failed because the exclusive lock remained after commit.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionCommitReleasesLocksAfterDurability` | Durable commit releases exclusive ownership so another shared acquisition can proceed |
| `TestTransactionCommitFailureKeepsLocksHeld` | A failed durability step returns before lock release and preserves the exclusive conflict |

This adds the `concurMgr.release()` portion of Java `Transaction.commit` after recovery commit. Buffer pin cleanup remains
separate so its ordering and failure behavior can be tested independently.

## Feature 62: Unpin buffers after committed lock release

After durable commit and local lock release, `Transaction.Commit` calls `BufferList.UnpinAll`. Successful cleanup removes
every pin occurrence and buffer lookup. Because lock release happens first, a competing shared acquisition can already
succeed when the first shared-manager unpin is observed. If unpin fails, the method returns cleanup context together with
the already-durable COMMIT LSN; the failed occurrence remains locally tracked for retry while locks stay released.

### Test-first record

Lock-before-unpin ordering, complete pin cleanup, durable LSN preservation after cleanup failure, and failed-occurrence
retention were tested first. The initial tests failed because commit never invoked unpin-all.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionCommitUnpinsBuffersAfterLockRelease` | Commit releases locks before unpinning and clears all local buffer ownership |
| `TestTransactionCommitReturnsDurableLSNWhenUnpinFails` | Post-commit unpin failure returns the durable LSN and retains the failed pin while locks remain released |

This completes Java `Transaction.commit` ordering: recovery commit, concurrency release, then `BufferList.unpinAll`.
Go additionally reports cleanup failure without obscuring that COMMIT is already durable.

## Feature 63: Pin a block through a transaction

`Transaction.Pin` delegates directly to its transaction-local `BufferList`. A successful shared-manager pin is therefore
available to later transaction operations through local block lookup, while a pin failure is returned without ownership
state. Pinning is deliberately independent of concurrency control: it can load a buffer even when the block is currently
locked exclusively elsewhere.

### Test-first record

One delegated pin, local frame lookup, lock independence, and failure atomicity were tested first. The initial build
failed because `Transaction.Pin` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionPinsBlockWithoutAcquiringLock` | Transaction pinning records the shared frame without attempting a block lock |
| `TestTransactionPinFailureDoesNotRecordBuffer` | A shared-manager pin failure propagates without local buffer ownership |

This maps directly to Java `Transaction.pin`, which delegates to `BufferList.pin`; value access methods acquire their own
locks separately.

## Feature 64: Unpin a block through a transaction

`Transaction.Unpin` delegates one occurrence to `BufferList.Unpin`. Releasing the final occurrence removes the local
block-to-buffer lookup, and missing ownership or shared-manager failures are returned unchanged through the wrapped error
chain. Buffer unpin is independent from concurrency ownership, so an exclusive block lock remains held afterward.

### Test-first record

One delegated release, final lookup removal, lock preservation, and missing-block error propagation were tested first.
The initial build failed because `Transaction.Unpin` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionUnpinsFinalOccurrenceWithoutReleasingLock` | Final transaction unpin removes buffer ownership while leaving its exclusive lock intact |
| `TestTransactionUnpinPropagatesOwnershipErrors` | Unpinning an unowned block returns `ErrBufferNotPinned` |

This maps directly to Java `Transaction.unpin`, keeping buffer lifetime distinct from concurrency lifetime.

## Feature 65: Read an integer through a shared transaction lock

`Transaction.GetInt` first requires transaction-local ownership of the target buffer, then acquires a shared lock through
the transaction's concurrency manager and reads the page's four-byte big-endian integer. The manager retains one global
shared hold for the transaction and reuses it on repeated reads. Missing ownership, lock acquisition, and page-boundary
failures retain the block and offset in their error context.

Checking ownership before locking is an intentional Go safety difference from Java `Transaction.getInt`, which assumes
the caller already pinned the block and indexes its buffer list after locking. The explicit check prevents a failed
precondition from leaving a newly acquired lock behind.

### Test-first record

Pinned-page decoding, retained shared ownership, repeated-read lock reuse, missing-pin failure, and page-boundary failure
were tested first. The initial build failed because `Transaction.GetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionGetIntReadsPinnedPageAndReusesSharedLock` | Two reads return the stored integer while retaining exactly one transaction-local shared hold |
| `TestTransactionGetIntReturnsMissingPinAndPageFailures` | Missing ownership is distinguishable and invalid offsets preserve the page error |

This maps to Java `Transaction.getInt`; string decoding remains a separate behavior and PR.

## Feature 66: Read a string through a shared transaction lock

`Transaction.GetString` requires transaction-local buffer ownership, acquires a shared lock through the same local
concurrency manager used by integer reads, then decodes the page's four-byte length prefix and returns the payload as a
Go string. Repeated reads reuse one global shared hold. Missing ownership, negative or out-of-range lengths, lock
failure, and page-boundary failure retain block and offset context.

As with `GetInt`, ownership is checked before locking so a missing pin cannot leave a newly acquired shared hold behind.
The stored bytes are converted directly to a Go string, matching SimpleDB's byte-preserving storage contract without
adding Unicode normalization or validation at the transaction layer.

### Test-first record

UTF-8 payload decoding, retained shared ownership, repeated-read lock reuse, missing-pin failure, negative length, and
page-boundary failure were tested first. The initial build failed because `Transaction.GetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionGetStringReadsPinnedPageAndReusesSharedLock` | Two reads return the stored UTF-8 string while retaining exactly one transaction-local shared hold |
| `TestTransactionGetStringReturnsMissingPinAndPageFailures` | Missing ownership, negative lengths, and invalid offsets return explicit errors |

This maps to Java `Transaction.getString`; mutation remains in later, independently tested behaviors.

## Feature 67: Write an integer without a recovery record

The `logUpdate == false` path of `Transaction.SetInt` requires transaction-local buffer ownership, upgrades the block to
an exclusive transaction lock, stores the four-byte integer, and marks the frame dirty with the transaction ID. It does
not append a SETINT record or associate a recovery LSN, which is the write mode recovery undo needs to avoid logging its
own restoration.

Missing ownership is rejected before locking. A page-boundary failure occurs after exclusive acquisition but before the
dirty marker, so no invalid page mutation can be flushed. At this stack point, `logUpdate == true` returned an explicit
error instead of silently performing an unlogged update; the next feature replaces that guard with WAL metadata.

### Test-first record

Exclusive ownership, page mutation, dirty attribution, absent recovery LSN, absent SETINT append, missing-pin failure,
page-boundary failure, and rejection of the not-yet-implemented logged path were tested first. The initial build failed
because `Transaction.SetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionSetIntWithoutLoggingUpdatesPinnedPageExclusively` | The no-log path updates the page, marks the transaction dirty, retains exclusive ownership, and writes no recovery record |
| `TestTransactionSetIntWithoutLoggingReturnsPreconditionAndPageFailures` | Missing ownership and invalid offsets fail without dirtying the frame, while logged writes cannot silently bypass WAL |

This maps to Java `Transaction.setInt(..., false)` and supplies the `IntegerUndoTarget` signature used by SETINT undo.

## Feature 68: Log the previous integer before a transaction write

The `logUpdate == true` path now acquires the transaction's exclusive block lock and calls
`recovery.LogPreviousInteger` before changing the page. The SETINT record captures the current transaction ID, assigned
block, offset, and old integer. Only a successful append permits the new value to be stored; the frame is then marked
dirty and associated with the returned recovery LSN for later WAL-ordered buffer flush.

If recovery append fails, `SetInt` returns the wrapped cause while the page retains its old value and the frame remains
clean with no recovery LSN. The existing `logUpdate == false` branch still performs no append and records no LSN.

### Test-first record

Old-value capture, append-before-mutate ordering, SETINT field identity, dirty attribution, recovery LSN propagation, and
append-failure atomicity were tested first. The initial success test failed with the deliberate logged-update guard from
Feature 67.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionSetIntLogsPreviousValueBeforeMutation` | SETINT records the old value before the page changes, then the frame carries the returned LSN |
| `TestTransactionSetIntLogFailurePreservesPageAndMetadata` | Append failure preserves the old page value and clean metadata |

This maps to Java `Transaction.setInt(..., true)` and its `RecoveryMgr.setInt` old-value append.

## Feature 69: Write a string without a recovery record

The `logUpdate == false` path of `Transaction.SetString` requires the block's transaction-local buffer, retains an
exclusive lock, writes the UTF-8 bytes with the page's four-byte length prefix, and marks the frame dirty for the current
transaction. It appends no SETSTRING record and associates no recovery LSN, so recovery can restore an old string without
logging the restoration itself.

Missing ownership is rejected before locking. Insufficient page capacity is returned before dirty metadata changes. At
this stack point the logged branch remained an explicit error, preventing a caller from silently receiving an unlogged
string update before the next feature added old-value capture.

### Test-first record

Length-prefixed UTF-8 mutation, exclusive ownership, dirty attribution, absent recovery metadata, missing-pin failure,
capacity failure, and logged-path rejection were tested first. The initial build failed because
`Transaction.SetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionSetStringWithoutLoggingUpdatesPinnedPageExclusively` | The no-log path writes the string, marks the transaction dirty, retains exclusive ownership, and appends nothing |
| `TestTransactionSetStringWithoutLoggingReturnsPreconditionAndPageFailures` | Missing ownership and insufficient capacity fail without dirtying the frame, while logged writes cannot bypass WAL |

This maps to Java `Transaction.setString(..., false)` and supplies the `StringUndoTarget` signature used by SETSTRING undo.

## Feature 70: Log the previous string before a transaction write

The `logUpdate == true` path acquires the transaction's exclusive block lock and calls `recovery.LogPreviousString`
before changing the length-prefixed page bytes. The SETSTRING record carries the current transaction ID, assigned block,
offset, and old string. After append succeeds, the new bytes are stored, the frame is marked dirty, and the returned LSN
is attached for WAL-ordered persistence.

Append failure returns before mutation, leaving the old string and clean frame metadata intact. The existing no-log path
continues to append nothing and carries no recovery LSN.

### Test-first record

Old-string capture, append-before-mutate ordering, SETSTRING identity, UTF-8 replacement, dirty attribution, recovery
LSN propagation, and append-failure atomicity were tested first. The initial tests failed against Feature 69's explicit
logged-update guard.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionSetStringLogsPreviousValueBeforeMutation` | SETSTRING records the old string before replacement, then the frame carries the returned LSN |
| `TestTransactionSetStringLogFailurePreservesPageAndMetadata` | Append failure preserves the old string and clean metadata |

This maps to Java `Transaction.setString(..., true)` and its `RecoveryMgr.setString` old-value append.

## Feature 71: Invoke durable rollback from a transaction

`Transaction.Rollback` passes its injected newest-to-oldest iterator factory, own ID, and itself as the undo target to
`recovery.RollbackTransaction`. Existing recovery composition restores SETINT/SETSTRING values without logging, flushes
the restored buffers, appends ROLLBACK, and flushes that marker's LSN. The transaction returns the durable LSN unchanged.

The iterator factory is an optional constructor dependency so transactions that only commit keep their existing setup;
calling rollback without it returns an explicit configuration error. This feature intentionally leaves locks and the
original buffer pins owned after both success and failure. Their post-durability cleanup order remains separately tested.

### Test-first record

Own-ID undo, restored value, durable ROLLBACK identity, retained locks/pins, and iterator failure propagation were tested
first. The initial build failed because the constructor did not accept the factory and `Transaction.Rollback` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRollbackInvokesDurabilityForOwnID` | Logged updates are restored and a durable own-ID ROLLBACK is returned while resources remain owned |
| `TestTransactionRollbackFailureKeepsLocksAndPins` | Iterator failure prevents durability and preserves transaction locks and pins |

This implements the recovery portion of Java `Transaction.rollback`; cleanup follows in independent features.

## Feature 72: Release locks after durable ROLLBACK

After `recovery.RollbackTransaction` returns a durable marker LSN, `Transaction.Rollback` releases every lock tracked by
its local concurrency manager and returns that LSN unchanged. Any iterator, undo, buffer flush, marker append, or marker
flush error returns before release, preserving lock ownership. Buffer pins deliberately remain owned in both cases.

### Test-first record

Shared availability after success, pin retention, LSN preservation, and lock retention after rollback failure were tested
first. The success test initially failed because the exclusive lock remained after durability.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRollbackInvokesDurabilityForOwnID` | Durable rollback restores the value, preserves its LSN and pin, then releases the exclusive lock |
| `TestTransactionRollbackFailureKeepsLocksAndPins` | A failed rollback leaves both lock and buffer ownership intact |

This adds the `concurMgr.release()` portion of Java `Transaction.rollback` after recovery durability.

## Feature 73: Unpin buffers after rolled-back lock release

After durable rollback and local lock release, `Transaction.Rollback` calls `BufferList.UnpinAll`. Successful cleanup
removes every pin occurrence and block lookup. If shared-manager unpin fails, rollback returns the already-durable marker
LSN with cleanup context; the failed occurrence remains tracked for retry while locks stay released. Earlier rollback
failure still returns before either cleanup phase.

### Test-first record

Complete pin cleanup, lock-before-unpin ordering, durable LSN preservation on cleanup failure, and failed-pin retention
were tested first. The initial tests failed because rollback did not invoke unpin-all.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRollbackInvokesDurabilityForOwnID` | Durable rollback restores data, releases locks, and removes the original transaction pin |
| `TestTransactionRollbackReturnsDurableLSNWhenUnpinFails` | Cleanup failure returns the durable LSN, retains the failed pin, and leaves locks released |

This completes Java `Transaction.rollback` ordering: recovery rollback, concurrency release, then buffer unpin-all.

## Feature 74: Reject repeated commit after durable COMMIT

`Transaction` now tracks active and committed lifecycle state. `Commit` changes state immediately after the COMMIT marker
is durable and before lock or pin cleanup. A later commit returns `ErrTransactionCompleted` before buffer or log I/O.
Durability failure leaves the transaction active and retryable; post-durability cleanup failure still leaves it committed.

### Test-first record

Repeated-commit rejection, absence of repeat I/O, retry after pre-durability failure, and terminal state after cleanup
failure were tested first. The initial build failed because `ErrTransactionCompleted` and lifecycle state did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsRepeatedCommitAfterDurability` | A second commit returns the completed error and performs no additional persistence |
| `TestTransactionCommitDurabilityFailureRemainsRetryable` | Failure before durable COMMIT permits a later successful retry |
| `TestTransactionCommitReturnsDurableLSNWhenUnpinFails` | Cleanup failure cannot make an already-durable transaction commit again |

Java relies on callers discarding a completed `Transaction`; Go makes that lifecycle invariant explicit and testable.

## Feature 75: Reject repeated rollback after durable ROLLBACK

The lifecycle state now distinguishes a durable rollback. `Rollback` checks for active state before iterator creation and
marks the transaction rolled back immediately after the ROLLBACK marker is durable, before lock or pin cleanup. A later
rollback returns `ErrTransactionCompleted` without iterator, buffer, log, lock, or pin activity.

Failure before durability leaves the transaction active and permits retry. Post-durability unpin failure remains
terminal because a second ROLLBACK marker would misrepresent the already-completed lifecycle.

### Test-first record

Repeated-rollback rejection, absence of repeat I/O, retry after iterator failure, and terminal state after cleanup failure
were tested first. The initial tests showed that rollback could append multiple terminal markers.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsRepeatedRollbackAfterDurability` | A second rollback returns the completed error before iterator creation or persistence |
| `TestTransactionRollbackFailureKeepsLocksAndPins` | Failure before durability remains retryable and can later complete |
| `TestTransactionRollbackReturnsDurableLSNWhenUnpinFails` | Cleanup failure cannot cause a second ROLLBACK marker |

As with commit, this makes Java's caller-side discard convention an explicit Go lifecycle invariant.

## Feature 76: Reject buffer pin after transaction completion

`Transaction.Pin` now checks active lifecycle state before delegating to `BufferList`. A committed or rolled-back
transaction returns `ErrTransactionCompleted` without invoking the shared buffer manager or recording local ownership.
Failure before commit/rollback durability leaves the transaction active, so pinning remains available for retry handling.

### Test-first record

Commit and rollback completion, absence of buffer-manager calls, unchanged local ownership, and pre-durability retry were
tested first. Both completed-state cases initially acquired a new pin.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsPinAfterCompletion` | Neither committed nor rolled-back transactions can acquire a new buffer pin |
| `TestTransactionPinRemainsAvailableAfterCommitDurabilityFailure` | A failed pre-durability commit does not prematurely disable pinning |

Java relies on completed transactions no longer being used; Go rejects this resource reacquisition explicitly.

## Feature 77: Reject integer reads after transaction completion

`Transaction.GetInt` now validates active lifecycle state before buffer lookup or shared-lock acquisition. Committed and
rolled-back transactions return `ErrTransactionCompleted`, so completion cannot be followed by stale page access or new
lock ownership. Lifecycle validation also takes precedence over the ordinary missing-pin error.

`Unpin` intentionally remains available after completion: a durable commit or rollback whose cleanup failed retains the
failed pin specifically so callers can retry releasing it without reopening data access.

### Test-first record

Commit and rollback completion, error precedence over missing ownership, and absence of shared-lock acquisition were
tested first. Both cases initially returned `ErrBufferNotPinned` and therefore did not enforce lifecycle state.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsGetIntAfterCompletion` | Completed transactions reject integer reads before buffer lookup and lock acquisition |

This makes Java's completed-transaction discard convention explicit while preserving cleanup retry.

## Feature 78: Reject string reads after transaction completion

`Transaction.GetString` now applies the same active-state boundary before buffer lookup or shared-lock acquisition.
Committed and rolled-back transactions return `ErrTransactionCompleted` instead of observing stale page contents or
acquiring new lock ownership. The lifecycle error takes precedence over an absent local pin.

As with integer reads, this guard does not disable `Unpin`: a terminal transaction may still need to retry releasing a
pin retained by failed post-durability cleanup.

### Test-first record

Commit and rollback completion, missing-pin error precedence, and absence of shared-lock acquisition were tested first.
Both cases initially returned `ErrBufferNotPinned` because string reads had no lifecycle validation.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsGetStringAfterCompletion` | Completed transactions reject string reads before buffer lookup and lock acquisition |

String and integer reads now share the same explicit completed-transaction boundary.

## Feature 79: Reject integer writes after transaction completion

`Transaction.SetInt` now checks active lifecycle state before buffer lookup, exclusive-lock acquisition, optional recovery
logging, or page mutation. Both logged and no-log writes return `ErrTransactionCompleted` after COMMIT or ROLLBACK, so
undo-capable and recovery-internal call paths cannot accidentally reopen a terminal transaction.

The completed-state guard leaves retained pins unchanged and keeps `Unpin` callable for cleanup retry.

### Test-first record

Commit and rollback completion were combined with both logging modes. The tests also assert missing-pin error precedence,
no new exclusive lock, and no additional recovery record. All four cases initially returned `ErrBufferNotPinned`.

| Test | Observable behavior |
| --- | --- |
| `TestTransactionRejectsSetIntAfterCompletion` | Completed transactions reject logged and no-log integer writes before side effects |

This adds the write-side lifecycle boundary without changing active transaction write semantics.

## Next behavior

Continue with Chapter 5 by rejecting string writes after transaction completion.

## Sources

- Edward Sciore, *Database Design and Implementation*, 2nd ed., Chapter 5, Sect. 5.3.9.1
- SimpleDB 3.4 Java source: `simpledb.tx.recovery.LogRecord`, `CheckpointRecord`
