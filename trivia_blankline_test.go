package tomledit

import "testing"

// --- Bug #1: blank-line runs in orphan trailing trivia are dropped ---
//
// Root cause: emitOrphanTrivia (parser.go) discards blank-line bytes when
// reconstructing orphan CommentNodes from the collected `comments` slice --
// it never looks at the blank-line bytes that precede/separate them.

func TestRoundTripExact_OrphanTrailingCommentAfterBlankLine(t *testing.T) {
	roundTripExact(t, "a = 1\n\n# bye\n")
}

func TestRoundTripExact_OrphanCommentBlockAfterBlankLine(t *testing.T) {
	roundTripExact(t, "a = 1\n\n# l1\n# l2\n# l3\n")
}

func TestRoundTripExact_OrphanInteriorBlankBetweenComments(t *testing.T) {
	roundTripExact(t, "a = 1\n# c1\n\n# c2\n")
}

func TestRoundTripExact_OrphanMultipleBlankLinesBeforeComment(t *testing.T) {
	roundTripExact(t, "a = 1\n\n\n# x\n")
}

// --- Bug #5a: Set() drops the blank line above the edited key ---
//
// Root cause: the Trivia struct (node.go) has no field for blank-line runs,
// so renderTrivia (render.go), used once a node is dirtied by Set, only
// emits LeadingComments + LeadingWhitespace -- losing any blank line that
// preceded the key in the original source.

func TestSet_PreservesBlankLineAboveEditedKey(t *testing.T) {
	doc, err := Parse([]byte("a = 1\n\nfoo = 'bar'\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if err := doc.Set("foo", "new"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got := string(doc.Bytes())
	// foo's original literal ('...') quote style is preserved by the #5c fix
	// (see edit_string_style_test.go); this test's own concern is the blank
	// line above foo, which must survive regardless of quote style.
	want := "a = 1\n\nfoo = 'new'\n"
	if got != want {
		t.Errorf("Bytes() = %q, want %q", got, want)
	}
}
