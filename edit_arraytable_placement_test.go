package tomledit

import (
	"strings"
	"testing"
)

// countArrayTableEntries counts the top-level ArrayTableNode children whose
// KeyPath matches keyPath exactly (semantic count, independent of text
// formatting).
func countArrayTableEntries(doc *DocumentNode, keyPath ...string) int {
	n := 0
	for _, c := range doc.Children {
		if at, ok := c.(*ArrayTableNode); ok && pathsEqual(at.KeyPath, keyPath) {
			n++
		}
	}
	return n
}

// --- Bug A: NewArrayTable placement ---

// Case 1: existing [[x]] block has a scoped sub-table ([x.sub]), followed by
// an unrelated [y] table and a trailing comment. The new [[x]] entry must
// land right after the [x.sub] block and before [y] -- not at EOF.
func TestNewArrayTable_PlacementWithScopedSubTable(t *testing.T) {
	src := "[[x]]\na = 1\n[x.sub]\ns = 9\n\n[y]\nb = 2\n\n# trailing\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := string(doc.Bytes())

	idxSub := strings.Index(out, "[x.sub]")
	idxY := strings.Index(out, "[y]")
	idxLastX := strings.LastIndex(out, "[[x]]")
	if idxSub == -1 || idxY == -1 || idxLastX == -1 {
		t.Fatalf("expected markers missing from output:\n%s", out)
	}
	if !(idxSub < idxLastX && idxLastX < idxY) {
		t.Errorf("expected new [[x]] to land after [x.sub] and before [y]; got order sub=%d lastX=%d y=%d\noutput:\n%s",
			idxSub, idxLastX, idxY, out)
	}
	if count := strings.Count(out, "[[x]]"); count != 2 {
		t.Errorf("expected 2 [[x]] headers, got %d in:\n%s", count, out)
	}

	// Semantic re-parse: verify the array-of-tables now has 2 entries.
	roundTrip(t, doc)
	doc2, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if got := countArrayTableEntries(doc2, "x"); got != 2 {
		t.Errorf("expected 2 [[x]] entries after re-parse, got %d", got)
	}
}

// Case 2: the existing [[x]] block is the last thing in the document. The
// new entry must be inserted right after it (fallback behavior: append at
// end, since there's nothing else to skip past).
func TestNewArrayTable_PlacementAtEndOfDocument(t *testing.T) {
	src := "[y]\nb = 1\n\n[[x]]\na = 1\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := string(doc.Bytes())
	if count := strings.Count(out, "[[x]]"); count != 2 {
		t.Errorf("expected 2 [[x]] headers, got %d in:\n%s", count, out)
	}
	roundTrip(t, doc)
}

// Case 3: no existing [[x]] anywhere -- NewArrayTable falls back to
// appending at the very end of the document (after unrelated content).
func TestNewArrayTable_PlacementFallbackNoExisting(t *testing.T) {
	src := "[y]\nb = 1\n\n# note\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := string(doc.Bytes())
	idxNote := strings.Index(out, "# note")
	idxX := strings.Index(out, "[[x]]")
	if idxNote == -1 || idxX == -1 {
		t.Fatalf("expected markers missing from output:\n%s", out)
	}
	if idxX < idxNote {
		t.Errorf("expected [[x]] to be appended after trailing comment; got x=%d note=%d\noutput:\n%s", idxX, idxNote, out)
	}
	roundTrip(t, doc)
}

// Case 4: multiple existing [[x]] entries, the last one owning a scoped
// sub-table. The new entry must land after the LAST entry's full scoped
// block (including [x.sub]), before the unrelated [y] table.
func TestNewArrayTable_PlacementAfterLastOfMultipleEntries(t *testing.T) {
	src := "[[x]]\na = 1\n[[x]]\na = 2\n[x.sub]\ns = 9\n\n[y]\nb = 1\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := doc.NewArrayTable("x"); err != nil {
		t.Fatalf("NewArrayTable returned error: %v", err)
	}

	out := string(doc.Bytes())
	idxSub := strings.Index(out, "[x.sub]")
	idxY := strings.Index(out, "[y]")
	idxLastX := strings.LastIndex(out, "[[x]]")
	if idxSub == -1 || idxY == -1 || idxLastX == -1 {
		t.Fatalf("expected markers missing from output:\n%s", out)
	}
	if !(idxSub < idxLastX && idxLastX < idxY) {
		t.Errorf("expected new [[x]] to land after [x.sub] and before [y]; got order sub=%d lastX=%d y=%d\noutput:\n%s",
			idxSub, idxLastX, idxY, out)
	}
	if count := strings.Count(out, "[[x]]"); count != 3 {
		t.Errorf("expected 3 [[x]] headers, got %d in:\n%s", count, out)
	}

	doc2 := roundTrip(t, doc)
	if got := countArrayTableEntries(doc2, "x"); got != 3 {
		t.Errorf("expected 3 [[x]] entries after re-parse, got %d", got)
	}
}
