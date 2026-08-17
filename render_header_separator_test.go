package tomledit

import "testing"

// These tests cover the bug where appending a table or array-table header
// to a document whose last emitted bytes do NOT end in a newline produces
// invalid, unparseable TOML: the new header glues directly onto the end of
// the previous line (e.g. "a = 1" + NewArrayTable("x") -> "a = 1[[x]]\n").
//
// Root cause: renderTableHeader/renderArrayTableHeader emit no leading
// separator, and DocumentNode.Bytes() concatenates serialized children with
// no separator logic at all.

// TestNewArrayTable_NoTrailingNewline_MustNotGlue is the corruption case from
// the bug report: appending an array-table to a document with no trailing
// newline must insert a separating newline, not glue the header onto the
// previous line.
func TestNewArrayTable_NoTrailingNewline_MustNotGlue(t *testing.T) {
	doc, err := Parse([]byte("a = 1"))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := doc.Bytes()
	want := "a = 1\n[[x]]\n"
	if string(out) != want {
		t.Errorf("Bytes() = %q, want %q", string(out), want)
	}

	if _, err := Parse(out); err != nil {
		t.Errorf("output does not re-parse (corruption): %v\noutput was: %q", err, string(out))
	}
}

// TestNewTable_NoTrailingNewline_MustNotGlue is the analogous corruption case
// for a regular [table] header (same root cause, same fix).
func TestNewTable_NoTrailingNewline_MustNotGlue(t *testing.T) {
	doc, err := Parse([]byte("a = 1"))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if err := doc.NewTable("t"); err != nil {
		t.Fatalf("NewTable returned error: %v", err)
	}

	out := doc.Bytes()
	want := "a = 1\n[t]\n"
	if string(out) != want {
		t.Errorf("Bytes() = %q, want %q", string(out), want)
	}

	if _, err := Parse(out); err != nil {
		t.Errorf("output does not re-parse (corruption): %v\noutput was: %q", err, string(out))
	}
}

// TestNewArrayTable_WithTrailingNewline_NoDoubleBlankLine is a regression
// guard: when the preceding sibling already ends in a newline, the fix must
// NOT insert a spurious extra blank line before the new header.
func TestNewArrayTable_WithTrailingNewline_NoDoubleBlankLine(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n"))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := doc.Bytes()
	want := "a = 1\n[[x]]\n"
	if string(out) != want {
		t.Errorf("Bytes() = %q, want %q (must not double the newline)", string(out), want)
	}

	if _, err := Parse(out); err != nil {
		t.Errorf("output does not re-parse: %v\noutput was: %q", err, string(out))
	}
}

// TestNewTable_WithTrailingNewline_NoDoubleBlankLine is the [table]
// counterpart of the regression guard above.
func TestNewTable_WithTrailingNewline_NoDoubleBlankLine(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n"))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if err := doc.NewTable("t"); err != nil {
		t.Fatalf("NewTable returned error: %v", err)
	}

	out := doc.Bytes()
	want := "a = 1\n[t]\n"
	if string(out) != want {
		t.Errorf("Bytes() = %q, want %q (must not double the newline)", string(out), want)
	}

	if _, err := Parse(out); err != nil {
		t.Errorf("output does not re-parse: %v\noutput was: %q", err, string(out))
	}
}

// TestNewArrayTable_EmptyDocument_NoLeadingNewline guards the other edge: an
// empty document has no preceding bytes at all, so no separator should be
// inserted before the first header.
func TestNewArrayTable_EmptyDocument_NoLeadingNewline(t *testing.T) {
	doc, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := doc.Bytes()
	want := "[[x]]\n"
	if string(out) != want {
		t.Errorf("Bytes() = %q, want %q (no leading separator on empty doc)", string(out), want)
	}

	if _, err := Parse(out); err != nil {
		t.Errorf("output does not re-parse: %v\noutput was: %q", err, string(out))
	}
}
