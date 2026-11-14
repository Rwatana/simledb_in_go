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

## Next behavior

Add conceptual-length access as the next stacked PR.
