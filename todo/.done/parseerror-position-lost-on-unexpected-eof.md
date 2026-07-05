# ParseError reports line 0, column 0 on unexpected EOF

## Context

A consumer uses `Unmarshal` to load user-written TOML files and surfaces
`ParseError`'s line/column in its error messages so users can find the typo.
This works for mid-file errors but breaks for end-of-file errors.

## Problem

When the parse error is an unexpected EOF — e.g. an unclosed array at the end
of the file:

```toml
exclude = ["a", "b"
```

the returned `*ParseError` carries `Line: 0, Column: 0`, rendering as
`line 0, column 0: ...`, which points nowhere (positions are otherwise
1-based). Mid-file errors report correct positions, e.g.
`line 2, column 9: expected value, got Equals`.

Likely cause: the EOF token (or the error constructed when the token stream
runs out) is built without propagating the last consumed token's position, so
the zero value of the position fields leaks into the error.

## Proposed solutions

1. Give the EOF token the position one past the last real token (final line,
   column after last content). All error paths then work unchanged.
   - Pros: single fix at tokenizer level; every EOF-adjacent error message
     improves at once.
   - Cons: none apparent.
2. Special-case error construction: when position is zero, substitute the last
   known good position and a message suffix like "(unexpected end of file)".
   - Pros: localized.
   - Cons: patches the symptom in one path; other zero-position leaks remain
     possible.

Option 1 is the correct fix. A regression test should parse an unclosed array
(and an unclosed table/string) at EOF and assert a non-zero, final-line
position — written red first per the red-green policy.

## Affected files

Tokenizer/lexer (EOF token construction), parser error paths, `errors.go`
(`ParseError`), parser tests.

## Effort estimate

Small — likely under an hour including the regression tests.
