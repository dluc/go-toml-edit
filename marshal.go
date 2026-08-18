package tomledit

import (
	"fmt"
	"reflect"
	"sort"
)

// Marshal encodes a map[string]any as TOML bytes.
//
// The root value must be a map type (e.g. map[string]any). Non-map types
// (structs, slices, primitives, nil) return an error. Top-level keys become
// key-value pairs; nested map[string]any values become [section] table headers.
// Keys are sorted alphabetically for deterministic output, with simple
// (non-map) keys emitted before sections.
func Marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("toml: Marshal requires a map type, got nil")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map {
		return nil, fmt.Errorf("toml: Marshal requires a map type, got %s", rv.Type())
	}
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("toml: Marshal requires a map type, got %s", rv.Type())
	}

	visited := make(map[uintptr]bool)
	if ptr, ok := mapIdentity(rv); ok {
		visited[ptr] = true
		defer delete(visited, ptr)
	}

	doc, err := mapToDocument(rv, visited)
	if err != nil {
		return nil, err
	}

	b := doc.Format()
	// Format() always ends with \n. For empty maps, return empty bytes.
	if len(doc.Children) == 0 {
		return []byte{}, nil
	}
	return b, nil
}

// errCyclicMarshal is returned whenever the marshal recursion encounters a
// map (or other reference value) that is already on the current descent
// path -- i.e. a genuine cycle, not merely a DAG-style shared reference.
var errCyclicMarshal = fmt.Errorf("toml: Marshal encountered a cyclic map/reference")

// mapIdentity returns the underlying data pointer of a map value (unwrapping
// interfaces first), used to detect cycles during marshal recursion. ok is
// false if v does not unwrap to a map, or the map is nil (nil maps have no
// entries to recurse into, so they can never participate in a cycle).
func mapIdentity(v reflect.Value) (uintptr, bool) {
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() != reflect.Map {
		return 0, false
	}
	ptr := v.Pointer()
	if ptr == 0 {
		return 0, false
	}
	return ptr, true
}

// mapToDocument builds a DocumentNode from a reflect.Value of map kind.
// visited tracks the identities of maps currently on the recursion stack
// (the descent path from the root), so that a true cycle is detected and
// reported as an error instead of recursing forever. Entries are removed
// from visited as the recursion unwinds, so a map referenced from two
// non-overlapping branches (a DAG, not a cycle) still marshals fine.
func mapToDocument(rv reflect.Value, visited map[uintptr]bool) (*DocumentNode, error) {
	doc := &DocumentNode{}
	doc.markDirty()

	keys := sortedMapKeys(rv)

	// Partition keys: simple values first, then map (section) values.
	var simpleKeys []string
	var sectionKeys []string
	for _, k := range keys {
		val := rv.MapIndex(reflect.ValueOf(k))
		if isMapValue(val) {
			sectionKeys = append(sectionKeys, k)
		} else {
			simpleKeys = append(simpleKeys, k)
		}
	}

	// Emit simple key-value pairs.
	for _, k := range simpleKeys {
		val := rv.MapIndex(reflect.ValueOf(k)).Interface()
		kv, err := makeKeyValue(k, val, visited)
		if err != nil {
			return nil, fmt.Errorf("toml: key %q: %w", k, err)
		}
		doc.Children = append(doc.Children, kv)
	}

	// Emit sections for nested maps.
	for _, k := range sectionKeys {
		val := rv.MapIndex(reflect.ValueOf(k))
		// Unwrap interface.
		for val.Kind() == reflect.Interface {
			val = val.Elem()
		}

		ptr, tracked := mapIdentity(val)
		if tracked {
			if visited[ptr] {
				return nil, fmt.Errorf("toml: section %q: %w", k, errCyclicMarshal)
			}
			visited[ptr] = true
		}
		children, err := mapChildrenToNodes(val, visited)
		if tracked {
			delete(visited, ptr)
		}
		if err != nil {
			return nil, fmt.Errorf("toml: section %q: %w", k, err)
		}
		tbl := &TableNode{
			KeyPath:  []string{k},
			Children: children,
		}
		tbl.markDirty()
		tbl.nodeTrivia.TrailingNewline = []byte("\n")
		doc.Children = append(doc.Children, tbl)
	}

	return doc, nil
}

// mapChildrenToNodes converts the entries of a map into KeyValueNode children.
// Nested maps at this level become inline tables (via valueToNode) since we
// only produce one level of [section] headers.
func mapChildrenToNodes(rv reflect.Value, visited map[uintptr]bool) ([]Node, error) {
	keys := sortedMapKeys(rv)
	var nodes []Node
	for _, k := range keys {
		val := rv.MapIndex(reflect.ValueOf(k)).Interface()
		kv, err := makeKeyValue(k, val, visited)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		nodes = append(nodes, kv)
	}
	return nodes, nil
}

// makeKeyValue creates a KeyValueNode for a single key and Go value.
func makeKeyValue(key string, val any, visited map[uintptr]bool) (*KeyValueNode, error) {
	valNode, err := valueToNodeVisited(val, visited)
	if err != nil {
		return nil, err
	}

	keyNode := &KeyNode{
		Parts: []string{key},
	}
	keyNode.markDirty()

	kv := &KeyValueNode{
		Key: keyNode,
		Val: valNode,
	}
	kv.markDirty()
	kv.nodeTrivia.TrailingNewline = []byte("\n")

	return kv, nil
}

// isMapValue returns true if the reflect.Value (potentially wrapped in an
// interface) is a map.
func isMapValue(v reflect.Value) bool {
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v.Kind() == reflect.Map
}

// sortedMapKeys returns the string keys of a map sorted alphabetically.
func sortedMapKeys(rv reflect.Value) []string {
	mapKeys := rv.MapKeys()
	keys := make([]string, len(mapKeys))
	for i, k := range mapKeys {
		keys[i] = k.String()
	}
	sort.Strings(keys)
	return keys
}
