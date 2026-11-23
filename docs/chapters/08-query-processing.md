# Chapter 8: Query Processing

This chapter introduces typed database values and composes them into scans and relational operators in the book's
dependency order. Each observable behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/query.Constant` | `simpledb/query/Constant.java` | Typed values read, compared, and written by query scans |

## Feature 1: Initialize an integer query constant

`NewIntConstant` constructs the integer form of a database value from an `int32`, matching the width used by the record
and transaction layers. `AsInt` returns the exact value and a successful type result; `AsString` returns the zero string
and `false`, so callers never trigger Java's null-unboxing failure by requesting the wrong representation.

The Go value uses an explicit private kind tag instead of Java's pair of nullable object fields. It performs no record,
transaction, or storage I/O.

### Test-first record

The test first failed to compile because `NewIntConstant` and the typed accessors did not exist.

| Test | Observable behavior |
| --- | --- |
| `TestNewIntConstantRetainsOnlyIntegerValue` | An integer constant returns its exact `int32` value and reports that no string representation is stored |

## Feature 2: Initialize a string query constant

`NewStringConstant` and `AsString` map to Java `Constant(String)` and `asString`. The kind tag distinguishes a stored
empty string from an unavailable string value. `AsString` returns the supplied string unchanged, and `AsInt` returns
`(0, false)` for a string constant.

`TestNewStringConstantRetainsOnlyStringValue` first failed because the constructor was undefined. It verifies
both a non-ASCII string and the empty string through the typed accessors.

## Feature 3: Test query constants for equality

`Constant.Equal` maps to Java `Constant.equals`. Constructors set the kind and populate only the corresponding
value field, so comparing initialized Go structs checks both kind and value. The zero-value constant is unset
and never matches. This gives callers a defined result for an input the Java constructors do not create.

`TestConstantEqual` first failed to compile because the method was absent. It covers equal and unequal values
of each kind, mixed kinds, and unset values in both operand positions.
