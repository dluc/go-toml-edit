// Package tomledit provides a comment-preserving TOML parser and editor.
//
// It parses TOML documents into a lossless AST that preserves comments,
// whitespace, and formatting. Values can be read, set, deleted, and renamed
// without disturbing unrelated parts of the file. The AST can be serialized
// back to bytes with Bytes (round-trip fidelity) or reformatted with Format.
//
// Key features:
//   - Lossless round-trip: parse and re-serialize without losing comments or formatting
//   - Path-based access: read and write values using dot-separated paths (e.g. "server.host")
//   - Structural editing: create tables, array-of-tables, rename and delete keys
//   - Diff and merge: compare two documents or merge defaults into an existing document
//   - Walk: traverse all key-value pairs in document order
//   - Unmarshal: decode TOML into Go structs and maps
package tomledit
