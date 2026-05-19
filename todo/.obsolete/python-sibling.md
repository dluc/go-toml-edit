# Python sibling library

## Context

go-toml-edit is a comment-preserving TOML editing library for Go. There is no equivalent Python library in the ecosystem. Python's stdlib has `tomllib` (read-only since 3.11) but no comment-preserving writer.

## Proposal

Build a mirroring Python library with the same capabilities: lossless round-trip TOML editing that preserves comments, formatting, and whitespace. Maintain both in an rlsbl monorepo with shared conformance tests, following the strictcli pattern (python/, go/, conformance/ sub-projects).

## Effort

Medium-large. The Go library is the reference implementation. The Python port needs to handle the same edge cases (inline tables, multiline strings, dotted keys, etc.).
