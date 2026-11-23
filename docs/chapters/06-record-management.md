# Chapter 6: Record Management

This chapter builds typed schemas, physical layouts, record pages, and table scans in the book's dependency order.
Each observable behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/record.Schema` | `simpledb/record/Schema.java` | Ordered logical field declarations |
| future `Layout` | `simpledb/record/Layout.java` | Physical field offsets and slot size |
| future record page | `simpledb/record/RecordPage.java` | Fixed-slot page access |
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

## Next behavior

Write one string field in a specified record slot as the next stacked PR.
