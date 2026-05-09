# Gaps

Everything needed before go-toml-edit is ready for public release, plus gaps in the current implementation for broader use cases.

## Must-have for public launch

### 1. Unmarshal

Decode TOML into Go structs. This is the most common TOML use case and the reason most people reach for a TOML library. Without it, adoption is limited to the editing niche.

Scope:
- `Unmarshal(data []byte, v any) error` -- top-level function
- `(*Document).Decode(v any) error` -- decode from an already-parsed document
- Full struct tag support: `toml:"name"`, `toml:"-"`, `toml:",omitempty"` (omitempty reserved for Marshal but recognized and ignored)
- Type mapping: string, all int/uint sizes, float32/float64, bool, time.Time, slices, arrays, maps, nested structs
- Custom `LocalDateTime`, `LocalDate`, `LocalTime` types with proper decoding
- Embedded struct promotion (like encoding/json)
- `any`/`interface{}` decoding to `map[string]any`, `[]any`, and Go primitives
- Case-insensitive field matching when no tag is present (exact match first, then case-insensitive)

Affected files: new `unmarshal.go`, new `unmarshal_test.go`.

Estimated effort: 400-600 LOC source, 500-800 LOC tests.

### 2. Array iteration and querying

The current API has no way to iterate over array elements or filter them by content. You can access individual elements by index (`Get("items[0]")`), but common operations like "find the element where name == X" or "remove all elements matching condition Y" require the consumer to manually walk the node tree.

Needed:
- `(*Document).Each(path string, fn func(index int, node Node) bool)` -- iterate elements, return false to stop
- `(*Document).Len(path string) int` -- number of elements at path (-1 if not an array)
- Consider: `(*Document).FindIndex(path string, fn func(Node) bool) int` -- find first matching element

These are building blocks for conditional set/remove operations.

Affected files: `document.go` or new `iterate.go`, tests.

Estimated effort: 100-200 LOC source, 200-300 LOC tests.

### 3. Deep merge

Set missing keys recursively without overwriting existing values. Given a defaults map, walk it and only set keys that don't already exist in the document.

```go
func (d *Document) MergeDefaults(path string, defaults map[string]any) error
```

Behavior:
- Maps/tables merge recursively (add missing keys, never overwrite existing)
- Scalars and arrays are atomic (if a value exists, keep it; if missing, set the default)
- Creates intermediate tables as needed

Affected files: `edit.go` or new `merge.go`, tests.

Estimated effort: 100-200 LOC source, 200-300 LOC tests.

## Should-have for credibility

### 5. Comprehensive README

A published library needs a README that:
- Shows the problem (round-trip destroys comments with existing libraries)
- Shows a before/after code example
- Documents the full public API with examples
- Includes installation instructions (`go get`)
- Links to pkg.go.dev documentation
- Shows benchmark results vs pelletier/go-toml/v2

### 6. License

Currently TBD. Pick and add a LICENSE file. MIT is standard for Go libraries.

### 7. CI/CD via rlsbl scaffold

Scaffold CI workflows for the project. Since this is a Go library (no `package main`), rlsbl will skip GoReleaser and set up CI-only workflows.

### 8. godoc comments on all exported types and functions

pkg.go.dev renders godoc. Every exported symbol needs a doc comment. Audit all exported types, functions, methods, and constants.

## Nice-to-have

### 9. SetComment / comment manipulation helpers

The Node interface has `SetComment` and `SetLeadingComments`, but there's no path-based convenience for adding or modifying comments:

```go
func (d *Document) SetComment(path string, comment string) error
func (d *Document) SetLeadingComments(path string, comments []string) error
```

### 10. Walk / visitor

Generic tree traversal for consumers who need to process every node:

```go
func (d *Document) Walk(fn func(path string, node Node) error) error
```

### 11. Diff

Compare two documents and report which paths differ:

```go
func Diff(a, b *Document) []Change
```

Useful for testing and debugging.

### 12. Benchmark comparison README section

Run benchmarks against pelletier/go-toml/v2 and BurntSushi/toml. Include results in README. The editing features are the differentiator, but parse/decode performance should be competitive enough not to deter adoption.

## v2 (deferred, not blocking launch)

- Marshal: struct-to-TOML serialization
- TOML 1.1 support when spec stabilizes
- Public streaming/event parser
- Lazy parsing for large files
