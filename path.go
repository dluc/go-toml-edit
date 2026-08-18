package tomledit

import (
	"fmt"
	"strconv"
	"strings"
)

// SegmentType distinguishes key lookups from index lookups.
type segmentType int

const (
	keySegment   segmentType = iota
	indexSegment
)

type pathSegment struct {
	Type  segmentType
	Key   string // for keySegment
	Index int    // for indexSegment (can be negative)
}

// parsePath parses a dot-separated path with support for bracket indices,
// escaped dots, and quoted segments.
//
// Syntax:
//   - Dot separates key segments: "a.b.c"
//   - [N] is an index (positive or negative): "a[0]", "a[-1]"
//   - Backslash-dot is a literal dot in a key: "host\.name"
//   - Quoted segment: "host.name" -> key "host.name" (quotes are part of the path syntax, not the key)
//   - Adjacent brackets: "[0][1]" for nested arrays
//
// Empty path returns an error. A leading dot, a trailing dot, or two
// consecutive dots all denote an empty key segment, which is also an error:
// every dot must have a real segment (bare key, quoted key, or index) on
// both sides of it.
func parsePath(path string) ([]pathSegment, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	var segments []pathSegment
	i := 0
	// expectSegment is true at the very start of the path and immediately
	// after consuming a '.' separator: a '.' encountered while it is still
	// true means an empty segment (leading dot, trailing dot once we fall
	// off the end, or two consecutive dots) -- report it as an error
	// instead of falling through to a parse step that could fail to
	// consume any input and stall the loop forever.
	expectSegment := true

	for i < len(path) {
		if path[i] == '.' {
			if expectSegment {
				return nil, fmt.Errorf("empty key segment in path %q at position %d", path, i)
			}
			i++
			if i >= len(path) {
				return nil, fmt.Errorf("trailing dot in path")
			}
			expectSegment = true
			continue
		}

		if path[i] == '[' {
			// Index segment
			seg, newI, err := parseIndex(path, i)
			if err != nil {
				return nil, err
			}
			segments = append(segments, seg)
			i = newI
			expectSegment = false
			continue
		}

		if path[i] == '"' {
			// Quoted key segment
			seg, newI, err := parseQuotedKey(path, i)
			if err != nil {
				return nil, err
			}
			segments = append(segments, seg)
			i = newI
			expectSegment = false
			continue
		}

		// Bare key segment (may contain escaped dots). By construction
		// path[i] here is never '.', '[', or '"' (all handled above), so
		// parseBareKey is guaranteed to consume at least one byte -- but
		// guard against zero progress explicitly anyway so this loop can
		// never stall regardless of future changes to parseBareKey.
		seg, newI := parseBareKey(path, i)
		if newI == i {
			return nil, fmt.Errorf("empty key segment in path %q at position %d", path, i)
		}
		segments = append(segments, seg)
		i = newI
		expectSegment = false
	}

	return segments, nil
}

// parseIndex parses "[N]" or "[-N]" starting at position i.
func parseIndex(path string, i int) (pathSegment, int, error) {
	if i >= len(path) || path[i] != '[' {
		return pathSegment{}, i, fmt.Errorf("expected '['")
	}
	i++ // skip '['

	end := strings.IndexByte(path[i:], ']')
	if end < 0 {
		return pathSegment{}, i, fmt.Errorf("unclosed bracket in path")
	}
	end += i

	content := path[i:end]
	if content == "" {
		return pathSegment{}, end, fmt.Errorf("empty index in path")
	}

	idx, err := strconv.Atoi(content)
	if err != nil {
		return pathSegment{}, end, fmt.Errorf("non-numeric index %q in path", content)
	}

	return pathSegment{Type: indexSegment, Index: idx}, end + 1, nil
}

// parseQuotedKey parses a quoted key segment like "host.name" starting at position i.
func parseQuotedKey(path string, i int) (pathSegment, int, error) {
	if i >= len(path) || path[i] != '"' {
		return pathSegment{}, i, fmt.Errorf("expected '\"'")
	}
	i++ // skip opening quote

	var key strings.Builder
	for i < len(path) {
		if path[i] == '\\' && i+1 < len(path) {
			// Escaped character inside quotes
			key.WriteByte(path[i+1])
			i += 2
			continue
		}
		if path[i] == '"' {
			return pathSegment{Type: keySegment, Key: key.String()}, i + 1, nil
		}
		key.WriteByte(path[i])
		i++
	}
	return pathSegment{}, i, fmt.Errorf("unclosed quote in path")
}

// parseBareKey parses a bare key segment, handling backslash-escaped dots.
func parseBareKey(path string, i int) (pathSegment, int) {
	var key strings.Builder
	for i < len(path) {
		if path[i] == '\\' && i+1 < len(path) && path[i+1] == '.' {
			// Escaped dot: literal dot in key
			key.WriteByte('.')
			i += 2
			continue
		}
		if path[i] == '.' || path[i] == '[' {
			break
		}
		key.WriteByte(path[i])
		i++
	}
	return pathSegment{Type: keySegment, Key: key.String()}, i
}
