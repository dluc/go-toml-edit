# Changelog

## 0.1.0

- Full TOML 1.0 parser with comment and whitespace preservation
- Zero-diff round-trip serialization for untouched regions
- Document editing API: Set, SetCreate, Delete, Rename, NewTable, NewArrayTable
- Typed getters: Get, GetString, GetInt, GetBool, GetFloat, GetTime
- Fluent cursor API with accumulating errors
- Explicit path resolution: Resolve(path) (Node, error)
- Path syntax: dot-separated keys, bracket indices, negative indices, quoted/escaped dots
- Unmarshal and Decode with struct tags, embedded structs, custom Unmarshaler, TextUnmarshaler
- Formatter with configurable style (indentation, line width, table spacing)
- Walk for depth-first document traversal
- Diff for comparing two documents
- MergeDefaults and Merge for deep merging
- SetComment and SetLeadingComments helpers
- Items iterator (range-over-func) and Len for arrays
- Full toml-test suite compliance (205 valid, 474 invalid)
- Fuzz testing for parser robustness

## 0.0.0

- Initial release
