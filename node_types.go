package tomledit

import "time"

// StringStyle indicates the quoting style for a string node.
type StringStyle int

const (
	StringBasic            StringStyle = iota // StringBasic is a double-quoted string ("...").
	StringLiteral                             // StringLiteral is a single-quoted string ('...').
	StringMultiLineBasic                      // StringMultiLineBasic is a triple-double-quoted string ("""...""").
	StringMultiLineLiteral                    // StringMultiLineLiteral is a triple-single-quoted string ('''...''').
)

// IntegerBase indicates the numeric base for an integer node.
type IntegerBase int

const (
	IntegerDecimal IntegerBase = iota // IntegerDecimal is base-10 (e.g. 42).
	IntegerHex                        // IntegerHex is base-16 (e.g. 0xFF).
	IntegerOctal                      // IntegerOctal is base-8 (e.g. 0o77).
	IntegerBinary                     // IntegerBinary is base-2 (e.g. 0b1010).
)

// DocumentNode is the root node of a TOML document.
type DocumentNode struct {
	nodeBase

	// Children holds the document's live top-level nodes (key-value pairs,
	// tables, array-tables, comments). This is the same slice Get/Resolve
	// traverse, not a copy. Reordering, inserting into, or removing from
	// this slice directly is not a supported mutation surface and is not
	// guaranteed to be reflected by Bytes(); use Set, Delete, NewTable, and
	// NewArrayTable to modify the document structure.
	Children []Node

	// leadingBOM holds a UTF-8 byte order mark (0xEF 0xBB 0xBF) found at the
	// very start of the parsed source, if any. It is nil for documents
	// without a leading BOM or created programmatically. Bytes prepends it
	// so a leading BOM round-trips byte-for-byte instead of being dropped.
	leadingBOM []byte
}

func (n *DocumentNode) Type() NodeType { return NodeDocument }
func (n *DocumentNode) Value() any     { return n.Children }

// TableNode represents a [table] header and its children.
//
// TableNode is normally created via DocumentNode.NewTable, which places it
// correctly in the document and marks it dirty so it renders from KeyPath.
// A directly-constructed TableNode (e.g. &TableNode{KeyPath: ...}) has no
// raw bytes and is not marked dirty; serialization still renders its header
// from KeyPath in that case, so the node does not vanish from the output.
type TableNode struct {
	nodeBase
	KeyPath []string

	// Children holds the table's live child nodes (the same slice Get
	// returns and traverses, not a copy). Reordering, inserting into, or
	// removing from this slice directly is not a supported mutation
	// surface and is not guaranteed to be reflected by Bytes(); use Set
	// and Delete to modify the table's contents.
	Children []Node
}

func (n *TableNode) Type() NodeType { return NodeTable }
func (n *TableNode) Value() any     { return n.Children }

// ArrayTableNode represents an [[array-table]] header and its children.
//
// ArrayTableNode is normally created via DocumentNode.NewArrayTable, which
// places it correctly in the document and marks it dirty so it renders from
// KeyPath. A directly-constructed ArrayTableNode (e.g.
// &ArrayTableNode{KeyPath: ...}) has no raw bytes and is not marked dirty;
// serialization still renders its header from KeyPath in that case, so the
// node does not vanish from the output.
type ArrayTableNode struct {
	nodeBase
	KeyPath []string

	// Children holds the array-table entry's live child nodes (the same
	// slice Get returns and traverses, not a copy). Reordering, inserting
	// into, or removing from this slice directly is not a supported
	// mutation surface and is not guaranteed to be reflected by Bytes();
	// use Set and Delete to modify the entry's contents.
	Children []Node
}

func (n *ArrayTableNode) Type() NodeType { return NodeArrayTable }
func (n *ArrayTableNode) Value() any     { return n.Children }

// KeyValueNode represents a key = value pair.
type KeyValueNode struct {
	nodeBase
	Key   *KeyNode
	Val   Node
}

func (n *KeyValueNode) Type() NodeType { return NodeKeyValue }
func (n *KeyValueNode) Value() any     { return n.Val }

// KeyNode represents a (possibly dotted) key.
type KeyNode struct {
	nodeBase
	Parts    []string   // semantic parts (e.g. ["server", "host"])
	RawParts [][]byte   // original bytes for each part
	Styles   []StringStyle // quoting style per part
}

func (n *KeyNode) Type() NodeType { return NodeKey }
func (n *KeyNode) Value() any     { return n.Parts }

// StringNode represents a string value.
type StringNode struct {
	nodeBase
	Val   string
	Style StringStyle
}

func (n *StringNode) Type() NodeType { return NodeString }
func (n *StringNode) Value() any     { return n.Val }

// IntegerNode represents an integer value.
type IntegerNode struct {
	nodeBase
	Val  int64
	Base IntegerBase
}

func (n *IntegerNode) Type() NodeType { return NodeInteger }
func (n *IntegerNode) Value() any     { return n.Val }

// FloatNode represents a float value.
type FloatNode struct {
	nodeBase
	Val float64
}

func (n *FloatNode) Type() NodeType { return NodeFloat }
func (n *FloatNode) Value() any     { return n.Val }

// BooleanNode represents a boolean value.
type BooleanNode struct {
	nodeBase
	Val bool
}

func (n *BooleanNode) Type() NodeType { return NodeBoolean }
func (n *BooleanNode) Value() any     { return n.Val }

// DateTimeNode represents an offset date-time value.
type DateTimeNode struct {
	nodeBase
	Val time.Time
}

func (n *DateTimeNode) Type() NodeType { return NodeDateTime }
func (n *DateTimeNode) Value() any     { return n.Val }

// LocalDateTimeNode represents a local date-time value (no timezone).
type LocalDateTimeNode struct {
	nodeBase
	Val LocalDateTime
}

func (n *LocalDateTimeNode) Type() NodeType { return NodeLocalDateTime }
func (n *LocalDateTimeNode) Value() any     { return n.Val }

// LocalDateNode represents a local date value.
type LocalDateNode struct {
	nodeBase
	Val LocalDate
}

func (n *LocalDateNode) Type() NodeType { return NodeLocalDate }
func (n *LocalDateNode) Value() any     { return n.Val }

// LocalTimeNode represents a local time value.
type LocalTimeNode struct {
	nodeBase
	Val LocalTime
}

func (n *LocalTimeNode) Type() NodeType { return NodeLocalTime }
func (n *LocalTimeNode) Value() any     { return n.Val }

// ArrayNode represents an array value.
type ArrayNode struct {
	nodeBase

	// Elements holds the array's live element nodes. This is the same
	// slice Get returns, not a copy. Reassigning, slicing, appending to,
	// or otherwise mutating this slice directly is NOT reflected by
	// Bytes() -- doing so does not mark the node dirty, so a clean
	// parsed array re-renders from its original raw bytes and the
	// mutation is silently dropped. Use Set (to replace an index or the
	// whole array) and Delete (to remove an index, e.g. "arr[0]") to
	// modify array contents; those APIs mark the node dirty for you.
	Elements []Node

	TrailingComments [][]byte // comments after the last element, before ']'
}

func (n *ArrayNode) Type() NodeType { return NodeArray }
func (n *ArrayNode) Value() any     { return n.Elements }

// InlineTableNode represents an inline table value.
type InlineTableNode struct {
	nodeBase

	// Children holds the inline table's live KeyValueNode entries. This is
	// the same slice Get returns, not a copy. Reordering, inserting into,
	// or removing from this slice directly is NOT reflected by Bytes() --
	// doing so does not mark the node dirty, so a clean parsed inline
	// table re-renders from its original raw bytes and the mutation is
	// silently dropped. Use Set and Delete to modify entries; those APIs
	// mark the node dirty for you.
	Children []Node // KeyValueNode entries
}

func (n *InlineTableNode) Type() NodeType { return NodeInlineTable }
func (n *InlineTableNode) Value() any     { return n.Children }

// CommentNode represents a standalone comment line.
type CommentNode struct {
	nodeBase
	Text string
}

func (n *CommentNode) Type() NodeType { return NodeComment }
func (n *CommentNode) Value() any     { return n.Text }
