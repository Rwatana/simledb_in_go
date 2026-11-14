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

## Next behavior

Add varchar-field convenience registration as the next stacked PR.
