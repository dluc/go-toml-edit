package tomledit

import (
	"strings"
	"testing"
)

// roundTripExact asserts that Parse(src).Bytes() returns byte-for-byte the
// original src. This is stricter than the roundTrip helper in edit_test.go,
// which only checks that the serialized output re-parses.
func roundTripExact(t *testing.T, src string) {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v (input length %d)", err, len(src))
	}
	got := string(doc.Bytes())
	if got == src {
		return
	}
	n := min(len(got), len(src))
	for i := 0; i < n; i++ {
		if got[i] != src[i] {
			lo := max(0, i-20)
			t.Fatalf("round-trip mismatch at byte %d:\n  got:  %q\n  want: %q\n  (lengths: got=%d want=%d)",
				i, got[lo:min(len(got), i+20)], src[lo:min(len(src), i+20)], len(got), len(src))
		}
	}
	t.Fatalf("round-trip length mismatch: got %d bytes, want %d bytes", len(got), len(src))
}

// deepArrayTOML returns a document whose value nests `depth` arrays.
func deepArrayTOML(depth int) string {
	return "a = " + strings.Repeat("[", depth) + strings.Repeat("]", depth) + "\n"
}

// deepInlineTableTOML returns a document whose value nests `depth` inline tables.
func deepInlineTableTOML(depth int) string {
	return "a = " + strings.Repeat("{b = ", depth) + "1" + strings.Repeat("}", depth) + "\n"
}

func TestRoundTripExact_NestingPermutations(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty_array", "a = []\n"},
		{"empty_inline_table", "a = {}\n"},
		{"nested_arrays", "a = [[[]]]\n"},
		{"nested_inline_tables", "a = {b = {c = {}}}\n"},
		{"mixed_array_of_inline_table", "a = [{b = [{}]}]\n"},
		{"array_interior_whitespace", "a = [ [ [ ] ] ]\n"},
		{"inline_table_interior_whitespace", "a = {  b  =  1  ,  c  =  2  }\n"},
		{"array_trailing_comma", "a = [1, 2, 3,]\n"},
		{"multiline_array", "a = [\n  1,\n  2,\n]\n"},
		{"multiline_array_comments", "a = [\n  # leading\n  1, # inline\n  2,\n  # trailing\n]\n"},
		{"multiline_array_of_inline_tables", "a = [\n  {b = 1},\n  {b = 2},\n]\n"},
		{"nested_multiline_mixed", "a = [\n  {b = [1, 2]}, # one\n  {c = {d = []}},\n]\n"},
		{"array_in_table", "[t]\na = [[1], [2]]\n"},
		{"crlf", "a = [1, 2]\r\n"},
		{"no_trailing_newline", "a = [1, 2]"},
		{"deep_arrays_100", deepArrayTOML(100)},
		{"deep_inline_tables_100", deepInlineTableTOML(100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripExact(t, tc.src)
		})
	}
}

func TestRoundTripExact_DeepBoundary(t *testing.T) {
	t.Run("arrays_2047", func(t *testing.T) { roundTripExact(t, deepArrayTOML(2047)) })
	t.Run("inline_tables_2047", func(t *testing.T) { roundTripExact(t, deepInlineTableTOML(2047)) })
}

func TestBytesDoesNotAliasSource(t *testing.T) {
	src := []byte("a = [1, 2]\n")
	original := string(src)
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	out := doc.Bytes()
	out[0] = 'X'
	if string(src) != original {
		t.Errorf("mutating Bytes() output changed the source: got %q, want %q", string(src), original)
	}
	doc2, err := Parse(src)
	if err != nil {
		t.Fatalf("re-Parse returned error: %v", err)
	}
	if string(doc2.Bytes()) != original {
		t.Errorf("re-parse after mutating Bytes() output: got %q, want %q", string(doc2.Bytes()), original)
	}
}
