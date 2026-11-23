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

## Intentional differences from Java

- The Go API accepts SQL text as a string rather than a Java `Reader`.
- Integer values are checked against the signed 32-bit range instead of silently narrowing a tokenizer number.
- Errors include the token's byte offset and do not advance the lexer, making callers able to report a stable location.
