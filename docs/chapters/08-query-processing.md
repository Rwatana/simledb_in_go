# Chapter 8: Query Processing

This chapter introduces typed database values and composes them into scans and relational operators in the book's
dependency order. Each observable behavior remains a separate Issue, branch, commit, and stacked Pull Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/query.Constant` | `simpledb/query/Constant.java` | Typed values read, compared, and written by query scans |
| `internal/query.Expression` | `simpledb/query/Expression.java` | Literal values and field references used by relational operators |
| `internal/query.Term` | `simpledb/query/Term.java` | Equality between two expressions |
| `internal/query.Scan` | `simpledb/query/Scan.java` | Shared traversal, field access, and cleanup contract |
| `internal/query.UpdateScan` | `simpledb/query/UpdateScan.java` | A scan with record mutation and RID positioning |

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
one field lookup, unchanged lookup-error propagation, and the contextual error returned for a field expression with a
nil scan. The nil-scan guard already existed; a full-module coverage audit found that this error branch had never been
executed, so the audit adds a focused regression test without changing production behavior. A second focused test now
verifies the existing contextual error for an unset expression and confirms it does not consult the scan; this closes
the other uncovered `Evaluate` return path found by that audit.

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

## Feature 13: Match a term field to another field

`Term.EquatesWithField` maps to Java `Term.equatesWithField`. It returns the opposite field when the requested field
is one side of a field-to-field equality, without depending on operand order. The Go presence result distinguishes an
explicit empty field name from an absent expression. Literal comparisons, field mismatches, and unset expressions
return `("", false)`.

`TestTermEquatesWithField` first failed because the method was undefined. It covers both operand orders, an empty
field name, and non-matching terms.

## Feature 14: Check whether a query term applies to a schema

`Term.AppliesTo` maps to Java `Term.appliesTo(Schema)`. A term applies only when both expressions apply, reusing
`Expression.AppliesTo` for literal and field-reference behavior. The Go method accepts `*record.Schema`; nil schemas
and unset expressions safely produce false for field-bearing or unset terms, while a literal-to-literal term applies.

`TestTermAppliesToSchema` first failed because `AppliesTo` was undefined. It covers two present fields, a missing field
on either side, field/literal and literal/literal terms, nil schemas, empty field names, and an unset expression.

## Feature 15: Format query expressions as text

`Expression.String` maps to Java `Expression.toString`. It delegates literal rendering to `Constant.String` and returns
field names unchanged. The Go zero value renders as `<unset-expression>` as a diagnostic for an expression form that
Java's constructors do not create. Rendering does not perform scan or schema access and does not quote SQL text.

`TestExpressionString` first failed because `Expression.String` was undefined. It covers integer and string literals,
empty text, field names, and the unset placeholder.

## Feature 16: Format query terms as text

`Term.String` maps to Java `Term.toString`. It concatenates the left expression, `=`, and the right expression by
using `Expression.String`. This preserves operand order and keeps literal rendering shared with the expression
behavior; it does not add SQL quoting. A zero-value side uses the stable expression placeholder.

`TestTermString` first failed because `Term.String` was undefined. It covers field/literal order, unquoted strings,
field/field terms, empty text, and a term with an unset expression.

## Feature 17: Treat an empty predicate as true

`Predicate.IsSatisfied` implements the vacuous-conjunction behavior of Java's empty `Predicate()` and
`isSatisfied(Scan)`. Go's zero-value `Predicate` represents the empty conjunction, so no constructor or term storage is
needed for this case. Evaluation returns true without consulting the scan and therefore also works with a nil scan.

`TestEmptyPredicateIsTrueWithoutScanAccess` first failed because `Predicate` did not exist. It verifies true for both
nil and instrumented scans and confirms that no value lookup occurs.

## Feature 18: Evaluate a single-term predicate

`NewPredicateWithTerm` maps to Java `Predicate(Term)`. `Predicate.IsSatisfied` delegates to the sole contained term and
returns its boolean result or scan error unchanged. The predicate retains slice-backed storage for the later
conjunction behavior; its zero value remains the empty true predicate from the prior feature.

The tests first failed because the constructor did not exist. They cover a satisfied field comparison, an unsatisfied
literal comparison, scan lookup order, and unchanged term error propagation.

## Feature 19: Conjoin query predicates

`Predicate.ConjoinWith` maps to Java `Predicate.conjoinWith(Predicate)`. It returns a value whose terms are the
receiver's terms followed by the other predicate's terms, preserving order and duplicates. Unlike Java's mutating
method, the Go method leaves both inputs unchanged and allocates independent term storage.

`TestPredicateConjoinWithConcatenatesTermsAndPreservesInputs` first failed because `ConjoinWith` was undefined. It
checks ordered duplicate preservation, evaluation of all included terms, empty operands, and that changing the result
does not change either input.

## Feature 20: Select predicate terms for a schema

`Predicate.SelectSubPredicate` maps to Java `Predicate.selectSubPred(Schema)`. It retains applicable terms in their
original order, including duplicates, by reusing `Term.AppliesTo`. Because Java returns `null` when no term applies,
Go returns a zero-value `Predicate` with `false`; a matching result carries `true`. Literal-only terms still apply to
a nil schema according to the expression behavior.

`TestPredicateSelectSubPredicate` first failed because `SelectSubPredicate` was undefined. It checks partial filtering,
ordering, duplicates, source immutability, no-match and empty predicates, and literal matching with a nil schema.

## Feature 21: Select join predicate terms across schemas

`Predicate.JoinSubPredicate` maps to Java `Predicate.joinSubPred(Schema, Schema)`. It selects terms that apply to the
union of both schemas while applying to neither input alone. The Go method preserves order and duplicates and returns
an explicit boolean instead of Java's `null` for an empty selection. Nil schema pointers are treated as empty schemas;
literal terms are excluded because they already apply to each input independently.

`TestPredicateJoinSubPredicate` first failed because `JoinSubPredicate` was undefined. It covers cross-schema terms,
terms applying to only one input or neither, duplicate ordering, source/schema immutability, empty results, and nil
schemas.

## Feature 22: Find constants implied by a predicate

`Predicate.EquatesWithConstant` maps to Java `Predicate.equatesWithConstant(String)`. It searches terms in order and
returns the first matching constant, delegating term shape and operand order to `Term.EquatesWithConstant`. The Go
boolean result distinguishes an absent match from a present zero-value `Constant`.

`TestPredicateEquatesWithConstant` first failed because the predicate method was undefined. It checks first-match
ordering, a present zero-value constant, a missing field, an empty predicate, and that lookup leaves terms unchanged.

## Feature 23: Find field equalities implied by a predicate

`Predicate.EquatesWithField` maps to Java `Predicate.equatesWithField(String)`. It scans terms in order and returns
the opposite field from the first match, delegating operand order to `Term.EquatesWithField`. Go's boolean result
distinguishes a present empty field name from no match.

`TestPredicateEquatesWithField` first failed because the predicate method was undefined. It covers first-match order,
an explicitly empty opposite name, absent and empty-predicate results, and source immutability.

## Feature 24: Format predicates as text

`Predicate.String` maps to Java `Predicate.toString`. It renders each term with `Term.String`, preserving term order,
and joins them with ` and `. An empty predicate, including the zero value, renders as an empty string.

`TestPredicateString` first failed because `Predicate.String` was undefined. It covers empty, single-term, and
multi-term output and verifies that formatting leaves the predicate unchanged.

## Feature 25: Define the query scan contract

`Scan` maps to Java's `simpledb.query.Scan` interface and combines before-first positioning, forward iteration,
typed integer/string reads, `ValueScan.GetVal`, field membership, and close. File-backed Go operations return errors
so query operators can propagate I/O and transaction failures without relying on Java runtime exceptions. The
interface is declared in the consumer package; record scans can satisfy it without importing the query package.

`TestScanContractProvidesTraversalAndValueOperations` first failed to compile because `query.Scan` did not exist.
The test uses a representative implementation to exercise every method through the interface. Concrete table-scan
adaptation and relational operators remain separate behaviors.

## Feature 26: Define the update scan contract

`UpdateScan` extends `Scan` with typed integer, string, and `Constant` writes; record insertion and deletion; and RID
retrieval and positioning. These operations map to Java's `UpdateScan` contract. Go methods that can touch transaction
or storage state return errors, while `record.RID` supplies the record identity type already used by the record layer.
This PR defines only the query-side contract; adding a concrete scan implementation remains a separate behavior.

`TestUpdateScanContractProvidesMutationAndRIDOperations` first failed to compile because `UpdateScan` was undefined.
It assigns an instrumented implementation to the interface and exercises every added operation through that boundary.
