package tomledit

// NodeType identifies the kind of AST node.
type NodeType int

const (
	NodeDocument NodeType = iota
	NodeTable
	NodeArrayTable
	NodeKeyValue
	NodeKey
	NodeString
	NodeInteger
	NodeFloat
	NodeBoolean
	NodeDateTime
	NodeLocalDateTime
	NodeLocalDate
	NodeLocalTime
	NodeArray
	NodeInlineTable
	NodeComment
)

var nodeTypeNames = [...]string{
	NodeDocument:      "Document",
	NodeTable:         "Table",
	NodeArrayTable:    "ArrayTable",
	NodeKeyValue:      "KeyValue",
	NodeKey:           "Key",
	NodeString:        "String",
	NodeInteger:       "Integer",
	NodeFloat:         "Float",
	NodeBoolean:       "Boolean",
	NodeDateTime:      "DateTime",
	NodeLocalDateTime: "LocalDateTime",
	NodeLocalDate:     "LocalDate",
	NodeLocalTime:     "LocalTime",
	NodeArray:         "Array",
	NodeInlineTable:   "InlineTable",
	NodeComment:       "Comment",
}

// String returns the human-readable name of the node type.
func (n NodeType) String() string {
	if int(n) >= 0 && int(n) < len(nodeTypeNames) {
		return nodeTypeNames[n]
	}
	return "Unknown"
}

// Trivia holds formatting and comment data attached to a node.
type Trivia struct {
	LeadingWhitespace []byte
	LeadingComments   [][]byte // each is a full "# ...\n" line
	InlineComment     []byte   // "# ..." after value on same line
	TrailingNewline   []byte
}

// Node is the interface implemented by all AST nodes.
type Node interface {
	Type() NodeType
	Value() any
	Comment() string
	SetComment(comment string)
	LeadingComments() []string
	SetLeadingComments(comments []string)
	Raw() []byte

	// unexported methods restrict implementation to this package
	setRaw([]byte)
	isDirty() bool
	markDirty()
	trivia() *Trivia
}

// nodeBase provides the shared implementation for all concrete node types.
type nodeBase struct {
	raw        []byte
	dirty      bool
	nodeTrivia Trivia
}

func (n *nodeBase) Raw() []byte {
	return n.raw
}

func (n *nodeBase) setRaw(b []byte) {
	n.raw = b
}

func (n *nodeBase) isDirty() bool {
	return n.dirty
}

func (n *nodeBase) markDirty() {
	n.dirty = true
}

func (n *nodeBase) trivia() *Trivia {
	return &n.nodeTrivia
}

func (n *nodeBase) Comment() string {
	return string(n.nodeTrivia.InlineComment)
}

func (n *nodeBase) SetComment(comment string) {
	n.nodeTrivia.InlineComment = []byte(comment)
	n.dirty = true
}

func (n *nodeBase) LeadingComments() []string {
	result := make([]string, len(n.nodeTrivia.LeadingComments))
	for i, c := range n.nodeTrivia.LeadingComments {
		result[i] = string(c)
	}
	return result
}

func (n *nodeBase) SetLeadingComments(comments []string) {
	n.nodeTrivia.LeadingComments = make([][]byte, len(comments))
	for i, c := range comments {
		n.nodeTrivia.LeadingComments[i] = []byte(c)
	}
	n.dirty = true
}

// nullNode provides no-op implementations of all Node interface methods.
// Internal virtual node types (dottedKeyView, dottedKeyGroup,
// compoundTableView, arrayTableCollection) embed nullNode and override
// only Type() and Value().
type nullNode struct{}

func (nullNode) Type() NodeType              { return NodeType(-1) }
func (nullNode) Value() any                  { return nil }
func (nullNode) Comment() string             { return "" }
func (nullNode) SetComment(string)           {}
func (nullNode) LeadingComments() []string   { return nil }
func (nullNode) SetLeadingComments([]string) {}
func (nullNode) Raw() []byte                 { return nil }
func (nullNode) setRaw([]byte)               {}
func (nullNode) isDirty() bool               { return false }
func (nullNode) markDirty()                  {}
func (nullNode) trivia() *Trivia             { return &Trivia{} }

// LocalDateTime represents a TOML local date-time (no timezone).
type LocalDateTime struct {
	Year, Month, Day       int
	Hour, Minute, Second   int
	Nanosecond             int
}

// LocalDate represents a TOML local date.
type LocalDate struct {
	Year, Month, Day int
}

// LocalTime represents a TOML local time.
type LocalTime struct {
	Hour, Minute, Second int
	Nanosecond           int
}
