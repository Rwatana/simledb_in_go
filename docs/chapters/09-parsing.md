# Chapter 9: Parsing

This chapter turns SQL text into tokens and then into the database's query and update data structures. The first
behavior reads an integer token; strings, identifiers, delimiters, keywords, and statement parsing remain separate
steps in the dependency order. Each observable behavior remains a separate Issue, branch, commit, and stacked Pull
Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/parse.Lexer` | `simpledb/parse/Lexer.java` | Read tokens while retaining source position for lookahead |
| `internal/parse.Parser` | `simpledb/parse/Parser.java` | Turn token sequences into query values |
| `internal/parse.DeleteData` | `simpledb/parse/DeleteData.java` | Hold the target table and predicate of a DELETE statement |
| `internal/parse.InsertData` | `simpledb/parse/InsertData.java` | Hold the target table, ordered fields, and corresponding values of an INSERT statement |
| `Parser.ParseInsert` | `simpledb/parse/Parser.java` (`insert`) | Parse the INSERT target, field list, and matching constant list |
| `internal/parse.ModifyData` | `simpledb/parse/ModifyData.java` | Hold the target table and field, new expression, and predicate of an UPDATE statement |
| `Parser.ParseUpdate` | `simpledb/parse/Parser.java` (`modify`) | Parse the UPDATE target, assignment, and optional predicate |
| `internal/parse.CreateTableData` | `simpledb/parse/CreateTableData.java` | Hold the table name and schema of a CREATE TABLE statement |
| `internal/parse.CreateViewData` | `simpledb/parse/CreateViewData.java` | Hold the view name and rendered query definition of a CREATE VIEW statement |
| `internal/parse.CreateIndexData` | `simpledb/parse/CreateIndexData.java` | Hold the index, table, and field names of a CREATE INDEX statement |
| `Parser.ParseCreateView` | `simpledb/parse/Parser.java` (`createView`) | Parse the view name and SELECT definition after CREATE VIEW |
| `Parser.ParseCreateIndex` | `simpledb/parse/Parser.java` (`createIndex`) | Parse the index name, table, and indexed field after CREATE INDEX |
| `Parser.ParseCreateTable` | `simpledb/parse/Parser.java` (`createTable`, `fieldDefs`, `fieldDef`, `fieldType`) | Parse a table name and ordered integer/varchar field definitions |
| `Parser.ParseUpdateCommand` | `simpledb/parse/Parser.java` (`updateCmd`, `create`) | Dispatch INSERT, DELETE, UPDATE, and CREATE TABLE/VIEW/INDEX into concrete command data |

## Feature 1: Read integer literals from SQL text

`NewLexer`, `MatchIntConstant`, and `EatIntConstant` correspond to Java `Lexer(Reader)`, `matchIntConstant`, and
`eatIntConstant`. The match operation skips leading ASCII SQL whitespace and recognizes decimal digits without
consuming the token. Eating parses the token as a non-negative `int32`, advances the cursor only after successful
conversion, and preserves following input for the next lexer behavior. An unexpected token or an out-of-range value
returns a byte-positioned error without consuming input.

Go keeps the original input in an immutable string and records byte offsets instead of exposing Java's mutable
`StreamTokenizer`. This behavior intentionally handles only integer tokens; signs, string constants, identifiers,
delimiters, and keywords are separate follow-up behaviors. The [integer-token diagram](../diagrams/ch09-integer-token.svg)
shows that matching is a lookahead and consuming advances only after a valid conversion; its editable source is
[`ch09-integer-token.drawio`](../diagrams/ch09-integer-token.drawio).

### Test-first record

The tests first failed to compile because `NewLexer` and `Lexer` were undefined. They cover non-consuming lookahead,
multiple integer tokens separated by SQL whitespace, invalid tokens, int32 overflow, empty input, and a nil receiver.

## Feature 2: Read string constants from SQL text

`MatchStringConstant` and `EatStringConstant` map to Java `Lexer.matchStringConstant` and
`eatStringConstant`. Matching is non-consuming and accepts a complete single-quoted token. Eating preserves the
literal's case and UTF-8 bytes, decodes doubled single quotes (`'it''s'` becomes `it's`), and advances only after
finding a closing quote. Unterminated and non-string tokens return byte-positioned errors without moving the cursor.

Go scans the immutable source string directly and stores the decoded token in the existing lookahead cache. This
keeps integer and string consumption consistent without introducing Java's mutable `StreamTokenizer`. Backslashes
remain ordinary string bytes; doubled quotes are the SQL escaping rule implemented here. The [string-token diagram](../diagrams/ch09-string-token.svg)
shows lookahead, quote decoding, and the non-consuming error path; its editable source is
[`ch09-string-token.drawio`](../diagrams/ch09-string-token.drawio).

### Test-first record

The tests first failed to compile because `MatchStringConstant` and `EatStringConstant` were undefined. They cover
non-consuming lookahead, preserved case and UTF-8, doubled quotes, an empty value, following integer input,
unterminated and non-string errors, stable byte offsets, empty input, and nil receivers.

## Feature 3: Read identifiers from SQL text

`MatchIdentifier` and `EatIdentifier` map to Java `Lexer.matchId` and `eatId`. The lexer accepts an ASCII letter or
underscore followed by ASCII letters, digits, or underscores. It normalizes the result to lower case, matching Java's
case-insensitive keyword and identifier handling, and rejects the reserved words initialized by the SimpleDB lexer.
Lookahead does not consume the token; a rejected word or non-word token returns a byte-positioned error without
advancing the cursor.

The Go scanner retains the normalized word in its existing lookahead cache so the next lexer behavior can distinguish
keywords without rescanning the input. Go's identifier grammar is intentionally ASCII-only; Java's `StreamTokenizer`
also treats some non-ASCII bytes as word characters. The [identifier-token diagram](../diagrams/ch09-identifier-token.svg)
shows normalization and reserved-word rejection; its editable source is
[`ch09-identifier-token.drawio`](../diagrams/ch09-identifier-token.drawio).

### Test-first record

Tests first failed to compile because `MatchIdentifier` and `EatIdentifier` were undefined. They cover case
normalization, underscores, digits after the first character, every reserved word in the Java keyword list, byte offsets,
non-consuming mismatch, empty input, whitespace, nil receivers, and the following integer token.

## Feature 4: Read SQL delimiters

`MatchDelimiter` and `EatDelimiter` map to Java `Lexer.matchDelim` and `eatDelim`. They compare a requested ASCII
punctuation byte with the next token. Lookahead does not consume input, and eating advances exactly one punctuation
byte only when it matches. Mismatches preserve the current token and report its byte offset.

The Go lexer classifies visible ASCII punctuation as delimiters except the single and double quote bytes. Operators
containing multiple punctuation bytes remain separate tokens, so `<` followed by `=` is two delimiter operations. The
[delimiter-token diagram](../diagrams/ch09-delimiter-token.svg) shows matching, single-byte consumption,
and the unchanged cursor on failure; the editable source is
[`ch09-delimiter-token.drawio`](../diagrams/ch09-delimiter-token.drawio).

### Test-first record

The tests first failed to compile because `MatchDelimiter` and `EatDelimiter` were undefined. They cover visible
punctuation, repeated lookahead, adjacent identifiers, mismatched expected delimiters, wrong token kinds, byte offsets,
empty input, whitespace, and nil receivers.

## Feature 5: Match expected SQL keywords

`MatchKeyword` and `EatKeyword` map to Java `Lexer.matchKeyword` and `eatKeyword`. The lexer compares the next complete
word with the expected spelling. Input words are normalized to lower case by the existing word scanner, while the
expected argument is compared as supplied, matching Java's API contract. Lookahead does not consume; a successful eat
advances past only that word. An unexpected token or spelling returns a byte-positioned error without advancing.

This operation intentionally checks the requested word, rather than requiring it to be a member of the reserved-word
list. That preserves Java's behavior when callers use it for a known grammar word while `MatchIdentifier` remains the
separate reserved-word check. The [keyword-token diagram](../diagrams/ch09-keyword-token.svg) shows exact word matching,
consumption, and the unchanged cursor on failure; its editable source is
[`ch09-keyword-token.drawio`](../diagrams/ch09-keyword-token.drawio).

### Test-first record

Tests first failed to compile because `MatchKeyword` and `EatKeyword` were undefined. They cover a reserved SQL word,
an ordinary expected word, uppercase input normalization, the Java expected-argument comparison rule, word boundaries,
repeated lookahead, following identifiers and keywords, mismatches, wrong token kinds, stable byte offsets, empty input,
whitespace, and nil receivers.

## Feature 6: Parse SQL constant literals

`Parser.ParseConstant` maps to Java `Parser.constant`. It reads either a single-quoted string or a non-negative integer
from the parser's lexer and returns the existing `query.Constant` value. String parsing keeps the lexer's decoded
contents; integer parsing keeps the checked `int32` range. The parser advances only after the selected lexer operation
succeeds, and lexer errors retain their byte offset and cause through a parser-specific context.

Go keeps the parser as a small adapter over `Lexer` and `query.Constant` instead of introducing a second constant type.
The [constant-parsing diagram](../diagrams/ch09-parse-constant.svg) shows the string and integer paths converging on the
query constant value while invalid input preserves the token; its editable source is
[`ch09-parse-constant.drawio`](../diagrams/ch09-parse-constant.drawio).

### Test-first record

The tests first failed to compile because `NewParser` and `Parser` were undefined. They cover integer and decoded string
values, preserving following input, wrong token kinds, unterminated strings, integer overflow, stable byte offsets,
repeated failures without consumption, empty input, whitespace, and a nil parser receiver.

## Feature 7: Parse SQL field names

`Parser.ParseField` maps to Java `Parser.field`. It consumes one legal identifier using the lexer's identifier rules
and returns the lower-case name. Reserved words and other token kinds are rejected with the lexer byte offset and
remain available after failure. The parser wraps the lexer error with field context and does not introduce a second
identifier type. The [field-parsing diagram](../diagrams/ch09-parse-field.svg) shows normalization, preserved following
input, and the non-consuming error path; its editable source is
[`ch09-parse-field.drawio`](../diagrams/ch09-parse-field.drawio).

### Test-first record

Tests first failed because `Parser.ParseField` was undefined. They cover mixed-case normalization, following delimiters
and strings, reserved-word and non-identifier rejection, stable byte offsets, repeated failures without consumption,
empty input, whitespace, and a nil parser receiver.

## Feature 8: Parse SQL expressions

`Parser.ParseExpression` maps to Java `Parser.expression`. It treats a valid identifier as a field reference and
otherwise parses an integer or string literal as a constant expression. Field names use the lexer's lower-case
normalization. Each successful path consumes one token and preserves following input; reserved words, delimiters,
empty input, and other invalid tokens return a byte-positioned error without consuming. Go reuses the existing
`query.Expression` constructors rather than introducing another expression representation. The
[expression-parsing diagram](../diagrams/ch09-parse-expression.svg) shows the identifier branch and literal branch
converging on the query expression value; its editable source is
[`ch09-parse-expression.drawio`](../diagrams/ch09-parse-expression.drawio).

### Test-first record

Tests first failed because `Parser.ParseExpression` was undefined. They cover normalized field references, integer and
decoded string constants, following tokens, reserved-word and delimiter failures, stable byte offsets, repeated
non-consuming errors, empty input, whitespace, and a nil parser receiver.

## Feature 9: Parse SQL equality terms

`Parser.ParseTerm` maps to Java `Parser.term`. It parses the left expression, consumes one `=` delimiter, then parses
the right expression and constructs the existing `query.Term`. Either side may be a field reference, integer, or
string constant. Errors identify the failing side or missing delimiter and preserve the lexer byte offset; parsing a
term does not roll back tokens consumed before the failure, matching the Java parser's sequential behavior. The
[term-parsing diagram](../diagrams/ch09-parse-term.svg) shows the left-expression, equality-delimiter, and right-expression
sequence; its editable source is [`ch09-parse-term.drawio`](../diagrams/ch09-parse-term.drawio).

### Test-first record

Tests first failed because `Parser.ParseTerm` was undefined. They cover field-to-literal and literal-to-field terms,
field-to-field terms, preserved trailing input, missing delimiters, invalid left and right operands, stable byte
offsets, whitespace, and a nil parser receiver.

## Feature 10: Parse SQL predicates

`Parser.ParsePredicate` maps to Java `Parser.predicate`. It begins with one equality term and recursively parses each
following term after an `and` keyword, joining them in source order with `query.Predicate.ConjoinWith`. The first
non-`and` token remains available to the enclosing query grammar. A malformed first term or an incomplete conjunction
returns contextual byte-positioned errors; tokens consumed before the failure remain consumed, matching the Java
parser's sequential behavior. The [predicate-parsing diagram](../diagrams/ch09-parse-predicate.svg) shows recursive term
composition and termination at the next grammar token; its editable source is
[`ch09-parse-predicate.drawio`](../diagrams/ch09-parse-predicate.drawio).

### Test-first record

Tests first failed because `Parser.ParsePredicate` was undefined. They cover a single term, ordered multi-term
conjunction, a following delimiter or keyword, invalid first and later terms, missing terms after `and`, stable byte
offsets, and a nil parser receiver.

## Feature 11: Store parsed SELECT query data

`parse.QueryData` maps to Java `parse.QueryData` and carries the selected fields, source tables, and query predicate.
Go stores the lists in their supplied order but copies them on construction and when returned by accessors, so later
caller mutation cannot change a parsed query. The predicate is returned as a value; its query API exposes no mutable
term storage. The zero value and incomplete field/table lists render as `<unset-query>` rather than panicking. The
[query-data diagram](../diagrams/ch09-query-data.svg) shows owned lists, the predicate value, list accessors, and
rendering; its editable source is [`ch09-query-data.drawio`](../diagrams/ch09-query-data.drawio).

### Test-first record

Tests first failed because `NewQueryData` and `QueryData` were undefined. They cover ordered fields and tables, stored
list isolation, independent accessor slices, predicate retention, optional `where` rendering, and safe zero or
incomplete values.

## Feature 12: Parse SQL SELECT statements

`Parser.ParseQuery` maps to Java `Parser.query`. It consumes `select`, a comma-separated field list, `from`, and a
comma-separated table list, then parses an optional `where` predicate with the existing parser. The result is the
existing `QueryData` value. Field and table order are preserved, and the first token after the query remains available
to an enclosing grammar instead of requiring end-of-input.

Go parses the comma-separated lists iteratively rather than using Java's recursive list methods, avoiding recursion
for long projections or table lists. Missing keywords, names, and malformed predicates return contextual errors with
the lexer's byte offset; earlier tokens remain consumed, matching the Java parser's sequential behavior. The
[SELECT-query diagram](../diagrams/ch09-parse-query.svg) shows the clause order, optional predicate, preserved trailing
token, and error exits; its editable source is [`ch09-parse-query.drawio`](../diagrams/ch09-parse-query.drawio).

### Test-first record

Tests first failed to compile because `Parser.ParseQuery` was undefined. They cover ordered multi-field and multi-table
lists, a multi-term `where` clause, queries without `where`, preservation of a following keyword, missing `select` or
`from`, missing names at either list position, incomplete predicates, byte offsets, and a nil parser receiver.

## Feature 13: Store parsed DELETE command data

`parse.DeleteData` maps to Java `parse.DeleteData`. It keeps the affected table name and the query predicate that selects
records to delete. Go uses unexported fields, a value constructor, and read-only accessors so command data cannot be
changed through field assignment. The zero value returns an empty table name and empty predicate. The
[DELETE-data diagram](../diagrams/ch09-delete-data.svg) shows the table and predicate stored together; its editable
source is [`ch09-delete-data.drawio`](../diagrams/ch09-delete-data.drawio).

### Test-first record

Tests first failed to compile because `DeleteData` and `NewDeleteData` were undefined. They cover storing and reading
the target table and predicate, plus safe accessors on the zero value.

## Feature 14: Parse SQL DELETE statements

`Parser.ParseDelete` maps to Java `Parser.delete`. It consumes `delete from`, reads the normalized table identifier,
and optionally parses a `where` predicate through the existing predicate parser. Without `where`, the returned
`DeleteData` contains an empty predicate. Any token after the command remains available to the caller.

Go keeps DELETE parsing independent from `Parser.ParseUpdateCommand`: this method parses one concrete command and
returns its existing data value; top-level dispatch is a separate behavior. Errors retain lexer byte offsets and add
clause context. The [DELETE-parser diagram](../diagrams/ch09-parse-delete.svg) shows the required clauses, optional
predicate, preserved next token, and error paths; the editable source is
[`ch09-parse-delete.drawio`](../diagrams/ch09-parse-delete.drawio).

### Test-first record

Tests first failed to compile because `Parser.ParseDelete` was undefined. They cover a mixed-case command with an
ordered conjunction, an omitted predicate, preservation of a following keyword, missing command clauses and table
names, incomplete predicates, byte offsets, and a nil receiver.

## Feature 15: Store parsed INSERT command data

`parse.InsertData` maps to Java `parse.InsertData`. It stores the target table, the ordered field names, and the ordered
`query.Constant` values corresponding to those fields. Go copies both slices when constructing the value and again
when returning them from accessors, so mutation through caller-owned slices cannot alter the stored command. The
zero value safely returns an empty table name and nil lists. The [INSERT-data diagram](../diagrams/ch09-insert-data.svg)
shows the aligned field/value lists and defensive-copy boundary; its editable source is
[`ch09-insert-data.drawio`](../diagrams/ch09-insert-data.drawio).

### Test-first record

Tests first failed to compile because `InsertData` and `NewInsertData` were undefined. They cover the table name,
field/value order, constructor-copy isolation, accessor-copy isolation, and the zero value.

## Feature 16: Parse SQL INSERT statements

`Parser.ParseInsert` maps to Java `Parser.insert`. It consumes `insert into`, reads a normalized table name, parses a
parenthesized ordered field list, then parses the matching parenthesized list of integer or string constants after
`values`. It returns `InsertData`, which preserves the source order and owns copies of both lists. Any token following
the closing parenthesis remains available to the caller.

Go parses each comma-separated list iteratively and does not add a field/value count check that the Java parser does
not perform. Contextual errors wrap the existing lexer failures so their byte offsets remain available. The
[INSERT-parser diagram](../diagrams/ch09-parse-insert.svg) shows the required sequence, ordered lists, preserved
following token, and error paths; its editable source is
[`ch09-parse-insert.drawio`](../diagrams/ch09-parse-insert.drawio).

### Test-first record

Tests first failed because `Parser.ParseInsert` was undefined. They cover mixed-case keywords and fields, ordered
string/integer values, a preserved following keyword, each required keyword and delimiter, missing list elements,
malformed constants, byte offsets, and a nil receiver.

## Feature 17: Store parsed UPDATE command data

`parse.ModifyData` maps to Java `parse.ModifyData`. It stores the target table and field, the expression that produces
the new field value, and the predicate selecting records to change. Go keeps these fields private and exposes them
through value-returning accessors. Its zero value safely exposes empty names, an unset expression, and an empty
predicate. The [UPDATE-data diagram](../diagrams/ch09-modify-data.svg) shows the target, expression, predicate, and
read-only accessors; its editable source is
[`ch09-modify-data.drawio`](../diagrams/ch09-modify-data.drawio).

### Test-first record

Tests first failed to compile because `ModifyData` and `NewModifyData` were undefined. They cover storing and reading
each command value and the safe zero-value accessors.

## Feature 18: Parse SQL UPDATE statements

`Parser.ParseUpdate` maps to Java `Parser.modify`. It consumes `update`, reads the normalized table and target field,
parses the assignment expression after `set` and `=`, then optionally parses a `where` predicate. The resulting
`ModifyData` keeps the table, target field, expression, and predicate together; when `where` is omitted, the predicate
is empty. The first token after the command remains available to the caller.

Go wraps each lexer or parser error with its grammar position while preserving the original byte offset. The
[UPDATE-parser diagram](../diagrams/ch09-parse-update.svg) shows the assignment path, optional predicate, returned data,
and error branches; its editable source is
[`ch09-parse-update.drawio`](../diagrams/ch09-parse-update.drawio).

### Test-first record

Tests first failed because `Parser.ParseUpdate` was undefined. They cover a mixed-case assignment with an ordered
predicate, a constant assignment without `where`, preservation of a following keyword, missing grammar elements,
malformed predicates, byte offsets, and a nil receiver.

## Feature 19: Store parsed CREATE TABLE command data

`parse.CreateTableData` maps to Java `parse.CreateTableData`. It stores the new table name and schema. Go copies the
schema at construction and when returned by its accessor, preventing mutations to the original or a returned schema
from changing the stored command. The zero value safely returns an empty name and empty schema. The
[CREATE TABLE-data diagram](../diagrams/ch09-create-table-data.svg) shows the schema-copy boundary and accessors; its
editable source is [`ch09-create-table-data.drawio`](../diagrams/ch09-create-table-data.drawio).

### Test-first record

Tests first failed to compile because `CreateTableData` and `NewCreateTableData` were undefined. They cover schema
field order and definitions, constructor-copy isolation, accessor-copy isolation, and the zero value.

## Feature 20: Store parsed CREATE VIEW command data

`parse.CreateViewData` maps to Java `parse.CreateViewData`. It stores the view name and a `QueryData` definition, and
`ViewDefinition` renders the query through its stable `String` representation. Go exposes read-only accessors and keeps
the unset zero value safe: its name is empty and its definition is `<unset-query>`. Parsing the CREATE VIEW syntax is a
separate behavior. The [CREATE VIEW-data diagram](../diagrams/ch09-create-view-data.svg) shows the stored name, query,
and returned definition; its editable source is [`ch09-create-view-data.drawio`](../diagrams/ch09-create-view-data.drawio).

### Test-first record

Tests first failed to compile because `CreateViewData` and `NewCreateViewData` were undefined. They cover the stored
view name, rendering a query definition with its predicate, and safe zero-value accessors.

## Feature 21: Store parsed CREATE INDEX command data

`parse.CreateIndexData` maps to Java `parse.CreateIndexData`. It stores the index, table, and indexed-field names and
returns them through read-only accessors. The zero value safely returns empty names. CREATE INDEX grammar parsing is a
separate behavior. The [CREATE INDEX-data diagram](../diagrams/ch09-create-index-data.svg) shows the three constructor
values and their accessors; its editable source is [`ch09-create-index-data.drawio`](../diagrams/ch09-create-index-data.drawio).

### Test-first record

Tests first failed to compile because `CreateIndexData` and `NewCreateIndexData` were undefined. They cover each stored
name and the safe zero-value accessors.

## Feature 22: Parse SQL CREATE VIEW statements

`Parser.ParseCreateView` maps to Java `Parser.createView`. It consumes `create view`, reads the normalized view name,
consumes `as`, and delegates the definition to `ParseQuery`. It returns the first token after that query untouched and
wraps grammar errors with context while preserving the byte offset. CREATE INDEX parsing and update dispatch remain
separate behaviors. The [CREATE VIEW-parser diagram](../diagrams/ch09-parse-create-view.svg) shows the required keyword
sequence, query delegation, result, and error paths; its editable source is
[`ch09-parse-create-view.drawio`](../diagrams/ch09-parse-create-view.drawio).

### Test-first record

Tests first failed because `Parser.ParseCreateView` was undefined. They cover mixed-case syntax and query predicates,
preservation of a following token, missing keywords/name/query pieces, byte offsets, and a nil receiver.

## Feature 23: Parse SQL CREATE INDEX statements

`Parser.ParseCreateIndex` maps to Java `Parser.createIndex`. It consumes `create index`, reads the normalized index and
table names around `on`, then parses one field name inside parentheses and returns `CreateIndexData`. It leaves the next
token untouched and wraps each grammar error without losing its byte offset. The [CREATE INDEX-parser diagram](../diagrams/ch09-parse-create-index.svg)
shows the command sequence, result, and delimiter/error paths; its editable source is
[`ch09-parse-create-index.drawio`](../diagrams/ch09-parse-create-index.drawio).

### Test-first record

Tests first failed because `Parser.ParseCreateIndex` was undefined. They cover mixed-case names, following-token
preservation, every required keyword/name/delimiter, byte offsets, and a nil receiver.

## Feature 24: Parse SQL CREATE TABLE statements

`Parser.ParseCreateTable` maps to Java `Parser.createTable` and its field-definition helpers. It reads `create table`,
the normalized table name, and one or more comma-separated field definitions. Integer fields use `int`; string fields
use `varchar(<non-negative length>)`. It returns the copied schema in declaration order, leaves trailing input for the
caller, and wraps grammar failures without losing byte offsets. The [CREATE TABLE-parser diagram](../diagrams/ch09-parse-create-table.svg)
shows the field-type choice, list loop, result, and error paths; its editable source is
[`ch09-parse-create-table.drawio`](../diagrams/ch09-parse-create-table.drawio).

### Test-first record

Tests first failed because `Parser.ParseCreateTable` was undefined. They cover integer and varchar field metadata,
ordered lists, trailing-token preservation, every grammar and type failure, byte offsets, and a nil receiver.

## Feature 25: Dispatch SQL update commands

`Parser.ParseUpdateCommand` maps to Java `Parser.updateCmd` and `Parser.create`. It selects the concrete INSERT,
DELETE, UPDATE, CREATE TABLE, CREATE VIEW, or CREATE INDEX parser from the leading keywords and returns the existing
command-data value as `any`. A caller can use a type switch without duplicating the Java command hierarchy. The
dispatcher consumes only its command and leaves the next token to the enclosing parser. Errors include the selected
command context and retain the lexer's byte offset. The [update-command dispatch diagram](../diagrams/ch09-update-command-dispatch.svg)
shows the top-level choice, nested CREATE choice, concrete results, preserved token, and error paths; its editable
source is [`ch09-update-command-dispatch.drawio`](../diagrams/ch09-update-command-dispatch.drawio).

### Test-first record

Tests first failed because `Parser.ParseUpdateCommand` was undefined. They cover all six concrete results,
following-token preservation, malformed commands for every route, an unsupported CREATE subtype, an unrelated query
statement, byte-offset retention, and a nil receiver.

## Intentional differences from Java

- The Go API accepts SQL text as a string rather than a Java `Reader`.
- Integer values are checked against the signed 32-bit range instead of silently narrowing a tokenizer number.
- Errors include the token's byte offset and do not advance the lexer, making callers able to report a stable location.
- SQL string literals support doubled single quotes; backslash escapes follow the simpler Go SQL lexer policy and are not decoded as Java `StreamTokenizer` escapes.
- SQL identifiers use an explicit ASCII grammar instead of `StreamTokenizer`'s additional non-ASCII word-byte behavior.
- Delimiters are visible ASCII punctuation bytes; multi-character operators are matched one byte at a time, and quoted values are not delimiters.
- Keyword matching compares the normalized input word to the expected argument exactly, as Java does; callers provide the canonical lower-case spelling.
- SELECT field and table lists are parsed iteratively rather than recursively, and query parsing leaves any trailing token for its caller.
- `DeleteData` keeps its fields private and exposes a safe zero value; Java exposes its command data only through accessor methods but does not define Go's zero-value behavior.
- `ParseDelete` is a concrete-command parser separate from Go's future update-command dispatcher; it leaves trailing tokens to its caller.
- `InsertData` copies its field and value slices for clear ownership; Java exposes its internal mutable lists directly.
- `ParseInsert` uses iterative list parsing and leaves the next token to its caller; Java recurses through each list.
- `ModifyData` returns the query predicate by value and retains an unset-expression zero value; Java provides accessors but no Go-style zero-value contract.
- `ParseUpdate` is a concrete command parser separate from top-level dispatch; it leaves trailing tokens to its caller.
- `CreateTableData` copies schema values to avoid shared internal slices and maps; Java stores and returns the same mutable schema reference.
- `CreateViewData` exposes the query definition as rendered text rather than returning `QueryData`; the stored query value remains read-only through its public API.
- `CreateIndexData` keeps the Java command's three string values private and exposes Go-style accessors; its zero value returns empty strings.
- `ParseCreateView` uses the Go `ParseQuery` method and preserves trailing input for its caller, matching Java's query delegation without consuming an extra token.
- `ParseCreateIndex` returns contextual errors and preserves the following token, while Java mutates a parser's shared stream and throws on grammar failure.
- `ParseCreateTable` parses field definitions iteratively and rejects unknown types with a contextual error; Java recurses through definitions and treats any non-`int` type as `varchar`.
- `ParseUpdateCommand` returns one of six existing command-data types through `any` instead of constructing Java's command subclasses; parsing remains separate from command execution.
