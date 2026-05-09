package tomledit

import (
	"strings"
	"testing"
)

// TestFormatArrayInlineComments verifies that Format() preserves inline
// comments on array elements.
func TestFormatArrayInlineComments(t *testing.T) {
	input := `arr = [
    1, # first
    2, # second
    3,
]
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(doc.Format())
	if !strings.Contains(got, "# first") {
		t.Errorf("lost inline comment '# first'\ngot:\n%s", got)
	}
	if !strings.Contains(got, "# second") {
		t.Errorf("lost inline comment '# second'\ngot:\n%s", got)
	}
	// All elements should still be present.
	if !strings.Contains(got, "1,") {
		t.Errorf("lost element 1\ngot:\n%s", got)
	}
	if !strings.Contains(got, "2,") {
		t.Errorf("lost element 2\ngot:\n%s", got)
	}
	if !strings.Contains(got, "3,") {
		t.Errorf("lost element 3\ngot:\n%s", got)
	}
}

// TestFormatArrayLeadingComments verifies that Format() preserves leading
// (standalone) comments between array elements.
func TestFormatArrayLeadingComments(t *testing.T) {
	input := `arr = [
    1,
    # between first and second
    2,
    3,
]
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(doc.Format())
	if !strings.Contains(got, "# between first and second") {
		t.Errorf("lost leading comment\ngot:\n%s", got)
	}
	// The comment should appear before element 2.
	idx2 := strings.Index(got, "2,")
	idxComment := strings.Index(got, "# between first and second")
	if idx2 < 0 || idxComment < 0 || idxComment >= idx2 {
		// idxComment should be less than idx2 (comment before element)
		if idxComment >= idx2 {
			t.Errorf("leading comment should appear before element 2\ngot:\n%s", got)
		}
	}
}

// TestFormatArrayTrailingComments verifies that Format() preserves trailing
// comments (after the last element, before the closing bracket).
func TestFormatArrayTrailingComments(t *testing.T) {
	input := `arr = [
    1,
    2,
    # trailing note
]
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(doc.Format())
	if !strings.Contains(got, "# trailing note") {
		t.Errorf("lost trailing comment\ngot:\n%s", got)
	}
	// Trailing comment should appear before the closing bracket.
	idxComment := strings.Index(got, "# trailing note")
	idxBracket := strings.LastIndex(got, "]")
	if idxComment < 0 || idxBracket < 0 || idxComment >= idxBracket {
		t.Errorf("trailing comment should appear before ']'\ngot:\n%s", got)
	}
}

// TestFormatArrayCommentsForceMultiLine verifies that a short array with
// comments is forced to multi-line format even though it would otherwise
// fit on a single line.
func TestFormatArrayCommentsForceMultiLine(t *testing.T) {
	input := `arr = [
    1, # note
    2,
]
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Use a very wide line width -- without comment forcing, this would be
	// rendered inline as [1, 2].
	got := string(doc.Format(WithLineWidth(200)))
	if !strings.Contains(got, "[\n") {
		t.Errorf("array with comments should be multi-line even with wide line width\ngot:\n%s", got)
	}
	if !strings.Contains(got, "# note") {
		t.Errorf("lost inline comment\ngot:\n%s", got)
	}
}
