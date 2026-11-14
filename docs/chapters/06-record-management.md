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
| future table scan | `simpledb/record/TableScan.java` | Multi-block record iteration and updates |

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

## Next behavior

Expose the record identifier's slot number as the next stacked PR.
