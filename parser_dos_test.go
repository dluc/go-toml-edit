package tomledit

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mixedNestingTOML returns a document that alternates arrays and inline tables.
// It opens `pairs` "[{" pairs (2*pairs levels) and, when extraArray is true,
// wraps the scalar in one more array (2*pairs+1 levels total).
func mixedNestingTOML(pairs int, extraArray bool) string {
	inner := "1"
	if extraArray {
		inner = "[1]"
	}
	return "a = " + strings.Repeat("[{b = ", pairs) + inner + strings.Repeat("}]", pairs) + "\n"
}

// nthByteColumn returns the 1-based column of the nth occurrence of c in src,
// used to predict the position a ParseError should report on a single-line
// document.
func nthByteColumn(src string, c byte, n int) int {
	count := 0
	for i := 0; i < len(src); i++ {
		if src[i] == c {
			count++
			if count == n {
				return i + 1
			}
		}
	}
	return -1
}

// parseErrorFor asserts that src fails to parse with a *ParseError (never a
// panic, never a plain error) and returns it for further assertions.
func parseErrorFor(t *testing.T, src string) *ParseError {
	t.Helper()
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatalf("Parse returned nil error, want *ParseError")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Parse returned %T (%v), want *ParseError", err, err)
	}
	return pe
}

// --- unterminated containers at EOF must error gracefully, never panic ---

func TestUnterminatedContainerAtEOF(t *testing.T) {
	cases := []string{
		"a = [1, 2",
		"a = [",
		"a = {b = 1",
		"a = {",
		"a = [[[",
		"a = [{b = ",
	}
	for _, src := range cases {
		t.Run(fmt.Sprintf("%q", src), func(t *testing.T) {
			pe := parseErrorFor(t, src)
			if pe.Line < 1 || pe.Column < 1 {
				t.Errorf("ParseError position = line %d, column %d; want both >= 1", pe.Line, pe.Column)
			}
		})
	}
}

// --- depth limit ---

func TestNestingDepth_OverLimitArrays(t *testing.T) {
	src := deepArrayTOML(maxNestingDepth + 1)
	pe := parseErrorFor(t, src)
	want := fmt.Sprintf("maximum nesting depth (%d) exceeded", maxNestingDepth)
	if pe.Message != want {
		t.Errorf("Message = %q, want %q", pe.Message, want)
	}
	if pe.Line != 1 {
		t.Errorf("Line = %d, want 1", pe.Line)
	}
	if wantCol := nthByteColumn(src, '[', maxNestingDepth+1); pe.Column != wantCol {
		t.Errorf("Column = %d, want %d", pe.Column, wantCol)
	}
}

func TestNestingDepth_OverLimitInlineTables(t *testing.T) {
	src := deepInlineTableTOML(maxNestingDepth + 1)
	pe := parseErrorFor(t, src)
	want := fmt.Sprintf("maximum nesting depth (%d) exceeded", maxNestingDepth)
	if pe.Message != want {
		t.Errorf("Message = %q, want %q", pe.Message, want)
	}
	if pe.Line != 1 {
		t.Errorf("Line = %d, want 1", pe.Line)
	}
	if wantCol := nthByteColumn(src, '{', maxNestingDepth+1); pe.Column != wantCol {
		t.Errorf("Column = %d, want %d", pe.Column, wantCol)
	}
}

func TestNestingDepth_MixedBracketsShareOneBudget(t *testing.T) {
	t.Run("at_limit", func(t *testing.T) {
		roundTripExact(t, mixedNestingTOML(maxNestingDepth/2, false))
	})
	t.Run("over_limit", func(t *testing.T) {
		pe := parseErrorFor(t, mixedNestingTOML(maxNestingDepth/2, true))
		want := fmt.Sprintf("maximum nesting depth (%d) exceeded", maxNestingDepth)
		if pe.Message != want {
			t.Errorf("Message = %q, want %q", pe.Message, want)
		}
	})
}

func TestNestingDepth_OverLimitReportsLine(t *testing.T) {
	src := "a = [\n" + strings.Repeat("[\n", maxNestingDepth)
	pe := parseErrorFor(t, src)
	if pe.Line != maxNestingDepth+1 {
		t.Errorf("Line = %d, want %d", pe.Line, maxNestingDepth+1)
	}
	if pe.Column != 1 {
		t.Errorf("Column = %d, want 1", pe.Column)
	}
}

func TestNestingDepth_AtLimitParses(t *testing.T) {
	t.Run("arrays", func(t *testing.T) { roundTripExact(t, deepArrayTOML(maxNestingDepth)) })
	t.Run("inline_tables", func(t *testing.T) { roundTripExact(t, deepInlineTableTOML(maxNestingDepth)) })
}

func TestNestingDepth_WideFileParses(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "k%d = [1, 2]\nm%d = {b = 1}\n", i, i)
	}
	roundTripExact(t, b.String())
}
