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

## Intentional differences from Java

- The Go API accepts SQL text as a string rather than a Java `Reader`.
- Integer values are checked against the signed 32-bit range instead of silently narrowing a tokenizer number.
- Errors include the token's byte offset and do not advance the lexer, making callers able to report a stable location.
- SQL string literals support doubled single quotes; backslash escapes follow the simpler Go SQL lexer policy and are not decoded as Java `StreamTokenizer` escapes.
- SQL identifiers use an explicit ASCII grammar instead of `StreamTokenizer`'s additional non-ASCII word-byte behavior.
- Delimiters are visible ASCII punctuation bytes; multi-character operators are matched one byte at a time, and quoted values are not delimiters.
- Keyword matching compares the normalized input word to the expected argument exactly, as Java does; callers provide the canonical lower-case spelling.
