package tomledit

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Set updates the value at the given path. If the final key does not exist in
// an existing parent, it is created as a new key-value pair. Returns an error
// if intermediate path segments do not exist.
//
// Supported value types: string, bool, int/int8-64, uint/uint8-64, float32/64,
// time.Time, LocalDateTime, LocalDate, LocalTime, []any, map[string]any, and
// any type implementing the Node interface. Use SetCreate to auto-create
// intermediate tables.
func (d *DocumentNode) Set(path string, value any) error {
	return d.setInternal(path, value, false)
}

// SetCreate is like Set but auto-creates intermediate [table] headers when they
// do not exist. Missing tables are appended to the document. This is convenient
// for inserting values into deeply nested paths that may not yet exist.
func (d *DocumentNode) SetCreate(path string, value any) error {
	return d.setInternal(path, value, true)
}

func (d *DocumentNode) setInternal(path string, value any, create bool) error {
	segments, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("path syntax error: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Reject -- before making any change -- an attempt to set a key whose
	// full path already names an existing [table] or [[array-table]].
	// Without this check, setKeyInChildren's search (below) only looks for
	// a sibling *KeyValueNode* with a matching key and is blind to a
	// same-named *TableNode*/*ArrayTableNode*, so it falls through to
	// appending a brand-new top-level KV. That KV then serializes directly
	// after the still-open table header with no intervening header, so on
	// re-parse it is silently swallowed as a *nested* key inside that table
	// instead of the new top-level value the caller asked for (BUG H-1).
	if kind, ok := d.collidesWithTable(segments); ok {
		return fmt.Errorf("cannot set key %q: already defined as a %s", path, kind)
	}

	// The last segment is the target key/index to set.
	parentSegs := segments[:len(segments)-1]
	lastSeg := segments[len(segments)-1]

	// Resolve the parent node.
	parent, err := d.resolveParentForEdit(parentSegs, create)
	if err != nil {
		return err
	}

	// Convert value to a node.
	valNode, err := valueToNode(value)
	if err != nil {
		return err
	}

	switch lastSeg.Type {
	case keySegment:
		return setKeyInParent(parent, lastSeg.Key, valNode)
	case indexSegment:
		return setIndexInParent(parent, lastSeg.Index, valNode)
	default:
		return fmt.Errorf("unknown segment type")
	}
}

// collidesWithTable reports whether the full path (built from segments,
// which must consist entirely of key segments -- an index anywhere in the
// path can never match a table's KeyPath) matches the exact KeyPath of an
// existing *TableNode or *ArrayTableNode in the document's top-level
// children. All tables and array-tables -- at any nesting depth -- are
// stored flat in d.Children with a compound KeyPath (mirroring how
// resolveKeyInDocument/resolveKeyInTable/etc. already look them up), so a
// single flat scan is sufficient to catch a collision regardless of where
// in the document the target path lives.
func (d *DocumentNode) collidesWithTable(segments []pathSegment) (kind string, ok bool) {
	keyPath := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg.Type != keySegment {
			return "", false
		}
		keyPath = append(keyPath, seg.Key)
	}
	return d.collidesWithTableAtPath(keyPath)
}

// collidesWithTableAtPath is collidesWithTable's path-based core: it reports
// whether keyPath matches the exact KeyPath of an existing *TableNode or
// *ArrayTableNode in the document's top-level children. Factored out so
// callers that already have a built []string key path (NewTable,
// NewArrayTable, keyPathExistsAsAnyKind) don't need to round-trip through
// []pathSegment just to reuse the scan.
func (d *DocumentNode) collidesWithTableAtPath(keyPath []string) (kind string, ok bool) {
	for _, child := range d.Children {
		switch n := child.(type) {
		case *TableNode:
			if pathsEqual(n.KeyPath, keyPath) {
				return "table", true
			}
		case *ArrayTableNode:
			if pathsEqual(n.KeyPath, keyPath) {
				return "array table", true
			}
		}
	}
	return "", false
}

// keyPathExistsAsAnyKind reports whether keyPath already names an existing
// top-level node in the document, regardless of its kind: a *TableNode, an
// *ArrayTableNode, or a top-level *KeyValueNode (matched against its full,
// possibly dotted, key). NewTable, NewArrayTable, and Rename each used to
// check only for a collision against their OWN node kind (or, in Rename's
// case, only against sibling *KeyValueNode*s) -- so creating/renaming onto
// a path that already existed as a *different* kind silently produced a
// document with two top-level definitions for the same key path. That
// serializes to TOML the package's own parser then rejects as a
// redefinition, i.e. a nil error paired with unparseable output (BUG H-2,
// see audit_repro_test.go). This helper centralizes the cross-kind check so
// all three entry points can reject the collision before any mutation.
func keyPathExistsAsAnyKind(d *DocumentNode, keyPath []string) (kind string, ok bool) {
	if kind, ok := d.collidesWithTableAtPath(keyPath); ok {
		return kind, ok
	}
	for _, child := range d.Children {
		if kv, ok := child.(*KeyValueNode); ok {
			if pathsEqual(kv.Key.Parts, keyPath) {
				return "key-value", true
			}
		}
	}
	return "", false
}

// resolveParentForEdit resolves the parent container for an edit operation.
// When create is true, missing intermediate tables are auto-created.
func (d *DocumentNode) resolveParentForEdit(segments []pathSegment, create bool) (Node, error) {
	if len(segments) == 0 {
		return d, nil
	}

	// Try resolving the full parent path first.
	node, err := resolveNodeForEdit(d, segments)
	if err == nil {
		return node, nil
	}

	if !create {
		return nil, fmt.Errorf("parent path not found: %w", err)
	}

	// Auto-create mode: walk segments, creating tables as needed.
	return d.resolveOrCreateParent(segments)
}

// resolveOrCreateParent walks the path segments, creating intermediate tables
// as needed. Returns the final parent container.
func (d *DocumentNode) resolveOrCreateParent(segments []pathSegment) (Node, error) {
	var current Node = d
	var currentTablePath []string

	for _, seg := range segments {
		if seg.Type != keySegment {
			// For index segments, the collection must already exist.
			next, err := resolveIndexSegment(d, current, currentTablePath, seg.Index)
			if err != nil {
				return nil, fmt.Errorf("cannot auto-create array index: %w", err)
			}
			current = next
			continue
		}

		// Try to resolve this key segment.
		next, tablePath, err := resolveKeySegment(d, current, currentTablePath, seg.Key, nil, 0)
		if err == nil {
			current = next
			currentTablePath = tablePath
			continue
		}

		// Key not found -- create a table for it.
		tablePath, err = d.createIntermediateTable(current, currentTablePath, seg.Key)
		if err != nil {
			return nil, err
		}

		// Re-resolve to get the newly created table.
		next, tablePath, err = resolveKeySegment(d, current, currentTablePath, seg.Key, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve after creation: %w", err)
		}
		current = next
		currentTablePath = tablePath
	}

	return current, nil
}

// createIntermediateTable creates a new [table] for the given key under the
// current scope. Returns the updated table path.
func (d *DocumentNode) createIntermediateTable(current Node, currentTablePath []string, key string) ([]string, error) {
	var newPath []string

	switch scope := current.(type) {
	case *DocumentNode:
		newPath = []string{key}
	case *TableNode:
		newPath = append(append([]string(nil), scope.KeyPath...), key)
	case *ArrayTableNode:
		newPath = append(append([]string(nil), scope.KeyPath...), key)
	default:
		return nil, fmt.Errorf("cannot create intermediate table under %s node", current.Type())
	}

	tbl := &TableNode{
		KeyPath: newPath,
	}
	tbl.markDirty()
	tbl.nodeTrivia.TrailingNewline = []byte("\n")

	d.Children = append(d.Children, tbl)
	return newPath, nil
}

// resolveNodeForEdit is like resolveNode but returns container nodes (tables,
// documents, inline tables) without unwrapping KeyValueNodes whose value is an
// inline table or array. This allows the edit operations to find the right
// parent for insertion.
func resolveNodeForEdit(doc *DocumentNode, segments []pathSegment) (Node, error) {
	if len(segments) == 0 {
		return doc, nil
	}

	var current Node = doc
	var currentTablePath []string

	for i, seg := range segments {
		switch seg.Type {
		case keySegment:
			node, tablePath, err := resolveKeySegment(doc, current, currentTablePath, seg.Key, segments, i)
			if err != nil {
				return nil, err
			}
			current = node
			currentTablePath = tablePath

		case indexSegment:
			node, err := resolveIndexSegment(doc, current, currentTablePath, seg.Index)
			if err != nil {
				return nil, err
			}
			current = node

		default:
			return nil, fmt.Errorf("unknown segment type")
		}
	}

	return current, nil
}

// setKeyInParent sets or replaces a key-value in a parent container.
func setKeyInParent(parent Node, key string, valNode Node) error {
	switch p := parent.(type) {
	case *DocumentNode:
		return setKeyInChildren(&p.Children, key, valNode, false)
	case *TableNode:
		return setKeyInChildren(&p.Children, key, valNode, false)
	case *ArrayTableNode:
		return setKeyInChildren(&p.Children, key, valNode, false)
	case *InlineTableNode:
		return setKeyInChildren(&p.Children, key, valNode, true)
	case *dottedKeyView:
		// The dotted key view points into a KV's value. If the value is an
		// inline table, set inside it.
		if p.partIndex >= len(p.kv.Key.Parts) {
			return setKeyInParent(p.kv.Val, key, valNode)
		}
		if p.kv.Key.Parts[p.partIndex] == key && p.partIndex+1 >= len(p.kv.Key.Parts) {
			// key is the terminal dotted-key part (e.g. "b" in "a.b = 1"):
			// update the existing KV's value in place rather than treating
			// this as an intermediate segment.
			carryStringStyle(p.kv.Val, valNode)
			p.kv.Val = valNode
			p.kv.markDirty()
			return nil
		}
		return fmt.Errorf("cannot set key %q: intermediate dotted key view", key)
	default:
		return fmt.Errorf("cannot set key %q in %s node", key, parent.Type())
	}
}

// setKeyInChildren searches children for an existing KV with the given key.
// If found, replaces its value. Otherwise, appends a new KV.
func setKeyInChildren(children *[]Node, key string, valNode Node, markParentDirty bool) error {
	for _, child := range *children {
		if kv, ok := child.(*KeyValueNode); ok {
			if len(kv.Key.Parts) == 1 && kv.Key.Parts[0] == key {
				carryStringStyle(kv.Val, valNode)
				kv.Val = valNode
				kv.markDirty()
				return nil
			}
		}
	}
	// Key not found: create a new KV and append.
	kv := newKeyValueNode(key, valNode)
	*children = append(*children, kv)
	return nil
}

// carryStringStyle preserves the old string node's quoting style onto the
// replacement node, so that Set doesn't silently flip a literal/multi-line
// value to basic quoting. It only carries the style when both the old and
// new values are strings, and only when the new value can be safely
// represented in the old style -- otherwise the replacement keeps its
// default StringBasic style (set by valueToNode), which can always
// represent any string via escaping. This guards the non-negotiable
// invariant that Bytes() output must always be valid, re-parseable TOML.
func carryStringStyle(oldVal, newVal Node) {
	oldStr, ok := oldVal.(*StringNode)
	if !ok {
		return
	}
	newStr, ok := newVal.(*StringNode)
	if !ok {
		return
	}
	if canRepresentString(newStr.Val, oldStr.Style) {
		newStr.Style = oldStr.Style
	}
}

// canRepresentString reports whether val can be safely rendered in the given
// string style without falling back to a different style. Conservative by
// design: when in doubt, it returns false so the caller falls back to
// StringBasic rather than risk emitting invalid TOML.
func canRepresentString(val string, style StringStyle) bool {
	switch style {
	case StringBasic:
		// Basic strings can represent any string via escaping.
		return true

	case StringLiteral:
		// No escapes available: no apostrophe, no line breaks, no control
		// characters other than tab.
		if strings.ContainsRune(val, '\'') {
			return false
		}
		for _, r := range val {
			if r == '\n' || r == '\r' {
				return false
			}
			if r != '\t' && (r < 0x20 || r == 0x7F) {
				return false
			}
		}
		return true

	case StringMultiLineLiteral:
		// No escapes available: the content must not contain the closing
		// delimiter sequence, must not end in a quote (which would collide
		// with the closing delimiter), and must not contain control
		// characters other than tab/newline.
		if strings.Contains(val, "'''") {
			return false
		}
		if strings.HasSuffix(val, "'") {
			return false
		}
		for _, r := range val {
			if r == '\t' || r == '\n' {
				continue
			}
			if r < 0x20 || r == 0x7F {
				return false
			}
		}
		return true

	case StringMultiLineBasic:
		// renderMultiLineBasicString fully escapes quotes, backslashes, and
		// control characters, so any value is representable.
		return true

	default:
		return false
	}
}

// setIndexInParent replaces an element at the given index in an array.
func setIndexInParent(parent Node, index int, valNode Node) error {
	switch p := parent.(type) {
	case *ArrayNode:
		idx, err := normalizeIndex(index, len(p.Elements))
		if err != nil {
			return err
		}
		// Transfer trivia (leading comments, leading whitespace, inline
		// comment) from the old element to the new one so that comments
		// on the replaced element survive re-rendering.
		oldTrivia := p.Elements[idx].trivia()
		newTrivia := valNode.trivia()
		newTrivia.LeadingComments = oldTrivia.LeadingComments
		newTrivia.LeadingWhitespace = oldTrivia.LeadingWhitespace
		newTrivia.InlineComment = oldTrivia.InlineComment
		p.Elements[idx] = valNode
		p.markDirty()
		return nil
	case *KeyValueNode:
		return setIndexInParent(p.Val, index, valNode)
	default:
		return fmt.Errorf("cannot set index [%d] in %s node", index, parent.Type())
	}
}

// newKeyValueNode creates a dirty KeyValueNode for the given key and value.
func newKeyValueNode(key string, val Node) *KeyValueNode {
	keyNode := &KeyNode{
		Parts:    []string{key},
		RawParts: [][]byte{[]byte(key)},
		Styles:   []StringStyle{StringBasic},
	}
	keyNode.markDirty()

	kv := &KeyValueNode{
		Key: keyNode,
		Val: val,
	}
	kv.markDirty()
	kv.nodeTrivia.TrailingNewline = []byte("\n")
	return kv
}

// Delete removes the node at the given path from the document. It handles
// key-value pairs, tables, array-of-tables, and array elements. Returns nil
// (no error) if the path does not exist, making it safe to call unconditionally.
func (d *DocumentNode) Delete(path string) error {
	segments, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("path syntax error: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	parentSegs := segments[:len(segments)-1]
	lastSeg := segments[len(segments)-1]

	// Resolve the parent container.
	parent, err := d.resolveParentForEdit(parentSegs, false)
	if err != nil {
		// Parent doesn't exist -- silent no-op.
		return nil
	}

	switch lastSeg.Type {
	case keySegment:
		return d.deleteKeyFromParent(parent, lastSeg.Key)
	case indexSegment:
		return deleteIndexFromParent(parent, lastSeg.Index)
	default:
		return fmt.Errorf("unknown segment type")
	}
}

// deleteKeyFromParent removes a key from a parent container.
func (d *DocumentNode) deleteKeyFromParent(parent Node, key string) error {
	switch p := parent.(type) {
	case *DocumentNode:
		deleteKeyFromChildren(&p.Children, key)
		d.deleteTableOrArrayTableByFirstKey(key)
		return nil
	case *TableNode:
		deleteKeyFromChildren(&p.Children, key)
		d.deleteSubTableByKey(p.KeyPath, key)
		return nil
	case *ArrayTableNode:
		deleteKeyFromChildren(&p.Children, key)
		return nil
	case *InlineTableNode:
		deleteKeyFromChildren(&p.Children, key)
		p.markDirty()
		return nil
	case *dottedKeyGroup:
		// p represents multiple KVs sharing a dotted-key prefix (e.g.
		// "a.b = 1" and "a.c = 2" both sharing prefix "a"). Find every KV
		// whose part at this group's depth matches key and remove it
		// entirely from the owning children slice: TOML forbids a dotted
		// key from existing both as a scalar and as a further-nested
		// prefix at the same time, so every matching KV -- whether key is
		// its terminal part or it continues deeper -- lies wholly beneath
		// the path being deleted and must go in full.
		for _, kv := range p.kvs {
			if p.depth < len(kv.Key.Parts) && kv.Key.Parts[p.depth] == key {
				removeKVByIdentity(p.children, kv)
			}
		}
		return nil
	default:
		// Silent no-op for unsupported parent types.
		return nil
	}
}

// removeKVByIdentity removes the KeyValueNode matching target (by pointer
// identity) from children, if present.
func removeKVByIdentity(children *[]Node, target *KeyValueNode) {
	if children == nil {
		return
	}
	for i, child := range *children {
		if kv, ok := child.(*KeyValueNode); ok && kv == target {
			*children = append((*children)[:i], (*children)[i+1:]...)
			return
		}
	}
}

// deleteKeyFromChildren removes the first KV with the given key from a children slice.
func deleteKeyFromChildren(children *[]Node, key string) {
	for i, child := range *children {
		if kv, ok := child.(*KeyValueNode); ok {
			if len(kv.Key.Parts) > 0 && kv.Key.Parts[0] == key {
				*children = append((*children)[:i], (*children)[i+1:]...)
				return
			}
		}
	}
}

// deleteTableOrArrayTableByFirstKey removes a table or array-table from the
// document's top-level children by matching the key path.
func (d *DocumentNode) deleteTableOrArrayTableByFirstKey(key string) {
	targetPath := []string{key}
	i := 0
	for i < len(d.Children) {
		child := d.Children[i]
		switch n := child.(type) {
		case *TableNode:
			if len(n.KeyPath) > 0 && n.KeyPath[0] == key {
				d.Children = append(d.Children[:i], d.Children[i+1:]...)
				continue
			}
		case *ArrayTableNode:
			if pathsEqual(n.KeyPath, targetPath) {
				d.Children = append(d.Children[:i], d.Children[i+1:]...)
				continue
			}
		}
		i++
	}
}

// deleteSubTableByKey removes a sub-table from the document's children.
func (d *DocumentNode) deleteSubTableByKey(parentPath []string, key string) {
	targetPath := append(append([]string(nil), parentPath...), key)
	i := 0
	for i < len(d.Children) {
		child := d.Children[i]
		switch n := child.(type) {
		case *TableNode:
			if pathsEqual(n.KeyPath, targetPath) {
				d.Children = append(d.Children[:i], d.Children[i+1:]...)
				continue
			}
		case *ArrayTableNode:
			if pathsEqual(n.KeyPath, targetPath) {
				d.Children = append(d.Children[:i], d.Children[i+1:]...)
				continue
			}
		}
		i++
	}
}

// deleteIndexFromParent removes an element at the given index.
func deleteIndexFromParent(parent Node, index int) error {
	switch p := parent.(type) {
	case *ArrayNode:
		if len(p.Elements) == 0 {
			return nil // silent no-op
		}
		idx, err := normalizeIndex(index, len(p.Elements))
		if err != nil {
			return nil // silent no-op for out-of-range
		}
		p.Elements = append(p.Elements[:idx], p.Elements[idx+1:]...)
		p.markDirty()
		return nil
	case *arrayTableCollection:
		if len(p.entries) == 0 {
			return nil
		}
		idx, err := normalizeIndex(index, len(p.entries))
		if err != nil {
			return nil
		}
		// Remove the ArrayTableNode and every sub-table/array-table scoped
		// to this specific entry (e.g. [x.sub], [x.sub.deep]) -- otherwise
		// those orphaned nodes are left in front of the next [[x]] entry,
		// producing output that either fails to re-parse or silently
		// re-scopes to the wrong entry.
		target := p.entries[idx]
		scopeIdx := -1
		for i, child := range p.doc.Children {
			if child == target {
				scopeIdx = i
				break
			}
		}
		if scopeIdx == -1 {
			return nil
		}
		toRemove := map[int]bool{scopeIdx: true}
		for _, i := range scopedDescendantIndices(p.doc, scopeIdx, target.KeyPath, target.KeyPath) {
			toRemove[i] = true
		}
		kept := p.doc.Children[:0:0]
		for i, child := range p.doc.Children {
			if !toRemove[i] {
				kept = append(kept, child)
			}
		}
		p.doc.Children = kept
		return nil
	case *KeyValueNode:
		return deleteIndexFromParent(p.Val, index)
	default:
		return nil // silent no-op
	}
}

// Rename changes the key name of the node at the given path to newKey.
// Returns an error if the path does not exist, if newKey conflicts with an
// existing sibling key, or if the last path segment is an array index (only
// key segments can be renamed).
func (d *DocumentNode) Rename(path string, newKey string) error {
	segments, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("path syntax error: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// All segments must be key segments for rename to make sense.
	lastSeg := segments[len(segments)-1]
	if lastSeg.Type != keySegment {
		return fmt.Errorf("cannot rename an array index")
	}

	parentSegs := segments[:len(segments)-1]

	parent, err := d.resolveParentForEdit(parentSegs, false)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	// Reject -- before any mutation -- renaming onto a NEW full path that
	// already names an existing top-level node of ANY kind (table,
	// array-table, or scalar key-value). The old duplicate check inside
	// renameKeyInParent only scanned sibling *KeyValueNode*s, so renaming
	// "name" to "server" when a top-level [server] table already existed
	// slipped through and produced a document with two conflicting
	// top-level "server" definitions and a nil error (BUG H-2). Only
	// checked when every parent segment is a key segment -- an index
	// segment anywhere in the path means the rename target lives inside
	// an array element, which isn't represented by a flat top-level
	// KeyPath and so can't collide with one.
	if newPath, ok := renamedKeyPath(parentSegs, newKey); ok {
		if kind, exists := keyPathExistsAsAnyKind(d, newPath); exists {
			return fmt.Errorf("cannot rename %q to %q: %q already exists as a %s", path, newKey, newKey, kind)
		}
	}

	return renameKeyInParent(parent, lastSeg.Key, newKey)
}

// renamedKeyPath builds the full top-level key path that a rename's NEW
// name would occupy: parentSegs' keys followed by newKey. Returns ok=false
// if parentSegs contains an index segment, since such a path cannot match
// any *TableNode*/*ArrayTableNode*'s flat KeyPath (see keyPathExistsAsAnyKind).
func renamedKeyPath(parentSegs []pathSegment, newKey string) ([]string, bool) {
	path := make([]string, 0, len(parentSegs)+1)
	for _, seg := range parentSegs {
		if seg.Type != keySegment {
			return nil, false
		}
		path = append(path, seg.Key)
	}
	path = append(path, newKey)
	return path, true
}

// renameKeyInParent renames a key inside a parent container.
func renameKeyInParent(parent Node, oldKey, newKey string) error {
	var children *[]Node

	switch p := parent.(type) {
	case *DocumentNode:
		children = &p.Children
	case *TableNode:
		children = &p.Children
	case *ArrayTableNode:
		children = &p.Children
	case *InlineTableNode:
		children = &p.Children
	default:
		return fmt.Errorf("cannot rename key in %s node", parent.Type())
	}

	// Check for duplicate: does newKey already exist?
	for _, child := range *children {
		if kv, ok := child.(*KeyValueNode); ok {
			if len(kv.Key.Parts) == 1 && kv.Key.Parts[0] == newKey {
				return fmt.Errorf("key %q already exists in parent", newKey)
			}
		}
	}

	// Find the KV with the old key.
	for _, child := range *children {
		if kv, ok := child.(*KeyValueNode); ok {
			if len(kv.Key.Parts) > 0 && kv.Key.Parts[0] == oldKey {
				// Update the last matching part (for simple keys, index 0).
				kv.Key.Parts[0] = newKey
				kv.Key.RawParts[0] = []byte(newKey)
				kv.Key.markDirty()
				kv.markDirty()
				return nil
			}
		}
	}

	return fmt.Errorf("key %q not found", oldKey)
}

// NewTable creates a new [table] header at the given path and appends it to
// the document. The path must consist of key segments only (no array indices).
// Returns an error if a table with that exact path already exists.
func (d *DocumentNode) NewTable(path string) error {
	segments, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("path syntax error: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Build the key path from segments.
	keyPath := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg.Type != keySegment {
			return fmt.Errorf("NewTable path must contain only key segments, not indices")
		}
		keyPath = append(keyPath, seg.Key)
	}

	// Reject -- before any mutation -- creating a table whose path already
	// names an existing node of ANY kind (table, array-table, or a scalar
	// key-value). The old check here only scanned for a same-kind
	// *TableNode* collision, so creating [server] over an existing scalar
	// `server = "..."` or an existing [[server]] array-table silently
	// appended a second top-level "server" definition and returned nil --
	// output the package's own parser then rejects as a redefinition
	// (BUG H-2).
	if kind, ok := keyPathExistsAsAnyKind(d, keyPath); ok {
		return fmt.Errorf("cannot create table [%s]: already defined as a %s", joinPath(keyPath), kind)
	}

	tbl := &TableNode{
		KeyPath: keyPath,
	}
	tbl.markDirty()
	tbl.nodeTrivia.TrailingNewline = []byte("\n")

	d.Children = append(d.Children, tbl)
	return nil
}

// NewArrayTable inserts a new [[array-table]] entry at the given path. If
// entries with this path already exist, the new entry is placed right after
// the last existing entry and everything scoped to it (nested sub-tables
// such as [x.sub] or [[x.y]]), keeping the whole group together instead of
// scattering entries across the document. If no entry exists yet, the new
// entry is appended at the end of the document. Multiple entries with the
// same path are valid in TOML and represent successive elements of the
// array. The path must consist of key segments only (no array indices).
func (d *DocumentNode) NewArrayTable(path string) error {
	segments, err := parsePath(path)
	if err != nil {
		return fmt.Errorf("path syntax error: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	keyPath := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg.Type != keySegment {
			return fmt.Errorf("NewArrayTable path must contain only key segments, not indices")
		}
		keyPath = append(keyPath, seg.Key)
	}

	// Reject -- before any mutation -- adding an array-table entry whose
	// path already names an existing node of a DIFFERENT kind (a plain
	// [table] or a scalar key-value). Appending another [[x]] entry when
	// [[x]] already exists as an array-table is legal (that's how
	// successive entries are added), so only a table/key-value collision
	// is an error here; NewArrayTable previously performed NO existence
	// check at all, silently producing a document with two conflicting
	// top-level definitions and a nil error (BUG H-2).
	if kind, ok := keyPathExistsAsAnyKind(d, keyPath); ok && kind != "array table" {
		return fmt.Errorf("cannot create array table [[%s]]: already defined as a %s", joinPath(keyPath), kind)
	}

	atbl := &ArrayTableNode{
		KeyPath: keyPath,
	}
	atbl.markDirty()
	atbl.nodeTrivia.TrailingNewline = []byte("\n")

	// Find the last existing entry for this path, if any, and insert right
	// after it and its scoped descendants (reusing the same helper the
	// array-table delete path uses to compute an entry's full extent).
	lastIdx := -1
	for i, child := range d.Children {
		if at, ok := child.(*ArrayTableNode); ok && pathsEqual(at.KeyPath, keyPath) {
			lastIdx = i
		}
	}

	insertAt := len(d.Children)
	if lastIdx >= 0 {
		insertAt = lastIdx + 1
		if scoped := scopedDescendantIndices(d, lastIdx, keyPath, keyPath); len(scoped) > 0 {
			insertAt = scoped[len(scoped)-1] + 1
		}
	}

	children := make([]Node, 0, len(d.Children)+1)
	children = append(children, d.Children[:insertAt]...)
	children = append(children, atbl)
	children = append(children, d.Children[insertAt:]...)
	d.Children = children
	return nil
}

// valueToNode converts a Go value to the appropriate AST node.
// All created nodes are dirty (no raw bytes).
//
// This is the public (package-internal) entry point: each call starts a
// fresh cycle-detection descent. Recursive calls made while already inside a
// descent (from sliceToArrayNode / mapToInlineTableNode / the marshal path)
// call valueToNodeVisited directly instead, threading the same visited set
// so a true cycle -- a map or slice that references itself, directly or
// through intermediate containers -- is detected instead of recursing until
// the stack overflows.
func valueToNode(v any) (Node, error) {
	return valueToNodeVisited(v, make(map[uintptr]bool))
}

// valueToNodeVisited is valueToNode's recursive worker. visited holds the
// identities (map data pointer / slice data pointer) of containers
// currently on the descent path; it is threaded through and shared with
// sliceToArrayNode and mapToInlineTableNode.
func valueToNodeVisited(v any, visited map[uintptr]bool) (Node, error) {
	if v == nil {
		return nil, fmt.Errorf("unsupported type: nil")
	}

	// Check if it already implements Node.
	if n, ok := v.(Node); ok {
		// A hand-built Node supplied directly by the caller (per Set/
		// SetCreate's documented "any type implementing the Node
		// interface" support) has a zero-value embedded nodeBase:
		// dirty=false and raw=nil. serializeNode's "clean, use cached
		// bytes" branches key off !isDirty(), so without marking it
		// dirty here the node would be mistaken for an already-rendered
		// parsed node and its empty Raw() would be emitted verbatim --
		// producing invalid/empty TOML (BUG H-3, see
		// audit_repro_test.go). Mark the whole subtree dirty so every
		// descendant (array elements, inline-table entries) also
		// re-renders from its semantic fields rather than its own
		// (equally empty) cached Raw().
		markSubtreeDirty(n)
		return n, nil
	}

	switch val := v.(type) {
	case string:
		n := &StringNode{Val: val, Style: StringBasic}
		n.markDirty()
		return n, nil

	case bool:
		n := &BooleanNode{Val: val}
		n.markDirty()
		return n, nil

	case int:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case int8:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case int16:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case int32:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case int64:
		n := &IntegerNode{Val: val, Base: IntegerDecimal}
		n.markDirty()
		return n, nil

	case uint:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case uint8:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case uint16:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case uint32:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil
	case uint64:
		n := &IntegerNode{Val: int64(val), Base: IntegerDecimal}
		n.markDirty()
		return n, nil

	case float32:
		n := &FloatNode{Val: float64(val)}
		n.markDirty()
		return n, nil
	case float64:
		n := &FloatNode{Val: val}
		n.markDirty()
		return n, nil

	case time.Time:
		n := &DateTimeNode{Val: val}
		n.markDirty()
		return n, nil

	case LocalDateTime:
		n := &LocalDateTimeNode{Val: val}
		n.markDirty()
		return n, nil

	case LocalDate:
		n := &LocalDateNode{Val: val}
		n.markDirty()
		return n, nil

	case LocalTime:
		n := &LocalTimeNode{Val: val}
		n.markDirty()
		return n, nil

	case []any:
		return sliceToArrayNode(val, visited)

	case map[string]any:
		return mapToInlineTableNode(val, visited)
	}

	// Use reflection for typed slices (e.g., []string, []int).
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		items := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items[i] = rv.Index(i).Interface()
		}
		return sliceToArrayNode(items, visited)
	}

	return nil, fmt.Errorf("unsupported type: %T", v)
}

// markSubtreeDirty marks n, and (for container node types) every descendant
// reachable from it, dirty. It exists to fix up a hand-built Node supplied
// directly to Set/SetCreate: such a node -- and any nested Node it contains
// -- carries a zero-value embedded nodeBase (dirty=false, raw=nil), which
// serializeNode's "clean, use cached bytes" branches would otherwise
// misinterpret as "already rendered, emit the (empty) cached Raw() verbatim"
// (BUG H-3). Marking the whole subtree dirty forces every node to re-render
// from its semantic fields regardless of any (already-false) dirty bit a
// nested node happens to carry on its own.
//
// This mirrors isSubtreeDirty's container traversal (render.go) but performs
// a write (markDirty) instead of a read, and additionally walks TableNode/
// ArrayTableNode/KeyNode so a hand-built table or key passed as a value is
// covered too, not just the scalar/array/inline-table cases isSubtreeDirty
// needs for its narrower read-only check.
func markSubtreeDirty(n Node) {
	if n == nil {
		return
	}
	n.markDirty()
	switch node := n.(type) {
	case *ArrayNode:
		for _, elem := range node.Elements {
			markSubtreeDirty(elem)
		}
	case *InlineTableNode:
		for _, child := range node.Children {
			markSubtreeDirty(child)
		}
	case *KeyValueNode:
		if node.Key != nil {
			markSubtreeDirty(node.Key)
		}
		markSubtreeDirty(node.Val)
	case *TableNode:
		for _, child := range node.Children {
			markSubtreeDirty(child)
		}
	case *ArrayTableNode:
		for _, child := range node.Children {
			markSubtreeDirty(child)
		}
	}
}

// sliceIdentity returns the underlying data pointer of a slice value, used
// (like mapIdentity) to detect cycles during marshal/valueToNode recursion.
// ok is false for a nil or empty slice (Pointer() == 0), which can never
// participate in a cycle since it has no elements to recurse into.
func sliceIdentity(items []any) (uintptr, bool) {
	ptr := reflect.ValueOf(items).Pointer()
	if ptr == 0 {
		return 0, false
	}
	return ptr, true
}

// sliceToArrayNode converts a []any to an ArrayNode with recursive
// conversion. visited tracks container identities on the current descent
// path (see valueToNodeVisited) so a self-referential slice errors instead
// of recursing forever; a slice referenced from two non-overlapping
// branches (not a cycle) still marshals fine.
func sliceToArrayNode(items []any, visited map[uintptr]bool) (Node, error) {
	if ptr, tracked := sliceIdentity(items); tracked {
		if visited[ptr] {
			return nil, errCyclicMarshal
		}
		visited[ptr] = true
		defer delete(visited, ptr)
	}

	arr := &ArrayNode{}
	arr.markDirty()
	for _, item := range items {
		elem, err := valueToNodeVisited(item, visited)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		arr.Elements = append(arr.Elements, elem)
	}
	return arr, nil
}

// mapToInlineTableNode converts a map[string]any to an InlineTableNode.
// Keys are sorted alphabetically for deterministic output. visited tracks
// container identities on the current descent path (see valueToNodeVisited)
// so a self-referential map errors instead of recursing forever; a map
// referenced from two non-overlapping branches (not a cycle) still
// marshals fine.
func mapToInlineTableNode(m map[string]any, visited map[uintptr]bool) (Node, error) {
	if ptr, tracked := mapIdentity(reflect.ValueOf(m)); tracked {
		if visited[ptr] {
			return nil, errCyclicMarshal
		}
		visited[ptr] = true
		defer delete(visited, ptr)
	}

	tbl := &InlineTableNode{}
	tbl.markDirty()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		valNode, err := valueToNodeVisited(m[k], visited)
		if err != nil {
			return nil, fmt.Errorf("inline table key %q: %w", k, err)
		}
		kv := newKeyValueNode(k, valNode)
		tbl.Children = append(tbl.Children, kv)
	}
	return tbl, nil
}
