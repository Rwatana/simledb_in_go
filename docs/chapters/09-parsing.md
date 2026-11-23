# Chapter 9: Parsing

This chapter turns SQL text into tokens and then into the database's query and update data structures. The first
behavior reads an integer token; strings, identifiers, delimiters, keywords, and statement parsing remain separate
steps in the dependency order. Each observable behavior remains a separate Issue, branch, commit, and stacked Pull
Request.

## Java source map

| Go area | Java reference | Responsibility |
| --- | --- | --- |
| `internal/parse.Lexer` | `simpledb/parse/Lexer.java` | Read tokens while retaining source position for lookahead |

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

## Intentional differences from Java

- The Go API accepts SQL text as a string rather than a Java `Reader`.
- Integer values are checked against the signed 32-bit range instead of silently narrowing a tokenizer number.
- Errors include the token's byte offset and do not advance the lexer, making callers able to report a stable location.
- SQL string literals support doubled single quotes; backslash escapes follow the simpler Go SQL lexer policy and are not decoded as Java `StreamTokenizer` escapes.
