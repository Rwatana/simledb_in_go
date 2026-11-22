# Chapter 8: Query Processing

This chapter introduces typed database values and composes them into scans and relational operators in the book's
dependency order. Each observable behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/query.Constant` | `simpledb/query/Constant.java` | Typed values read, compared, and written by query scans |
| `internal/query.Expression` | `simpledb/query/Expression.java` | Literal values and field references used by relational operators |
| `internal/query.Term` | `simpledb/query/Term.java` | Equality between two expressions |

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

## Feature 4: Order query constants of the same kind

`Constant.Compare` maps to Java `Constant.compareTo`. It compares signed integers directly, which avoids overflow
at the limits of `int32`. Strings use Go's lexicographic byte order. Java uses UTF-16 code units, so some
supplementary characters sort differently. Mixed kinds and unset values return `(0, false)`.

`TestConstantCompare` first failed because the method was undefined. It verifies integer limits, string prefixes,
equality, reversed operands, and incomparable values.

## Feature 5: Format a query constant as text

`Constant.String` maps to Java `Constant.toString`. It produces decimal text for integers and the original,
unquoted text for strings. The Go zero value renders as `<unset>` for diagnostics. The output is for display
and carries no SQL quoting or escaping.

`TestConstantString` first failed because the method was undefined. It verifies the minimum integer,
zero, non-ASCII text, the empty string, and the unset representation.

## Feature 6: Hash a query constant deterministically

`Constant.HashCode` maps to Java `Constant.hashCode`. An integer returns its signed 32-bit value. For strings,
each UTF-16 code unit contributes to the Java multiply-by-31 recurrence with signed 32-bit wraparound. This
gives the later hash index a repeatable result across database sessions. Go range decoding supplies replacement
runes for malformed UTF-8. An unset constant reports that no hash is available.

`TestConstantHashCode` first failed because the method was undefined. It covers an integer, an empty string,
ASCII, a BMP character, a supplementary character, signed overflow, malformed UTF-8, and the unset value.

## Feature 7: Construct a literal query expression

`NewConstantExpression` and `AsConstant` implement the literal constructor and accessor from Java `Expression(Constant)`
and `asConstant`. A separate presence bit preserves the difference between an expression containing a value and a
zero-value expression. `IsFieldName` is false for this literal form. Field references and evaluation against a scan
remain separate behaviors.

`TestLiteralExpressionReturnsItsConstant` initially failed because the constructor and type did not exist.
`TestZeroValueExpressionHasNoConstant` checks the absent-literal result.

## Feature 8: Construct a field reference expression

`NewFieldExpression`, `IsFieldName`, and `AsFieldName` map to Java `Expression(String)`, `isFieldName`, and
`asFieldName`. A separate presence bit preserves the distinction between an absent name and a supplied empty
name. A field expression has no literal constant. Scan evaluation and schema applicability remain separate
behaviors.

`TestFieldExpressionReturnsItsName` initially failed because the field constructor and accessor did not exist.
`TestZeroValueExpressionHasNoFieldName` checks the absent field result.

## Feature 9: Evaluate query expressions against a scan

`Expression.Evaluate` maps to Java `Expression.evaluate`. A literal returns its value without scan access. A field
reference asks the minimal `ValueScan` boundary for exactly that field and preserves lookup errors. The consumer
defines this small interface, so query expressions do not depend on a concrete record scan. An unset expression and
a field expression with a nil scan return contextual errors.

The tests first failed because `Evaluate` and `ValueScan` did not exist. They verify literal short-circuiting,
one field lookup, unchanged lookup-error propagation, the contextual error for a field expression with a nil scan, and
the contextual error for an unset expression without consulting the scan.

## Feature 10: Check expression applicability to a schema

`Expression.AppliesTo` maps to Java `Expression.appliesTo(Schema)`. A literal is independent of the schema and always
applies. A field reference delegates membership to `record.Schema.HasField`. Go returns `false` for an unset
expression or a nil schema rather than allowing a nil dereference; a literal still applies when the schema is nil.
The method takes the existing `*record.Schema` type because the query layer already relies on record values and this
operation needs precisely its membership behavior.

`TestExpressionAppliesToSchema` first failed to compile because `AppliesTo` was undefined. It verifies literal,
present and absent field, empty-name field, nil-schema, and zero-value expression cases.

## Feature 11: Evaluate a query equality term

`NewTerm` and `Term.IsSatisfied` map to Java `Term(Expression, Expression)` and `Term.isSatisfied(Scan)`. The term
evaluates its left expression before its right expression and compares the resulting constants with `Constant.Equal`.
This keeps scan access behind the existing `ValueScan` boundary. Go returns lookup errors unchanged; if the left side
fails, the right side is not evaluated.

The tests first failed because `NewTerm` was undefined. They cover equal and unequal literal values, mixed value kinds,
left-to-right field lookup, and propagation of either evaluation error without evaluating later expressions.

## Feature 12: Match a term field to a constant

`Term.EquatesWithConstant` maps to Java `Term.equatesWithConstant`. It returns the literal when exactly one side
references the requested field and the other side is a literal, regardless of operand order. Go uses the expression's
presence result for constants, so even a literal containing the zero-value `Constant` remains distinguishable from an
absent expression. A field mismatch, two fields, two literals, or an unset expression returns `(Constant{}, false)`.

`TestTermEquatesWithConstant` first failed because the method was undefined. It covers both operand orders, the empty
field name, a present zero-value literal, and non-matching term forms.
