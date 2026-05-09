package tomledit

import (
	"errors"
	"fmt"
	"strings"
)

// SkipTable is returned from a Walk visitor to skip the current table's
// children (or inline table's children).
var SkipTable = errors.New("skip table")

// Walk visits every key-value pair in the document in order, calling fn
// with the dot-path and the value node. Tables are walked into, not yielded
// as standalone entries.
func (d *DocumentNode) Walk(fn func(path string, node Node) error) error {
	// Phase 1: root-level KVs (before any table header)
	for _, child := range d.Children {
		switch child.(type) {
		case *TableNode, *ArrayTableNode:
			// stop at first table header
		case *KeyValueNode:
			if err := walkKV("", child.(*KeyValueNode), fn); err != nil {
				return err
			}
			continue
		default:
			// CommentNode etc. -- skip
			continue
		}
		// We hit a table/array-table; stop root-level KV scan
		break
	}

	// Phase 2: tables and array-of-tables in document order.
	// Array-of-tables need index tracking per key-path.
	arrayCounters := map[string]int{} // key-path string -> next index

	for _, child := range d.Children {
		switch n := child.(type) {
		case *TableNode:
			prefix := buildPathFromParts("", n.KeyPath)
			if err := walkTableChildren(prefix, n.Children, fn); err != nil {
				return err
			}

		case *ArrayTableNode:
			counterKey := joinKeyPath(n.KeyPath)
			idx := arrayCounters[counterKey]
			arrayCounters[counterKey] = idx + 1

			prefix := buildArrayTablePath("", n.KeyPath, idx)
			if err := walkTableChildren(prefix, n.Children, fn); err != nil {
				return err
			}
		}
	}

	return nil
}

// walkTableChildren walks the KV children of a table or array-table entry.
// prefix is the dot-path prefix for all children.
func walkTableChildren(prefix string, children []Node, fn func(string, Node) error) error {
	for _, child := range children {
		if kv, ok := child.(*KeyValueNode); ok {
			if err := walkKV(prefix, kv, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkKV walks a single key-value pair. If the value is an inline table or
// array, it recurses into it. The prefix is prepended to the key parts.
func walkKV(prefix string, kv *KeyValueNode, fn func(string, Node) error) error {
	fullPath := buildPathFromParts(prefix, kv.Key.Parts)
	return walkValue(fullPath, kv.Val, fn)
}

// walkValue visits a value node. Scalars are yielded directly. Inline tables
// and arrays are recursed into.
func walkValue(path string, node Node, fn func(string, Node) error) error {
	switch v := node.(type) {
	case *InlineTableNode:
		err := fn(path, v)
		if err != nil {
			if errors.Is(err, SkipTable) {
				return nil
			}
			return err
		}
		for _, child := range v.Children {
			if kv, ok := child.(*KeyValueNode); ok {
				if err := walkKV(path, kv, fn); err != nil {
					return err
				}
			}
		}
		return nil

	case *ArrayNode:
		err := fn(path, v)
		if err != nil {
			if errors.Is(err, SkipTable) {
				return nil
			}
			return err
		}
		for i, elem := range v.Elements {
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := walkValue(elemPath, elem, fn); err != nil {
				return err
			}
		}
		return nil

	default:
		// Scalar value: yield it
		err := fn(path, node)
		if err != nil {
			if errors.Is(err, SkipTable) {
				// SkipTable on a non-table is a no-op (nothing to skip)
				return nil
			}
			return err
		}
		return nil
	}
}

// buildPathFromParts constructs a dot-path by appending key parts to a prefix.
func buildPathFromParts(prefix string, parts []string) string {
	result := prefix
	for _, part := range parts {
		if result != "" {
			result += "."
		}
		result += quoteKeyIfNeeded(part)
	}
	return result
}

// buildArrayTablePath constructs the path for an array-of-tables entry.
// For example, with prefix="" and keyPath=["products"], idx=0, it produces "products[0]".
// With prefix="servers[0]" and keyPath=["servers","db"], idx=1, it produces "servers[0].db[1]".
//
// The keyPath may contain multiple parts (e.g., ["a","b","c"] for [[a.b.c]]).
// All parts except the last are joined with dots; the last gets the index.
func buildArrayTablePath(prefix string, keyPath []string, idx int) string {
	if len(keyPath) == 0 {
		return prefix
	}
	// Build path for all parts except the last
	result := prefix
	for _, part := range keyPath[:len(keyPath)-1] {
		if result != "" {
			result += "."
		}
		result += quoteKeyIfNeeded(part)
	}
	// Last part gets the index
	last := keyPath[len(keyPath)-1]
	if result != "" {
		result += "."
	}
	result += quoteKeyIfNeeded(last) + fmt.Sprintf("[%d]", idx)
	return result
}

// quoteKeyIfNeeded wraps a key in quotes if it contains characters that
// would be ambiguous in a dot-path (dots, brackets, spaces, etc.).
func quoteKeyIfNeeded(key string) string {
	if key == "" {
		return `""`
	}
	for i := 0; i < len(key); i++ {
		if key[i] > 0x7F || !isBareKeyChar(key[i]) {
			return `"` + escapeKey(key) + `"`
		}
	}
	return key
}

// escapeKey escapes quotes and backslashes in a key for quoting.
func escapeKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// joinKeyPath joins key parts with dots for use as a map key.
func joinKeyPath(parts []string) string {
	return strings.Join(parts, ".")
}
