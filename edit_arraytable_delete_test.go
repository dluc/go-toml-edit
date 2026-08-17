package tomledit

import "testing"

// These tests cover the bug where deleting an array-of-tables entry
// (`doc.Delete("x[N]")`) removes only the `[[x]]` header node but leaves
// any `[x.sub]` sub-tables scoped to that entry behind in the document.
// Those orphaned sub-tables then land in front of the next `[[x]]` entry,
// which either fails to re-parse (TOML forbids a table appearing before
// an array-of-tables entry of the same name once that name has been used
// as a table) or, worse, silently re-scopes to the wrong entry.

// arrayTableDeleteCase describes one delete scenario: input document, the
// path to delete, and the exact expected serialized output after the fix.
type arrayTableDeleteCase struct {
	name     string
	in       string
	path     string
	expected string
}

func runArrayTableDeleteCases(t *testing.T, cases []arrayTableDeleteCase) {
	t.Helper()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			doc, err := Parse([]byte(c.in))
			if err != nil {
				t.Fatalf("failed to parse input: %v", err)
			}

			if err := doc.Delete(c.path); err != nil {
				t.Fatalf("Delete(%q) returned error: %v", c.path, err)
			}

			out := doc.Bytes()
			if string(out) != c.expected {
				t.Errorf("Bytes() after Delete(%q) = %q, want %q", c.path, string(out), c.expected)
			}

			// The output must always be valid, re-parseable TOML -- this is
			// the corruption check: a bug here means the emitted document
			// can no longer be read back.
			if _, err := Parse(out); err != nil {
				t.Errorf("output of Delete(%q) does not re-parse: %v\noutput was: %q", c.path, err, string(out))
			}
		})
	}
}

// TestDelete_ArrayTableEntry_OwnsSubTable covers the core corruption bug:
// deleting an array-of-tables entry must also remove sub-tables scoped to
// that specific entry (first, middle, and last entry positions), while
// leaving sibling entries and their own sub-tables untouched.
func TestDelete_ArrayTableEntry_OwnsSubTable(t *testing.T) {
	runArrayTableDeleteCases(t, []arrayTableDeleteCase{
		{
			name:     "first_entry_owns_subtable",
			in:       "[[x]]\na = 1\n[x.sub]\nb = 2\n[[x]]\na = 3\n",
			path:     "x[0]",
			expected: "[[x]]\na = 3\n",
		},
		{
			name:     "middle_entry_owns_subtable",
			in:       "[[x]]\na = 1\n[[x]]\na = 2\n[x.sub]\nb = 3\n[[x]]\na = 4\n",
			path:     "x[1]",
			expected: "[[x]]\na = 1\n[[x]]\na = 4\n",
		},
		{
			name:     "last_entry_owns_subtable",
			in:       "[[x]]\na = 1\n[x.sub]\nb = 2\n[[x]]\na = 3\n",
			path:     "x[1]",
			expected: "[[x]]\na = 1\n[x.sub]\nb = 2\n",
		},
	})
}

// TestDelete_ArrayTableEntry_OwnsNestedSubTable covers an entry that owns a
// compound/nested sub-table path (e.g. [x.sub.deep]) -- all levels scoped
// to the deleted entry must be removed, not just the immediate child.
func TestDelete_ArrayTableEntry_OwnsNestedSubTable(t *testing.T) {
	runArrayTableDeleteCases(t, []arrayTableDeleteCase{
		{
			name:     "first_entry_owns_deeply_nested_subtable",
			in:       "[[x]]\na = 1\n[x.sub]\nb = 2\n[x.sub.deep]\nc = 3\n[[x]]\na = 4\n",
			path:     "x[0]",
			expected: "[[x]]\na = 4\n",
		},
	})
}

// TestDelete_ArrayTableEntry_NoSubTable is a regression guard: deleting an
// entry with no scoped sub-tables must continue to work exactly as before.
func TestDelete_ArrayTableEntry_NoSubTable(t *testing.T) {
	runArrayTableDeleteCases(t, []arrayTableDeleteCase{
		{
			name:     "delete_first_of_two_plain_entries",
			in:       "[[y]]\na = 1\n[[y]]\na = 2\n",
			path:     "y[0]",
			expected: "[[y]]\na = 2\n",
		},
		{
			name:     "delete_last_of_two_plain_entries",
			in:       "[[y]]\na = 1\n[[y]]\na = 2\n",
			path:     "y[1]",
			expected: "[[y]]\na = 1\n",
		},
	})
}
