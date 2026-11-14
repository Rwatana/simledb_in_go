# Chapter 6: Record Management

This chapter builds typed schemas, physical layouts, record pages, and table scans in the book's dependency order.
Each observable behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/record.Schema` | `simpledb/record/Schema.java` | Ordered logical field declarations |
| `internal/record.Layout` | `simpledb/record/Layout.java` | Physical field offsets and slot size |
| `internal/record.RecordPage` | `simpledb/record/RecordPage.java` | Fixed-slot page access |
| `internal/record.RID` | `simpledb/record/RID.java` | Stable block-and-slot record identity |
| `internal/record.TableScan` | `simpledb/record/TableScan.java` | Multi-block record iteration and updates |

## Feature 1: Register a generic schema field

`Schema.AddField` records a field name, a Go-native `FieldType`, and its conceptual length while preserving declaration
order. `IntegerField` and `VarcharField` replace Java SQL type integers with a closed named type that can be extended only
deliberately.

The zero value initializes its definition map on the first registration. `Fields` returns a copy rather than Java's
mutable list, while `Field` returns the registered definition plus an existence flag instead of dereferencing a missing
map entry.

### Test-first record

The first test failed to compile because the record package, `Schema`, and field types did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaAddFieldPreservesOrderAndMetadata` | Generic field registration preserves declaration order and type/length metadata |

## Intentional differences from Java

- Field types use the named `FieldType` constants instead of `java.sql.Types` integers.
- Schema field lists are returned as copies, preventing callers from mutating declaration order out of band.
- Missing field lookup is explicit through a boolean result.

## Feature 2: Register an integer field

`Schema.AddIntegerField` delegates to generic registration with `IntegerField` and conceptual length zero. It preserves
the same declaration-order semantics when mixed with fields registered through `AddField`.

### Test-first record

The test first failed to compile because the integer convenience method did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaAddIntegerFieldUsesIntegerMetadata` | Integer convenience registration records integer type, zero length, and declaration order |

This maps Java `Schema.addIntField` to a Go name that spells out `Integer` while keeping one metadata path.

## Feature 3: Register a varchar field

`Schema.AddVarcharField` delegates to generic registration with `VarcharField` and the caller's conceptual character
length. It preserves declaration order when mixed with integer or generic fields.

### Test-first record

The test first failed to compile because the varchar convenience method did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaAddVarcharFieldUsesSuppliedLength` | Varchar convenience registration records varchar type, supplied length, and declaration order |

This maps Java `Schema.addStringField` to terminology that matches the logical varchar type.

## Feature 4: Copy one field from another schema

`Schema.AddFrom` looks up one named field in a source schema, copies its type and conceptual length through the generic
registration path, and appends the name to the destination declaration order. It reports whether the source field exists;
a missing name leaves the destination unchanged instead of causing Java's implicit null dereference.

### Test-first record

The test first failed to compile because the field-copy operation did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaAddFromCopiesOneFieldDefinition` | One named source field preserves type and length in destination order, while a missing field is reported without mutation |

This maps Java `Schema.add` to a Go operation with an explicit missing-field result.

## Feature 5: Merge every field from another schema

`Schema.AddAllFrom` walks the source's copied field-name slice in declaration order and delegates each entry to
`AddFrom`. Existing destination fields remain first, and all copied type and conceptual-length metadata follows the
single-field path.

### Test-first record

The test first failed to compile because the all-field merge operation did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaAddAllFromPreservesSourceOrderAndMetadata` | All source fields retain order, type, and length after existing destination fields |

This maps Java `Schema.addAll` while retaining the immutable field-list boundary used by the Go schema.

## Feature 6: Check field membership

`Schema.HasField` performs a read-only definition lookup and reports whether a name is declared. It works for every
field type without exposing either the internal map or the declaration-order slice.

### Test-first record

The test first failed to compile because the membership operation did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaHasFieldReportsMembership` | Declared integer and varchar names are present, while an unknown name is absent |

This maps Java `Schema.hasField` directly to a Go boolean query.

## Feature 7: Read a field's logical type

`Schema.TypeOf` delegates to the definition lookup and returns the registered `FieldType` plus an existence flag. The
flag keeps unknown names explicit instead of relying on Java's implicit null dereference.

### Test-first record

The test first failed to compile because the type accessor did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaTypeOfReportsLogicalType` | Integer and varchar fields return their logical type, while an unknown name is absent |

This maps Java `Schema.type` to a Go accessor with explicit presence.

## Feature 8: Read a field's conceptual length

`Schema.LengthOf` delegates to the definition lookup and returns the stored conceptual length plus an existence flag.
Integer fields retain the generic registration value zero, while varchar fields return their configured character limit.

### Test-first record

The test first failed to compile because the length accessor did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestSchemaLengthOfReportsConceptualLength` | Integer and varchar fields return their stored length, while an unknown name is absent |

This maps Java `Schema.length` to a Go accessor with explicit presence.

## Feature 9: Calculate physical field offsets

`NewLayout` reserves four bytes at the start of each record slot for the empty/in-use flag, then walks a schema in
declaration order. Integer fields occupy four bytes. Varchar fields reserve a four-byte payload-length prefix plus four
bytes per conceptual character, which is the maximum width of one UTF-8 code point in Go.

`Layout.Offset` returns the calculated byte position plus an existence flag, keeping unknown field names explicit.
Slot-size access and catalog-metadata reconstruction remain separate behaviors.

### Test-first record

The test first failed to compile because `NewLayout` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLayoutCalculatesOrderedFieldOffsets` | Flag space, integer width, maximum UTF-8 varchar width, declaration order, and unknown lookup determine field offsets |

This maps Java `Layout(Schema)`, `lengthInBytes`, and `offset`. Java asks its configured charset encoder for maximum
bytes per character; Go uses the fixed `utf8.UTFMax` bound for its explicit UTF-8 representation.

## Feature 10: Expose the record slot size

`Layout.SlotSize` returns the offset calculation's final byte position: the four-byte empty/in-use flag followed by all
encoded field widths. An empty schema therefore still occupies four bytes so each slot can carry its occupancy state.

### Test-first record

The test first failed to compile because the slot-size accessor did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLayoutSlotSizeIncludesFlagAndEncodedFields` | Mixed fields include the flag and encoded widths, while an empty schema retains only the flag |

This maps Java `Layout.slotSize` without exposing mutable layout state.

## Feature 11: Expose the layout's logical schema

`Layout.Schema` returns the schema retained during construction so record and metadata code can inspect field order and
definitions through the schema's existing APIs. Access does not recalculate physical offsets or slot size.

As in Java `Layout.schema`, callers must treat the associated schema as stable after layout construction. Adding fields
later would not retroactively change the already-calculated physical layout.

### Test-first record

The test first failed to compile because the schema accessor did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLayoutExposesAssociatedSchema` | The layout returns its construction schema with the same field declarations |

This maps Java `Layout.schema` directly while documenting the mutation boundary explicitly.

## Feature 12: Reconstruct a layout from catalog metadata

`NewLayoutFromMetadata` accepts an associated schema, precomputed field offsets, and a precomputed slot size. It retains
those physical values exactly rather than rerunning schema-based sizing, matching the path used after catalog reads.

The Go constructor copies the offset map, so later mutation by the catalog assembly code cannot silently alter an
already-created layout. Validation of inconsistent catalog metadata remains outside this behavior.

### Test-first record

The test first failed to compile because the metadata constructor did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestLayoutFromMetadataPreservesPhysicalDescription` | Supplied schema, non-derived offsets, and slot size are preserved, and caller map mutation is isolated |

This maps Java `Layout(Schema, Map<String,Integer>, int)` with defensive map ownership.

## Feature 13: Initialize a pinned record page

`NewRecordPage` accepts a narrow transaction boundary, block identifier, and layout. It pins the block exactly once
before returning a usable `RecordPage`; a pin failure returns no page and wraps the block context.

The record page retains these dependencies for later field operations. This PR does not read or mutate any record bytes,
format the block, search slots, or define unpin ownership.

### Test-first record

The tests first failed to compile because `NewRecordPage` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewRecordPagePinsTargetBlock` | Successful initialization pins the requested block exactly once and returns a page |
| `TestNewRecordPageReturnsPinFailure` | Pin failure returns no page and retains the cause plus block context |

This maps the pinning boundary of Java `RecordPage(Transaction, BlockId, Layout)` without mixing in field access.

## Feature 14: Read an integer field from a record slot

`RecordPage.GetInt` resolves the field offset, adds it to `slot * Layout.SlotSize()`, and asks the retained transaction to
read exactly one integer from the retained block. An unknown field stops before transaction access. Read failures retain
the block, slot, field name, and calculated byte position while wrapping the transaction cause.

### Test-first record

The tests first failed to compile because `RecordPage.GetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageGetIntReadsCalculatedFieldPosition` | Slot and layout metadata select one transaction integer read and return its value |
| `TestRecordPageGetIntReportsLookupAndReadFailures` | Unknown fields prevent reads, while transaction failures retain complete record context |

This maps Java `RecordPage.getInt` and makes its implicit map/transaction failures explicit Go errors.

## Feature 15: Read a string field from a record slot

`RecordPage.GetString` uses the same slot and field offset calculation as integer reads, then performs one transaction
string read from the retained block. UTF-8 values pass through unchanged. Unknown fields stop before transaction access,
and read failures retain the full physical record context.

### Test-first record

The tests first failed to compile because `RecordPage.GetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageGetStringReadsCalculatedFieldPosition` | Slot and layout metadata select one transaction string read and preserve UTF-8 |
| `TestRecordPageGetStringReportsLookupAndReadFailures` | Unknown fields prevent reads, while transaction failures retain record context |

This maps Java `RecordPage.getString` with explicit Go error boundaries.

## Feature 16: Write an integer field in a record slot

`RecordPage.SetInt` calculates the slot-relative field position and performs exactly one transaction integer write with
logging enabled. Unknown fields and positions outside the transaction's signed 32-bit offset contract stop before the
write. Transaction failures retain the block, slot, field, and byte position.

### Test-first record

The tests first failed to compile because `RecordPage.SetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageSetIntWritesCalculatedFieldPositionWithLogging` | One logged transaction write receives the retained block, calculated position, and value |
| `TestRecordPageSetIntRejectsInvalidPositionAndWrapsFailures` | Unknown fields and offset overflow prevent writes, while transaction failures retain context |

This maps Java `RecordPage.setInt`; the explicit range guard prevents Go `int` to `int32` truncation.

## Feature 17: Write a string field in a record slot

`RecordPage.SetString` calculates the slot-relative field position and performs exactly one logged transaction string
write. UTF-8 values pass through unchanged. Unknown fields and signed 32-bit offset overflow stop before transaction
access; write failures retain the complete physical record context.

### Test-first record

The tests first failed to compile because `RecordPage.SetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageSetStringWritesUTF8AtCalculatedPositionWithLogging` | One logged write receives the retained block, calculated position, and unchanged UTF-8 value |
| `TestRecordPageSetStringRejectsInvalidPositionAndWrapsFailures` | Unknown fields and offset overflow prevent writes, while transaction failures retain context |

This maps Java `RecordPage.setString` with explicit range and error handling.

## Feature 18: Delete a record slot

`RecordPage.Delete` calculates the slot's leading occupancy-flag position and performs one logged integer write of zero.
It does not clear field payload bytes; subsequent scans ignore the slot because its flag is empty. Signed 32-bit offset
overflow stops before transaction access, and write failures retain block, slot, and position context.

### Test-first record

The tests first failed to compile because `RecordPage.Delete` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageDeleteWritesEmptyFlagWithLogging` | Deletion writes one logged zero at the selected slot's flag position |
| `TestRecordPageDeleteRejectsInvalidPositionAndWrapsFailures` | Offset overflow prevents access, while transaction failure retains physical context |

This maps Java `RecordPage.delete` and its private `setFlag(slot, EMPTY)` behavior.

## Feature 19: Format a new record block

`RecordPage.Format` obtains the transaction block size once, then initializes every complete fixed-size slot in physical
order. Each slot receives an empty occupancy flag followed by zero for integer fields and an empty string for varchar
fields in schema declaration order. Every write disables update logging because a newly appended block has no meaningful
old values. An incomplete trailing region is left untouched.

Unlike the Java method, the Go method rejects non-positive slot sizes and returns contextual errors for block-size lookup,
layout lookup, offset overflow, and the first failed flag or field write.

### Test-first record

The tests first failed to compile because `RecordPage.Format` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageFormatInitializesCompleteSlotsWithoutLogging` | Complete slots are initialized in schema order without logging while trailing bytes remain untouched |
| `TestRecordPageFormatRejectsInvalidLayoutAndWrapsFailures` | Invalid layout and transaction failures stop formatting with physical context |

This maps Java `RecordPage.format` and its `isValidSlot` boundary rule.

## Feature 20: Find the next occupied record slot

`RecordPage.NextAfter` scans occupancy flags from the slot immediately after the caller's position and returns the first
slot marked used. Passing `-1` begins at slot zero. Only complete slots are read, so trailing bytes smaller than one slot
are ignored, and exhaustion returns `-1` without modifying the page.

The Go API rejects starting positions before `-1` and non-positive slot sizes, and it returns contextual block-size and
flag-read failures instead of relying on unchecked transaction calls.

### Test-first record

The tests first failed to compile because `RecordPage.NextAfter` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageNextAfterFindsOccupiedSlotWithinCompleteSlots` | Search starts after the caller's slot, returns the first used flag, and ignores incomplete trailing bytes |
| `TestRecordPageNextAfterRejectsInvalidInputAndWrapsFailures` | Invalid starts/layouts and transaction failures return explicit context |

This maps Java `RecordPage.nextAfter` and the used-flag branch of `searchAfter`.

## Feature 21: Find the next empty record slot

`RecordPage.NextEmptyAfter` uses the same complete-slot boundary and ordered flag scan as `NextAfter`, but targets the
empty occupancy value. It returns the first reusable slot or `-1` at exhaustion and performs no writes, keeping slot
selection separate from the later reservation step.

### Test-first record

The tests first failed to compile because `RecordPage.NextEmptyAfter` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageNextEmptyAfterFindsEmptySlotWithoutWriting` | The first empty flag is returned without writes, and incomplete trailing bytes are ignored |
| `TestRecordPageNextEmptyAfterWrapsFlagReadFailure` | A flag-read failure retains block, slot, and byte-position context |

This maps the empty-flag branch of Java `RecordPage.searchAfter`. The separate Go method intentionally exposes selection
without combining it with the mutation performed by Java `insertAfter`.

## Feature 22: Mark a record slot occupied

`RecordPage.Occupy` changes only the selected slot's leading occupancy flag to the used value. It performs exactly one
logged integer write, without reading the flag or touching field payload bytes. Signed 32-bit position overflow stops
before transaction access, and write failures retain block, slot, and byte-position context.

### Test-first record

The tests first failed to compile because `RecordPage.Occupy` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageOccupyWritesUsedFlagWithLogging` | One logged used-flag write reserves the selected slot without extra access |
| `TestRecordPageOccupyRejectsInvalidPositionAndWrapsFailures` | Offset overflow prevents access, while transaction failure retains physical context |

This maps Java `RecordPage.setFlag(slot, USED)`. The explicit Go operation keeps reservation separate from empty-slot
selection so each stacked PR has one observable behavior.

## Feature 23: Insert into the next empty record slot

`RecordPage.InsertAfter` composes the two independently tested operations: it selects the next empty complete slot, then
marks that slot used with one logged flag write. Exhaustion returns `-1` without a write. Search and reservation failures
also return `-1`, retaining both the composition step and the underlying physical context.

### Test-first record

The tests first failed to compile because `RecordPage.InsertAfter` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageInsertAfterOccupiesNextEmptySlot` | The first empty slot is selected, marked used, and returned |
| `TestRecordPageInsertAfterReturnsNoSlotWithoutWriting` | A full page returns `-1` without modifying flags |
| `TestRecordPageInsertAfterPropagatesSearchAndOccupyFailures` | Search and reservation failures do not report successful insertion |

This maps Java `RecordPage.insertAfter`. The Go implementation makes its selection and mutation composition explicit.

## Feature 24: Expose the record page block identity

`RecordPage.Block` returns the immutable `storage.BlockID` value retained when the page was initialized. It performs no
transaction operation, so callers can identify or later release the page without introducing an additional storage read.

### Test-first record

The test first failed to compile because `RecordPage.Block` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRecordPageBlockReturnsRetainedIdentityWithoutTransactionAccess` | The original block identity is returned as a value without transaction access |

This maps Java `RecordPage.block`.

## Feature 25: Expose a record identifier block number

`RID` is a small Go value containing private block and slot coordinates. `NewRID` constructs both coordinates, while
`BlockNumber` exposes exactly the retained block number without allowing callers to mutate identifier state directly.

### Test-first record

The test first failed to compile because `NewRID` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRIDBlockNumberReturnsConstructedValue` | A constructed RID returns its exact block number |

This maps Java `RID.RID` and `RID.blockNumber`; slot access remains a separate behavior.

## Feature 26: Expose a record identifier slot number

`RID.Slot` returns the exact slot coordinate retained by `NewRID`, while the private fields keep the identifier immutable
to callers.

### Test-first record

The test first failed to compile because `RID.Slot` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRIDSlotReturnsConstructedValue` | A constructed RID returns its exact slot number |

This maps Java `RID.slot`.

## Feature 27: Compare record identifiers

`RID.Equal` compares both private coordinates and returns true only for the same block number and slot. Its typed `RID`
parameter avoids Java's unchecked cast from `Object` while preserving value semantics.

### Test-first record

The test first failed to compile because `RID.Equal` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRIDEqualComparesBothCoordinates` | Equality requires matching block and slot coordinates |

This maps Java `RID.equals` with a type-safe Go signature.

## Feature 28: Format a record identifier

`RID.String` implements `fmt.Stringer` and renders the retained coordinates exactly as `[blockNumber, slot]`, including
signed values, without additional labels.

### Test-first record

The test first failed to compile because `RID.String` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestRIDStringFormatsCoordinatePair` | RID text contains the exact block and slot coordinate pair |

This maps Java `RID.toString`. Java `RID` and the Go value now have matching public behavior.

## Feature 29: Initialize a scan for an existing table

`NewTableScan` derives the `.tbl` filename, reads its size once, and for a non-empty file creates a `RecordPage` pinned to
block zero. The cursor begins at slot `-1`, matching the before-first position used by later iteration. Size and pin errors
retain table context. This feature isolated the non-empty branch before the following empty-file behavior was added.

### Test-first record

The tests first failed to compile because `NewTableScan` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewTableScanPinsFirstBlockOfExistingTable` | One size lookup and one block-zero pin initialize the before-first cursor |
| `TestNewTableScanReportsSizeEmptyAndPinFailures` | Size, empty-file, and pin failures return explicit table context |

This maps the existing-file branch of Java `TableScan.TableScan`.

## Feature 30: Initialize a scan for an empty table

When the initial table size is zero, `NewTableScan` appends exactly one first block, creates a pinned `RecordPage` for the
returned block, and formats every complete slot without logging. The cursor then begins at `-1`, exactly like the existing
table path. Append, pin, and format failures stop the sequence and retain table context.

### Test-first record

The tests first failed because the zero-size path still returned the explicit deferred error from Feature 29.

| Test | Observable behavior |
| --- | --- |
| `TestNewTableScanAppendsPinsAndFormatsEmptyTable` | Empty initialization appends, pins, no-log formats, and positions the before-first cursor |
| `TestNewTableScanReportsEmptyTableAppendPinAndFormatFailures` | Each failed initialization stage stops later work and retains context |

This maps the empty-file branch of Java `TableScan.TableScan` and its call to `moveToNewBlock`.

## Feature 31: Close a table scan

`TableScan.Close` unpins the block retained by the current record page. A successful release clears the page reference,
making repeated closes no-op successes. An unpin failure retains the page for retry and returns table and block context.
Closing an already empty scan performs no transaction access.

### Test-first record

The tests first failed to compile because `TableScan.Close` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanCloseUnpinsCurrentBlockOnce` | A successful close unpins once and subsequent closes are idempotent |
| `TestTableScanCloseRetainsPageForUnpinRetry` | Failed unpin retains the page and a later close retries it |
| `TestTableScanCloseWithoutCurrentPageDoesNothing` | No current page produces a no-op success |

This maps Java `TableScan.close` while adding explicit error propagation, idempotency, and cleanup retry behavior.

## Feature 32: Rewind a table scan before its first record

`TableScan.BeforeFirst` closes the current page, pins a new record page for block zero, and resets the cursor to `-1` only
after the replacement page is ready. A close failure prevents the new pin and preserves the current page. A repin failure
after a successful close leaves no current page and does not claim the cursor was reset.

### Test-first record

The tests first failed to compile because `TableScan.BeforeFirst` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanBeforeFirstRepinsBlockZeroAndResetsCursor` | Rewind releases the current page, repins block zero, and restores before-first position |
| `TestTableScanBeforeFirstStopsOnCloseFailure` | A failed release stops before repinning and retains current state |
| `TestTableScanBeforeFirstReportsRepinFailureAfterClose` | A failed block-zero pin leaves the scan without a current page and retains cursor state |

This maps Java `TableScan.beforeFirst` and `moveToBlock(0)` with explicit sequencing errors.

## Feature 33: Advance within a table scan block

`TableScan.NextInBlock` asks the current record page for the next occupied slot after the cursor. A match updates the
cursor and returns true. Current-block exhaustion records slot `-1` and returns false without moving blocks. A flag-read
failure preserves the previous cursor and adds table context, while a scan without a current page is rejected explicitly.

### Test-first record

The tests first failed to compile because `TableScan.NextInBlock` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanNextInBlockAdvancesToOccupiedSlotsAndReportsExhaustion` | Occupied slots advance the cursor and current-block exhaustion resets it to `-1` |
| `TestTableScanNextInBlockPreservesCursorOnReadFailure` | Flag-read failure preserves the previous cursor and returns table context |
| `TestTableScanNextInBlockRejectsClosedScan` | A scan without a current page cannot advance |

This isolates the current-block portion of Java `TableScan.next` before cross-block traversal is added.

## Feature 34: Detect the last table scan block

`TableScan.AtLastBlock` requires a current record page, reads the retained table filename's size once, and compares the
current block number with `size - 1`. Size failures retain table and block context, and a non-positive size while a page
is current is rejected as inconsistent state.

### Test-first record

The tests first failed to compile because `TableScan.AtLastBlock` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanAtLastBlockComparesCurrentBlockWithFileSize` | Current block and one size lookup determine first-versus-last status |
| `TestTableScanAtLastBlockRejectsUnavailableStateAndWrapsSizeFailure` | Closed, inconsistent, and failed-size states return explicit context |

This maps Java `TableScan.atLastBlock` with explicit error handling.

## Feature 35: Move a table scan to the next block

`TableScan.MoveToNextBlock` captures the current block number, closes that page, and pins the next numbered block for
the same table file. The cursor becomes `-1` only after the replacement page is ready. A close failure leaves the old
page and cursor available for retry; a subsequent pin failure leaves the scan closed while preserving the old cursor.

### Test-first record

The tests first failed to compile because `TableScan.MoveToNextBlock` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanMoveToNextBlockReleasesCurrentPageAndResetsCursor` | Moving releases block zero, pins block one, and restores the before-first cursor |
| `TestTableScanMoveToNextBlockStopsOnCloseFailure` | A failed release stops before the next pin and retains current state |
| `TestTableScanMoveToNextBlockReportsPinFailureAfterClose` | A failed next-block pin leaves no current page and preserves cursor state |
| `TestTableScanMoveToNextBlockRejectsClosedScan` | A scan without a current page cannot derive or pin a next block |

This maps Java `TableScan.moveToBlock(rp.block().number()+1)` while keeping release and pin failures explicit.

## Feature 36: Iterate a table scan across existing blocks

`TableScan.Next` first searches the current record page. On exhaustion it checks whether that page is final, and when
more pages exist it moves forward and repeats. This composition skips any number of empty intermediate blocks, returns
the first occupied slot found, and leaves the final page pinned with cursor `-1` when the table is exhausted.

### Test-first record

The tests first failed to compile because `TableScan.Next` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanNextSkipsEmptyBlocksAndFindsTheNextRecord` | Iteration crosses empty pages, returns the first occupied slot, then stops on final exhaustion |
| `TestTableScanNextStopsAtAnEmptyLastBlock` | An empty final page ends iteration without moving or unpinning it |
| `TestTableScanNextWrapsTraversalFailure` | A failed last-page check retains its cause and identifies the block being left |
| `TestTableScanNextRejectsClosedScan` | A scan without a current page cannot begin iteration |

This completes Java `TableScan.next` by composing the separately tested current-page search, final-page check, and
`moveToBlock(rp.block().number()+1)` behaviors.

## Feature 37: Read an integer field from a table scan

`TableScan.GetInt` requires an open page and a cursor positioned on a record, then delegates one read to the current
record page with that slot. Successful and failed reads preserve scan position; failures add table, block, slot, and
field context around the record-page error.

### Test-first record

The tests first failed to compile because `TableScan.GetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanGetIntReadsTheCurrentRecordField` | The cursor slot and layout produce one integer read and the exact stored value |
| `TestTableScanGetIntWrapsFieldAndReadFailures` | Lookup and storage failures retain their causes and scan coordinates without moving the cursor |
| `TestTableScanGetIntRequiresACurrentRecord` | Before-first and closed scans reject reads before transaction access |

This maps Java `TableScan.getInt` while making invalid cursor state and read errors explicit.

## Feature 38: Read a string field from a table scan

`TableScan.GetString` requires an open page and a cursor positioned on a record, then delegates one string read to the
current record page. UTF-8 content is returned unchanged. Successful and failed reads preserve scan position, and
failures add table, block, slot, and field context around the record-page error.

### Test-first record

The tests first failed to compile because `TableScan.GetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanGetStringReadsTheCurrentRecordField` | The cursor slot and layout produce one string read and the exact stored UTF-8 value |
| `TestTableScanGetStringWrapsFieldAndReadFailures` | Lookup and storage failures retain their causes and scan coordinates without moving the cursor |
| `TestTableScanGetStringRequiresACurrentRecord` | Before-first and closed scans reject reads before transaction access |

This maps Java `TableScan.getString` while making invalid cursor state and read errors explicit.

## Feature 39: Check table scan field membership

`TableScan.HasField` asks the schema retained by its layout whether a field is declared. Membership is layout metadata,
so it remains available before the first record and after the scan's current page is closed, without transaction access.

### Test-first record

The test first failed to compile because `TableScan.HasField` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanHasFieldUsesTheRetainedLayoutWithoutARecord` | Registered integer and string fields are found, missing fields are rejected, and closing the page does not affect metadata |

This maps Java `TableScan.hasField` directly to the immutable layout/schema metadata retained by the Go scan.

## Feature 40: Write an integer field through a table scan

`TableScan.SetInt` requires an open page and a cursor positioned on a record, then delegates one logged integer write to
the current record page. Both successful and failed writes preserve scan position, and failures add table, block, slot,
and field context around the record-page error.

### Test-first record

The tests first failed to compile because `TableScan.SetInt` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanSetIntWritesTheCurrentRecordFieldWithLogging` | The cursor slot and layout produce one logged write with the exact signed value |
| `TestTableScanSetIntWrapsFieldAndWriteFailures` | Lookup and storage failures retain their causes and scan coordinates without moving the cursor |
| `TestTableScanSetIntRequiresACurrentRecord` | Before-first and closed scans reject writes before transaction access |

This maps Java `TableScan.setInt`; the existing record-page boundary preserves transaction logging for the update.

## Feature 41: Write a string field through a table scan

`TableScan.SetString` requires an open page and a cursor positioned on a record, then delegates one logged string write
to the current record page. UTF-8 content is forwarded unchanged. Both successful and failed writes preserve scan
position, and failures add table, block, slot, and field context around the record-page error.

### Test-first record

The tests first failed to compile because `TableScan.SetString` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanSetStringWritesTheCurrentRecordFieldWithLogging` | The cursor slot and layout produce one logged write with the exact UTF-8 value |
| `TestTableScanSetStringWrapsFieldAndWriteFailures` | Lookup and storage failures retain their causes and scan coordinates without moving the cursor |
| `TestTableScanSetStringRequiresACurrentRecord` | Before-first and closed scans reject writes before transaction access |

This maps Java `TableScan.setString`; the existing record-page boundary preserves transaction logging for the update.

## Feature 42: Insert within the current table scan block

`TableScan.InsertInBlock` asks the current record page to find and occupy the next empty slot after the cursor. A match
becomes the new cursor and returns true. Current-block exhaustion sets the cursor to `-1` and returns false without
changing pages. Search or occupation failure preserves the previous cursor, and occupation remains transaction-logged.

### Test-first record

The tests first failed to compile because `TableScan.InsertInBlock` did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestTableScanInsertInBlockOccupiesTheNextEmptySlot` | Search skips an occupied slot, logs the selected flag update, and moves the cursor |
| `TestTableScanInsertInBlockReportsCurrentBlockExhaustion` | No later empty slot resets the cursor without changing blocks or writing |
| `TestTableScanInsertInBlockPreservesCursorOnSearchOrOccupyFailure` | Search and logged occupation failures retain their causes and the previous cursor |
| `TestTableScanInsertInBlockRejectsClosedScan` | A scan without a current page cannot insert |

This isolates the current-block portion of Java `TableScan.insert` before existing-block movement and new-block
rollover are added.

## Next behavior

Append and format a new table block for insertion rollover.
