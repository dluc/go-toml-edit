# go-toml-edit Design Spec (v1)

Comment-preserving TOML editing library for Go. The Go equivalent of Python's tomlkit.

## Project Identity

| Property | Value |
|----------|-------|
| Module path | `github.com/smm-h/go-toml-edit` |
| Package name | `tomledit` |
| Type | Pure Go library (no CLI, no binary) |
| TOML spec | 1.0 only |
| Go version | 1.21+ |
| License | TBD |

## v1 Scope

Ship the differentiator: editing with preservation. No Unmarshal, no Marshal.

- Parse TOML source into a Document
- Read values via dot-path or fluent cursor
- Edit values (Set, Delete, Rename)
- Serialize back to bytes with zero-diff on untouched regions
- Format (normalize) with configurable style
- Full TOML 1.0 feature coverage

Deferred to v2: Unmarshal (struct decoding), Marshal (struct encoding), TOML 1.1.

## Core Model: Tree-Walk Concatenation

Each AST node holds its own raw byte slice (including trivia). There are no global byte offsets into the original source.

1. Parse TOML source into a tree of nodes. Each node stores the raw bytes that produced it.
2. Trivia (comments, whitespace, blank lines) is attached to adjacent nodes, not discarded.
3. When a node is modified via the API, it is marked dirty. Its raw bytes are discarded.
4. On serialization, walk the tree and concatenate: clean nodes emit their raw bytes unchanged; dirty nodes are re-rendered from their semantic value + default trivia formatting.

This guarantees zero-diff round-trips for untouched regions without the complexity of maintaining and recomputing global byte offsets on structural mutations (insertions, deletions).

## Trivia Model

Every node has associated trivia:

- Leading whitespace (spaces, tabs before the node on its line)
- Leading comments (full-line `# ...` comments above the node)
- Inline comment (`# ...` after the value on the same line)
- Trailing whitespace/newlines

Trivia is stored as raw byte slices. When a dirty node is re-rendered, default trivia formatting is applied.

## Node Types

| Node type | TOML construct | Example |
|-----------|---------------|---------|
| Document | Root container | (the whole file) |
| Table | Standard table | `[server]` |
| ArrayTable | Array of tables | `[[products]]` |
| KeyValue | Key-value pair | `name = "Tom"` |
| Key | Bare or quoted key | `name`, `"name"`, `'name'` |
| String | Basic or literal string | `"hello"`, `'hello'` |
| MultilineString | Multi-line strings | `"""..."""`, `'''...'''` |
| Integer | Integer value | `42`, `0xff`, `0o77`, `0b1010` |
| Float | Float value | `3.14`, `inf`, `nan` |
| Boolean | Boolean value | `true`, `false` |
| DateTime | Offset date-time | `1979-05-27T07:32:00Z` |
| LocalDateTime | Local date-time | `1979-05-27T07:32:00` |
| LocalDate | Local date | `1979-05-27` |
| LocalTime | Local time | `07:32:00` |
| Array | Array value | `[1, 2, 3]` |
| InlineTable | Inline table | `{name = "Tom", age = 30}` |

## Path Syntax

Dot-separated keys, bracket syntax for array indices.

| Path | Meaning |
|------|---------|
| `server.host` | Key `host` in table `[server]` |
| `products[0].name` | First element of `[[products]]`, key `name` |
| `products[-1]` | Last element of `[[products]]` |
| `matrix[0][1]` | Nested array indexing |
| `server."host.name"` | Key `host.name` (literal dot) in table `[server]` |
| `server.host\.name` | Same as above (backslash escape, alternative syntax) |

Rules:
- Bare segments are key lookups: `server`, `host`
- `[N]` is an array index (zero-based): `[0]`, `[2]`
- `[-N]` is a negative index from the end: `[-1]` = last, `[-2]` = second-to-last
- Literal dots in key names can be escaped two ways: quote the segment (`"host.name"`) or backslash-escape the dot (`host\.name`)
- Both escaping forms are always accepted; they are equivalent

## Public API (v1)

### Document Operations

```go
func Parse(source []byte) (*Document, error)
func (d *Document) Bytes() []byte
```

### String-Path API: Reading

```go
func (d *Document) Get(path string) Node
func (d *Document) GetString(path string) (string, bool)
func (d *Document) GetInt(path string) (int64, bool)
func (d *Document) GetBool(path string) (bool, bool)
func (d *Document) GetFloat(path string) (float64, bool)
func (d *Document) GetTime(path string) (time.Time, bool)
```

### String-Path API: Editing

```go
// Auto-create is mandatory and explicit at every call site.
// Implementor chooses between two-method or typed-constant approach:
//
// Two-method approach:
//   func (d *Document) Set(path string, value any) error       // errors on missing intermediates
//   func (d *Document) SetCreate(path string, value any) error  // auto-creates intermediates
//
// OR typed-constant approach:
//   func (d *Document) Set(path string, value any, create CreateMode) error
//   const Create CreateMode = ...
//   const NoCreate CreateMode = ...
//
// Whichever has simpler implementation wins.

func (d *Document) Delete(path string) error
func (d *Document) Rename(path string, newKey string) error
```

Set accepted types for `value any`:
- Primitives: `string`, `int`, `int8`..`int64`, `uint`..`uint64`, `float32`, `float64`, `bool`, `time.Time`
- `Node` values (copy from another location or document)
- `[]any` and typed slices (creates arrays)
- `map[string]any` and typed maps (creates tables)

Auto-created intermediate table style (standard header vs inline) is configurable.

### String-Path API: Structure

```go
func (d *Document) NewTable(path string) error
func (d *Document) NewArrayTable(path string) error
```

### Fluent Cursor API

Never-nil cursor with accumulating errors. First error makes the cursor inert (all subsequent operations are no-ops). Check `Err()` at the end.

```go
func (d *Document) Key(name string) *Cursor
func (c *Cursor) Key(name string) *Cursor
func (c *Cursor) At(index int) *Cursor  // supports negative indices

func (c *Cursor) Node() Node
func (c *Cursor) String() (string, bool)
func (c *Cursor) Int() (int64, bool)
func (c *Cursor) Bool() (bool, bool)
func (c *Cursor) Float() (float64, bool)
func (c *Cursor) Time() (time.Time, bool)

func (c *Cursor) Err() error
```

### Explicit Path Resolution

For when proper error handling matters more than chaining.

```go
func (d *Document) Resolve(path string) (Node, error)
```

### Node Interface

```go
type Node interface {
    Type() NodeType
    Value() any
    Comment() string
    SetComment(comment string)
    LeadingComments() []string
    SetLeadingComments(comments []string)
    Raw() []byte  // raw bytes for clean nodes; re-rendered bytes for dirty nodes
}
```

### Rename Semantics

`Rename(path, newKey)` renames the last key segment at the given path. It does NOT move values across tables.

- `Rename("server.host", "address")` renames key `host` to `address` in `[server]`
- Preserves the value's trivia (comments, formatting)
- Errors if the path doesn't exist
- Errors if `newKey` already exists in the same table

### Delete Semantics

- `Delete("server.host")` removes the key-value pair
- `Delete("products[0]")` removes the array element; subsequent indices shift down (standard array semantics)
- `Delete` on a non-existent path is a silent no-op

### Format

Normalizes formatting while preserving all comments. Implementor chooses API shape from:
1. `doc.Format() []byte` + `doc.FormatWith(config) []byte`
2. `tomledit.NewFormatter(config).Format(doc) []byte`
3. `doc.Format(opts ...FormatOption) []byte`

Formatter behavior:
- Consistent key-value spacing: `key = value`
- One blank line between tables
- No trailing whitespace
- Multi-line arrays: one element per line if array exceeds configurable line width
- Comments stay attached to their associated nodes
- Configurable via `FormatConfig` (indentation, line width, etc.)

## Concurrency

- Safe concurrent reads: multiple goroutines can call Get/GetString/Resolve/cursor operations simultaneously after parsing. This is guaranteed by design (no lazy mutation on read paths), not by mutexes.
- Writes (Set, Delete, Rename, SetComment, etc.) require exclusive access. No concurrent reads during writes.
- Document-level: the unit of synchronization is one `*Document`.

## Parser

- Hand-written recursive descent (the only sane choice for trivia preservation)
- Lexer produces tokens with trivia attached
- Parser builds the AST tree from tokens
- Both lexer and parser are unexported (internal implementation details)
- Strict TOML 1.0 compliance: duplicate keys, table redefinition, and other spec violations are parse errors

### Parse Error Format

```go
type ParseError struct {
    Line    int
    Column  int
    Offset  int
    Snippet string  // a few chars of context around the error
    Message string  // "expected X, found Y"
}
```

### Edit Error Types

- Setting a child of a scalar: typed error
- Path resolution failure (when auto-create is off): typed error
- Type validation: reject Go types with no TOML representation
- Rename to existing key: typed error

## TOML 1.0 Feature Coverage

Full coverage. See `todo/original-idea.md` for the complete feature matrix.

## Testing Strategy

- Table-driven tests for all TOML 1.0 features
- Round-trip tests: parse and serialize every test case, assert byte-identical output
- Edit tests: parse, modify specific paths, assert only the modified region changed
- Official toml-test suite (https://github.com/toml-lang/toml-test) for spec compliance
- Fuzz testing for the parser (Go's built-in fuzzing)
- Benchmarks against pelletier/go-toml/v2 for parse performance

## Architecture Summary

```
source []byte
    |
    v
  Lexer (unexported)
    |  tokens with trivia
    v
  Parser (unexported)
    |  AST nodes, each holding raw bytes
    v
  *Document (public)
    |
    +-- String-path API: Get/Set/Delete/Rename
    +-- Fluent cursor API: Key().At().Key()
    +-- Explicit: Resolve(path) (Node, error)
    +-- Output: Bytes() (preserving), Format() (normalizing)
```

## Key Design Invariants

1. Clean nodes always emit their original bytes. No normalization, no whitespace changes.
2. Read operations never mutate internal state (enables safe concurrent reads).
3. Auto-create is never implicit. The caller always states intent.
4. Trivia belongs to nodes, not to whitespace between nodes. Every comment has exactly one owning node.
5. The parser is strict. If the input isn't valid TOML 1.0, Parse returns an error.
