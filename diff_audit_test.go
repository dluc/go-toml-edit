package tomledit

import (
	"testing"
)

// Audit 1: Diff with inline tables -- changes inside {x = 1, y = 2}.
func TestAudit_Diff_InlineTables(t *testing.T) {
	a, err := Parse([]byte(`config = {x = 1, y = 2}
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`config = {x = 1, y = 99}
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Modified, Path: "config.y", OldValue: int64(2), NewValue: int64(99)},
	})
}

// Audit 1b: Diff with inline table -- added key inside inline table.
func TestAudit_Diff_InlineTableAddedKey(t *testing.T) {
	a, err := Parse([]byte(`config = {x = 1}
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`config = {x = 1, y = 2}
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Added, Path: "config.y", NewValue: int64(2)},
	})
}

// Audit 2: Diff with dotted keys -- a.b.c = 1 vs a.b.c = 2.
func TestAudit_Diff_DottedKeys(t *testing.T) {
	a, err := Parse([]byte(`a.b.c = 1
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`a.b.c = 2
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Modified, Path: "a.b.c", OldValue: int64(1), NewValue: int64(2)},
	})
}

// Audit 2b: Diff with dotted keys -- added dotted key.
func TestAudit_Diff_DottedKeysAdded(t *testing.T) {
	a, err := Parse([]byte(`a.b.c = 1
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`a.b.c = 1
a.b.d = 2
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Added, Path: "a.b.d", NewValue: int64(2)},
	})
}

// Audit 3: Diff symmetry -- Diff(a, b) Removed entries should be Diff(b, a) Added.
func TestAudit_Diff_Symmetry(t *testing.T) {
	a, err := Parse([]byte(`name = "alpha"
count = 10
old_key = "gone"
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`name = "beta"
count = 10
new_key = "here"
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	ab := Diff(a, b)
	ba := Diff(b, a)

	// Build maps for easier lookup
	abMap := make(map[string]Change)
	for _, c := range ab {
		abMap[c.Path+"/"+c.Kind.String()] = c
	}
	baMap := make(map[string]Change)
	for _, c := range ba {
		baMap[c.Path+"/"+c.Kind.String()] = c
	}

	// Removed in Diff(a,b) should be Added in Diff(b,a)
	for _, c := range ab {
		if c.Kind == Removed {
			counterpart, ok := baMap[c.Path+"/"+Added.String()]
			if !ok {
				t.Errorf("Removed %q in Diff(a,b) has no Added counterpart in Diff(b,a)", c.Path)
				continue
			}
			if !valuesEqual(c.OldValue, counterpart.NewValue) {
				t.Errorf("Removed %q OldValue %v != Added NewValue %v", c.Path, c.OldValue, counterpart.NewValue)
			}
		}
	}

	// Added in Diff(a,b) should be Removed in Diff(b,a)
	for _, c := range ab {
		if c.Kind == Added {
			counterpart, ok := baMap[c.Path+"/"+Removed.String()]
			if !ok {
				t.Errorf("Added %q in Diff(a,b) has no Removed counterpart in Diff(b,a)", c.Path)
				continue
			}
			if !valuesEqual(c.NewValue, counterpart.OldValue) {
				t.Errorf("Added %q NewValue %v != Removed OldValue %v", c.Path, c.NewValue, counterpart.OldValue)
			}
		}
	}

	// Modified in Diff(a,b) should be Modified in Diff(b,a) with swapped values
	for _, c := range ab {
		if c.Kind == Modified {
			counterpart, ok := baMap[c.Path+"/"+Modified.String()]
			if !ok {
				t.Errorf("Modified %q in Diff(a,b) has no Modified counterpart in Diff(b,a)", c.Path)
				continue
			}
			if !valuesEqual(c.OldValue, counterpart.NewValue) {
				t.Errorf("Modified %q: Diff(a,b).OldValue %v != Diff(b,a).NewValue %v",
					c.Path, c.OldValue, counterpart.NewValue)
			}
			if !valuesEqual(c.NewValue, counterpart.OldValue) {
				t.Errorf("Modified %q: Diff(a,b).NewValue %v != Diff(b,a).OldValue %v",
					c.Path, c.NewValue, counterpart.OldValue)
			}
		}
	}

	// Same number of changes in both directions
	if len(ab) != len(ba) {
		t.Errorf("Diff(a,b) has %d changes, Diff(b,a) has %d", len(ab), len(ba))
	}
}

// Audit 4: Diff with comments only differing -- should NOT appear as changes.
func TestAudit_Diff_CommentsOnly(t *testing.T) {
	a, err := Parse([]byte(`# Comment on name
name = "test"
port = 8080 # inline comment
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`# Different comment on name
name = "test"
port = 8080 # different inline comment
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	if len(changes) != 0 {
		t.Errorf("expected no changes for comment-only differences, got %d:\n%s",
			len(changes), formatChanges(changes))
	}
}

// Audit 4b: Diff with comments only on tables -- should NOT appear as changes.
func TestAudit_Diff_CommentsOnlyTables(t *testing.T) {
	a, err := Parse([]byte(`# Comment A
[server]
host = "localhost"
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`# Comment B
[server]
host = "localhost"
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	if len(changes) != 0 {
		t.Errorf("expected no changes for comment-only differences, got %d:\n%s",
			len(changes), formatChanges(changes))
	}
}

// Audit 5: Diff with array elements -- [1, 2, 3] vs [1, 2, 4].
func TestAudit_Diff_ArrayElements(t *testing.T) {
	a, err := Parse([]byte(`items = [1, 2, 3]
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`items = [1, 2, 4]
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Modified, Path: "items[2]", OldValue: int64(3), NewValue: int64(4)},
	})
}

// Audit 5b: Diff with array length change.
func TestAudit_Diff_ArrayLengthChange(t *testing.T) {
	a, err := Parse([]byte(`items = [1, 2, 3]
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`items = [1, 2]
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Removed, Path: "items[2]", OldValue: int64(3)},
	})
}

// Audit 5c: Diff with array of strings.
func TestAudit_Diff_ArrayOfStrings(t *testing.T) {
	a, err := Parse([]byte(`tags = ["a", "b", "c"]
`))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}

	b, err := Parse([]byte(`tags = ["a", "B", "c", "d"]
`))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	changes := Diff(a, b)
	requireChanges(t, changes, []Change{
		{Kind: Modified, Path: "tags[1]", OldValue: "b", NewValue: "B"},
		{Kind: Added, Path: "tags[3]", NewValue: "d"},
	})
}
